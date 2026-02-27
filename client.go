package gogram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

//go:generate go run ./cmd/gen

const (
	defaultHost    = "api.telegram.org"
	defaultRPS     = 30
	defaultTimeout = 10 * time.Second
	defaultUpdates = 100
)

type ClientOption func(client *Client) ClientOption

func WithHost(host string) ClientOption {
	return func(c *Client) ClientOption {
		c.locker.Lock()
		defer c.locker.Unlock()

		previous := c.options.host
		c.options.host = host

		c.buildLinkPrefix(c.options.test, c.options.host, c.token)

		return WithHost(previous)
	}
}

func WithRPS(rps int) ClientOption {
	return func(c *Client) ClientOption {
		c.locker.Lock()
		defer c.locker.Unlock()

		previous := c.options.rps
		c.options.rps = rps

		c.rateLimiter.SetLimit(rate.Every(time.Second / time.Duration(rps)))
		c.rateLimiter.SetBurst(rps)

		return WithRPS(previous)
	}
}

func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) ClientOption {
		c.locker.Lock()
		defer c.locker.Unlock()

		previous := c.options.timeout
		c.options.timeout = timeout

		c.httpClient.Timeout = timeout

		return WithTimeout(previous)
	}
}

func WithTest(test bool) ClientOption {
	return func(c *Client) ClientOption {
		c.locker.Lock()
		defer c.locker.Unlock()

		previous := c.options.test
		c.options.test = test

		c.buildLinkPrefix(c.options.test, c.options.host, c.token)

		return WithTest(previous)
	}
}

func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) ClientOption {
		c.locker.Lock()
		defer c.locker.Unlock()

		previous := c.httpClient
		c.httpClient = client

		return WithHTTPClient(previous)
	}
}

func WithDefaultParseMode(parseMode string) ClientOption {
	return func(c *Client) ClientOption {
		c.locker.Lock()
		defer c.locker.Unlock()

		previous := c.defaultParseMode
		c.defaultParseMode = parseMode

		return WithDefaultParseMode(previous)
	}
}

var defaultOpts = []ClientOption{
	WithHost(defaultHost),
	WithRPS(defaultRPS),
	WithTest(false),
	WithTimeout(defaultTimeout),
}

type clientOption struct {
	host    string
	rps     int
	timeout time.Duration
	test    bool
}

// Client
//
// https://core.telegram.org/bots/api
//
// https://core.telegram.org/bots/webapps
type Client struct {
	id      int64
	token   string
	options clientOption

	linkPrefix string

	httpTrace             *httptrace.ClientTrace
	httpClient            *http.Client
	localAddr, remoteAddr net.Addr

	rateLimiter *rate.Limiter

	defaultParseMode string

	updates chan *Update
	done    chan struct{}
	started bool

	router *Router

	locker sync.RWMutex
}

func (c *Client) buildLinkPrefix(test bool, host, token string) {
	if test {
		c.linkPrefix = "https://" + host + "/bot" + token + "/test/"
	} else {
		c.linkPrefix = "https://" + host + "/bot" + token + "/"
	}
}

func (c *Client) Option(opts ...ClientOption) (previous ClientOption) {
	for _, opt := range opts {
		previous = opt(c)
	}
	return previous
}

func NewClient(token string, opts ...ClientOption) (*Client, error) {
	if token == "" {
		return nil, errors.New("gogram: no token provided")
	}

	botID, err := strconv.ParseInt(strings.SplitN(token, ":", 2)[0], 10, 64)
	if err != nil {
		return nil, errors.New("gogram: invalid token provided")
	}

	c := new(Client)

	c.id = botID
	c.token = token

	c.httpTrace = &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			c.localAddr = info.Conn.LocalAddr()
			c.remoteAddr = info.Conn.RemoteAddr()
		},
	}

	c.httpClient = &http.Client{}
	c.rateLimiter = &rate.Limiter{}

	c.router = NewRouter()

	c.Option(defaultOpts...)
	c.Option(opts...)

	return c, nil
}

