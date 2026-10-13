package gogram_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/darxnet/gogram"
)

// newStubClient returns a client whose transport answers every request with
// body, so benchmarks measure the library, not the network.
func newStubClient(tb testing.TB, body string) *gogram.Client {
	tb.Helper()

	raw := []byte(body)

	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		_, _ = io.Copy(io.Discard, req.Body)

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(bytes.NewReader(raw)),
			Header:     http.Header{},
			Request:    req,
		}, nil
	})

	client, err := gogram.NewClient(testToken,
		gogram.WithRPS(0),
		gogram.WithHTTPClient(&http.Client{Transport: transport}),
	)
	if err != nil {
		tb.Fatal(err)
	}

	return client
}

func BenchmarkSendMessage(b *testing.B) {
	client := newStubClient(b,
		`{"ok":true,"result":{"message_id":1,"date":1700000000,"chat":{"id":7,"type":"private"},"text":"hi"}}`)

	params := &gogram.SendMessageParams{ChatID: "7", Text: "hi"}
	ctx := b.Context()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if _, err := client.SendMessage(ctx, params); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetUpdates_OneUpdate(b *testing.B) {
	client := newStubClient(b,
		`{"ok":true,"result":[{"update_id":1,"message":{"message_id":1,"date":1700000000,`+
			`"chat":{"id":7,"type":"private"},"text":"hi"}}]}`)

	params := &gogram.GetUpdatesParams{Limit: 100, Timeout: 25}
	ctx := b.Context()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if _, err := client.GetUpdates(ctx, params); err != nil {
			b.Fatal(err)
		}
	}
}
