//go:build go1.25

package gogram_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptrace"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/darxnet/gogram"
)

var (
	errDial  = errors.New("dial tcp: connection refused")
	errReset = errors.New("read tcp: connection reset by peer")
)

// flakyClient fails the first `fails` attempts with err, reporting the
// request as written when written is set, and answers the rest with success.
func flakyClient(t *testing.T, fails int32, written bool, err error, calls *atomic.Int32) *gogram.Client {
	t.Helper()

	client, newErr := gogram.NewClient(testToken, gogram.WithHTTPClient(&http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			n := calls.Add(1)

			if n <= fails {
				if written {
					httptrace.ContextClientTrace(req.Context()).WroteRequest(httptrace.WroteRequestInfo{})
				}

				return nil, err
			}

			resp := testResponse{OK: true, Result: []byte(`{"message_id":1,"date":1,"chat":{"id":7,"type":"private"}}`)}

			return jsonHTTPResponse(t, &resp), nil
		}),
	}))
	if newErr != nil {
		t.Fatalf("NewClient: %v", newErr)
	}

	return client
}

func sendHi(t *testing.T, client *gogram.Client) error {
	t.Helper()

	_, err := client.SendMessage(t.Context(), &gogram.SendMessageParams{ChatID: gogram.ChatID{ID: 7}, Text: "hi"})

	return err
}

// TestCall_RetriesUnsentRequest validates that a transport error before the
// request was written is retried after the usual pause.
func TestCall_RetriesUnsentRequest(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int32

		client := flakyClient(t, 1, false, errDial, &calls)
		start := time.Now()

		if err := sendHi(t, client); err != nil {
			t.Fatalf("SendMessage: %v", err)
		}

		if n := calls.Load(); n != 2 {
			t.Fatalf("requests = %d, want 2", n)
		}

		if waited := time.Since(start); waited != 5*time.Second {
			t.Fatalf("waited %v before the retry, want 5s", waited)
		}
	})
}

// TestCall_DoesNotRetryWrittenRequest validates that an error after the
// request reached the network is returned at once: Telegram may have
// executed the call, and a retry could send the message twice.
func TestCall_DoesNotRetryWrittenRequest(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int32

		client := flakyClient(t, 1, true, errReset, &calls)

		err := sendHi(t, client)
		if !errors.Is(err, errReset) {
			t.Fatalf("err = %v, want the transport error", err)
		}

		if n := calls.Load(); n != 1 {
			t.Fatalf("requests = %d, want 1", n)
		}
	})
}

// TestCall_UnsentRetryLimit validates that unsent retries stop after the
// retry limit and return the transport error itself, not a RetryError.
func TestCall_UnsentRetryLimit(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int32

		client := flakyClient(t, 100, false, errDial, &calls)

		err := sendHi(t, client)
		if !errors.Is(err, errDial) {
			t.Fatalf("err = %v, want the transport error", err)
		}

		var retryErr *gogram.RetryError
		if errors.As(err, &retryErr) {
			t.Fatalf("err = %v is a RetryError; callers must not retry it again", err)
		}

		if n := calls.Load(); n != 6 {
			t.Fatalf("requests = %d, want 6 (one attempt and 5 retries)", n)
		}
	})
}

// TestCall_DoesNotRetryBodyError validates that a failure to produce the
// upload (InputFile.Open) is not retried: it is local, not a network error.
func TestCall_DoesNotRetryBodyError(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var (
			uploads []string
			opens   atomic.Int32
		)

		client, err := gogram.NewClient(testToken, gogram.WithHTTPClient(uploadServer(t, &uploads)))
		if err != nil {
			t.Fatal(err)
		}

		doc := gogram.FromOpener("a.txt", func(context.Context) (io.ReadCloser, error) {
			opens.Add(1)
			return nil, errDenied
		})

		_, err = client.SendDocument(t.Context(), &gogram.SendDocumentParams{ChatID: gogram.ChatID{ID: 7}, Document: doc})
		if !errors.Is(err, errDenied) {
			t.Fatalf("err = %v, want the Open error", err)
		}

		if n := opens.Load(); n != 1 {
			t.Fatalf("opens = %d, want 1", n)
		}
	})
}