func (c *Client) ID() int64 {
	return c.id
}

func (c *Client) Token() string {
	return c.token
}

func (c *Client) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *Client) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *Client) Router() *Router {
	return c.router
}

func (c *Client) SetRouter(r *Router) {
	c.router = r
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	err := c.rateLimiter.Wait(req.Context())
	if err != nil {
		return nil, err
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), c.httpTrace))

	return c.httpClient.Do(req)
}

func (c *Client) Raw(method string, reader io.Reader, contentType ...string) (json.RawMessage, error) {
	link := c.linkPrefix + method

	req, err := http.NewRequest(http.MethodPost, link, reader)
	if err != nil {
		return nil, err
	}

	if len(contentType) != 0 {
		req.Header.Set("Content-Type", contentType[0])
	} else {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	v := new(Response)

	err = json.NewDecoder(resp.Body).Decode(v)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.Join(ErrBadRequest, ErrEOF)
		}
		return nil, err
	}

	if !v.OK {
		var retryErr *RetryError

		err = genError(v.ErrorCode, resp.Status, v.Description, v.Parameters)

		if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrNotFoundBanned) {
			c.Stop()
		}

		if errors.As(err, &retryErr) {
			c.rateLimiter.SetLimitAt(
				time.Now().Add(retryErr.RetryAfter),
				c.rateLimiter.Limit(),
			)
		}

		return nil, err
	}

	return v.Result, nil
}

func (c *Client) Start(ctx context.Context, params *GetUpdatesParams) error {
	c.locker.Lock()

	if c.started {
		c.locker.Unlock()
		return errors.New("gogram: already started")
	}

	c.updates = make(chan *Update, defaultUpdates)
	c.done = make(chan struct{})
	c.started = true

	done := c.done
	timeout := c.options.timeout

	c.locker.Unlock()

	localParams := GetUpdatesParams{
		Limit:   defaultUpdates,
		Timeout: int64(timeout.Seconds()),
	}
	if params != nil {
		localParams = *params
		if localParams.Limit == 0 {
			localParams.Limit = defaultUpdates
		}
	}

	workerCount := min(max(runtime.GOMAXPROCS(0), 1), int(localParams.Limit))

	go func() {
		for {
			select {
			case <-ctx.Done():
				c.Stop()

			case <-done:
				close(c.updates)
				return

			default:
				updates, err := c.GetUpdates(&localParams)
				if err != nil {
					c.locker.RLock()
					handlerErr := c.Router().handlerErr
					c.locker.RUnlock()

					if handlerErr != nil {
						gogramCtx := c.acquireContext(ctx, nil)
						handlerErr(gogramCtx, err)
						c.releaseContext(gogramCtx)
					}

					continue
				}

				for i := range updates {
					c.updates <- &updates[i]
					localParams.Offset = updates[i].UpdateID + 1
				}
			}
		}
	}()

	wg := sync.WaitGroup{}

	wg.Add(workerCount)

	for range workerCount {
		go func() {
			c.processUpdates(ctx)
			wg.Done()
		}()
	}

	wg.Wait()

	c.locker.Lock()

	c.updates = nil
	c.done = nil
	c.started = false

	c.locker.Unlock()

	return ctx.Err()
}

func (c *Client) Stop() {
	c.locker.Lock()

	if c.done != nil {
		close(c.done)
		c.done = nil
	}

	c.locker.Unlock()
}

func (c *Client) processUpdates(ctx context.Context) {
	for u := range c.updates {
		c.processUpdate(ctx, u)
	}
}

func (c *Client) processUpdate(ctx context.Context, u *Update) {
	gogramCtx := c.acquireContext(ctx, u)
	defer c.releaseContext(gogramCtx)

	c.Router().Process(gogramCtx)
}
