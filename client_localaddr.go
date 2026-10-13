package gogram

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"sync/atomic"
	"time"
)

// WithLocalAddrs makes the client dial Telegram from the given local IPs,
// one connection pool per address, rotating between them and failing over
// to the next address when a dial fails. Use it on a multi-homed host to
// keep the bot reachable when one uplink goes down. Telegram's limits are
// per bot token, so this does not raise throughput.
//
// It wraps the transport of the client's http.Client (see [WithHTTPClient]),
// keeping its TLS and proxy settings, and must come after it in the options.
func WithLocalAddrs(addrs ...netip.Addr) ClientOption {
	return func(c *Client) {
		if len(addrs) == 0 {
			return
		}

		base, ok := c.cfg.httpClient.Transport.(*http.Transport)
		if !ok {
			base, _ = http.DefaultTransport.(*http.Transport)
		}

		multi := &multiAddrTransport{transports: make([]*http.Transport, len(addrs))}

		for i, addr := range addrs {
			dialer := &net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				LocalAddr: &net.TCPAddr{IP: addr.AsSlice()},
			}

			t := base.Clone()
			t.DialContext = dialer.DialContext
			multi.transports[i] = t
		}

		// Copy the client so the caller's http.Client is left untouched.
		hc := *c.cfg.httpClient
		hc.Transport = multi
		c.cfg.httpClient = &hc
	}
}

// multiAddrTransport round-robins requests over per-address transports
// and retries a request on the next address when dialing failed. A dial
// error happens before any byte of the body is sent, so the retry is safe
// even for streamed bodies.
type multiAddrTransport struct {
	transports []*http.Transport
	next       atomic.Uint64
}

// RoundTrip implements http.RoundTripper.
func (m *multiAddrTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := m.next.Add(1) - 1

	var err error

	for i := range m.transports {
		t := m.transports[(start+uint64(i))%uint64(len(m.transports))] //nolint:gosec // G115: i is a small index

		resp, tryErr := t.RoundTrip(req)
		if tryErr == nil {
			return resp, nil
		}

		err = tryErr

		if !isDialError(tryErr) || req.Context().Err() != nil {
			break
		}
	}

	return nil, err
}

// isDialError reports whether err happened while connecting, i.e. before
// the request was written and another address may still work.
func isDialError(err error) bool {
	var opErr *net.OpError
	return errors.As(err, &opErr) && opErr.Op == "dial"
}

// CloseIdleConnections implements the optional interface net/http uses.
func (m *multiAddrTransport) CloseIdleConnections() {
	for _, t := range m.transports {
		t.CloseIdleConnections()
	}
}
