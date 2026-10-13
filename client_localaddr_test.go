package gogram_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/darxnet/gogram"
)

func TestWithLocalAddrs_FailsOverToAWorkingAddress(t *testing.T) {
	t.Parallel()

	var served atomic.Int32

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served.Add(1)

		if !strings.HasPrefix(r.RemoteAddr, "127.0.0.1:") {
			t.Errorf("remote addr = %s, want 127.0.0.1", r.RemoteAddr)
		}

		resp := testResponse{OK: true, Result: json.RawMessage(`{"id":1,"is_bot":true,"first_name":"b"}`)}
		_, _ = w.Write(mustMarshal(t, &resp))
	}))
	defer server.Close()

	// 192.0.2.1 (TEST-NET-1) is not a local address: binding to it fails at
	// dial time, so the transport must move on to 127.0.0.1.
	client, err := gogram.NewClient(testToken,
		gogram.WithHost(strings.TrimPrefix(server.URL, "https://")),
		gogram.WithHTTPClient(server.Client()),
		gogram.WithLocalAddrs(netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("127.0.0.1")),
	)
	if err != nil {
		t.Fatal(err)
	}

	for range 3 { // rotation starts on the bad address every other request
		if _, err = client.GetMe(context.Background(), nil); err != nil {
			t.Fatalf("GetMe: %v", err)
		}
	}

	if served.Load() != 3 {
		t.Fatalf("served = %d, want 3", served.Load())
	}
}
