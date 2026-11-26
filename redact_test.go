package gogram_test

import (
	"errors"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"testing"

	"github.com/darxnet/gogram"
)

var errNetwork = errors.New("connection refused")

// failingTransport fails every request the way a network error does, after
// the request was written: such an error is not retried, so the test does
// not wait for retries.
type failingTransport struct{}

func (failingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if trace := httptrace.ContextClientTrace(req.Context()); trace != nil && trace.WroteRequest != nil {
		trace.WroteRequest(httptrace.WroteRequestInfo{})
	}

	return nil, errNetwork
}

func TestNetworkError_RedactsToken(t *testing.T) {
	t.Parallel()

	client, err := gogram.NewClient(testToken,
		gogram.WithHTTPClient(&http.Client{Transport: failingTransport{}}))
	if err != nil {
		t.Fatal(err)
	}

	checks := map[string]error{}

	_, checks["api call"] = client.GetMe(t.Context(), nil)
	checks["invalid method"] = client.Call(t.Context(), "get\x7fMe", nil, nil)
	_, checks["file download"] = client.ReceiveFileReader(t.Context(), &gogram.File{FilePath: "photos/1.jpg"})

	for name, callErr := range checks {
		if callErr == nil {
			t.Fatalf("%s: want an error", name)
		}

		if msg := callErr.Error(); strings.Contains(msg, testToken) || !strings.Contains(msg, "<token>") {
			t.Errorf("%s: token not redacted: %s", name, msg)
		}
	}

	// The cause stays inspectable.
	var ue *url.Error
	if !errors.As(checks["api call"], &ue) || !errors.Is(checks["api call"], errNetwork) {
		t.Fatalf("api call error lost its cause: %v", checks["api call"])
	}
}
