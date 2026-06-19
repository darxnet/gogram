package gogram_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/darxnet/gogram"
)

const testToken = "123456:ABC-DEF1234567890"

// TestNewClient_Success validates successful client creation.
func TestNewClient_Success(t *testing.T) {
	t.Parallel()
	client, err := gogram.NewClient(testToken)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.ID() != int64(123456) {
		t.Errorf("expected ID 123456, got %d", client.ID())
	}
	if client.Token() != testToken {
		t.Errorf("expected token %q, got %q", testToken, client.Token())
	}
}

// TestNewClient_Failure validates error handling for invalid tokens.
func TestNewClient_Failure(t *testing.T) {
	t.Parallel()
	_, err := gogram.NewClient("")
	if !errors.Is(err, gogram.ErrNoToken) {
		t.Fatalf("expected ErrNoToken, got %v", err)
	}

	_, err = gogram.NewClient("invalid-token")
	if !errors.Is(err, gogram.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

// TestClient_StartPolling_Success validates that updates are delivered to handlers
// and polling stops when the context is cancelled.
// Uses a TLS test server because the gogram client always uses https:// URLs.
func TestClient_StartPolling_Success(t *testing.T) {
	t.Parallel()

	updateIDCounter := int64(1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/getUpdates") {
			t.Errorf("unexpected URL path: %s", r.URL.Path)
		}

		// Send one real update on the first request, empty lists thereafter.
		currentID := atomic.AddInt64(&updateIDCounter, 1)
		var updates []gogram.Update
		if currentID == 2 {
			updates = []gogram.Update{
				{UpdateID: currentID, Message: &gogram.Message{Text: "hello"}},
			}
		} else {
			time.Sleep(10 * time.Millisecond) // Prevent busy loop for subsequent requests
		}

		resp := gogram.Response{
			OK:     true,
			Result: json.RawMessage(mustMarshal(t, updates)),
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(mustMarshal(t, &resp))
	}))
	defer server.Close()

	var processedUpdate atomic.Bool
	router := gogram.NewRouter()
	router.HandleOnMessage(func(_ *gogram.Context, msg *gogram.Message) error {
		if msg.Text != "hello" {
			t.Errorf("expected message text %q, got %q", "hello", msg.Text)
		}
		processedUpdate.Store(true)
		return nil
	}, gogram.FilterAny())

	// server.URL is already https://127.0.0.1:<port>; strip the scheme so that
	// WithHost receives only the host:port portion.
	host := strings.TrimPrefix(server.URL, "https://")

	client, err := gogram.NewClient(testToken,
		gogram.WithHost(host),
		// server.Client() carries the TLS transport that trusts the test cert.
		gogram.WithHTTPClient(server.Client()),
		gogram.WithRouter(router),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	err = client.Start(ctx, &gogram.GetUpdatesParams{Limit: 1})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("polling should stop due to context deadline: got %v", err)
	}
	if !processedUpdate.Load() {
		t.Error("update should have been processed")
	}
}

// TestClient_StartPolling_Resilience validates that the polling loop survives
// transient server errors and retries successfully.
//
// We use HTTP 429 (Too Many Requests) with a short RetryAfter (1 s) instead of
// HTTP 500 (Internal Server Error, default 5 s back-off) so that all three
// requests can complete within the 2-second test window.
func TestClient_StartPolling_Resilience(t *testing.T) {
	t.Parallel()

	requestCount := atomic.Int32{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := requestCount.Add(1)
		if count <= 2 {
			// Return 429 with a 1-second retry-after so the rate limiter only
			// pauses for 1 s – safely within the 2-second test budget.
			resp := gogram.Response{
				OK:          false,
				ErrorCode:   http.StatusTooManyRequests,
				Description: "Too Many Requests: retry after 1",
				Result:      json.RawMessage(`null`),
				Parameters:  &gogram.ResponseParameters{RetryAfter: 1},
			}
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write(mustMarshal(t, &resp))
			return
		}
		// Succeed from the third request onward.
		resp := gogram.Response{OK: true, Result: json.RawMessage(`[]`)}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(mustMarshal(t, &resp))
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "https://")

	client, err := gogram.NewClient(testToken,
		gogram.WithHost(host),
		gogram.WithHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
	defer cancel()

	err = client.Start(ctx, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}

	// We expect at least 3 requests: 2 rate-limited, 1+ successful.
	if requestCount.Load() < 3 {
		t.Errorf("expected at least 3 requests, got %d", requestCount.Load())
	}
}

func TestInputFile_MarshalJSON(t *testing.T) {
	t.Parallel()

	v := &gogram.InputFile{
		FileID: "123",
	}

	b, err := json.Marshal(&v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `"123"` {
		t.Errorf("expected %q, got %q", `"123"`, string(b))
	}
}

func TestInputMedia_MarshalJSON(t *testing.T) {
	t.Parallel()

	buf := []byte("test")
	list := []gogram.InputMedia{
		{
			InputMediaDocument: &gogram.InputMediaDocument{
				Media:   gogram.InputFile{File: bytes.NewReader(buf)},
				Caption: "text1",
			},
		},
		{
			InputMediaDocument: &gogram.InputMediaDocument{
				Media:   gogram.InputFile{File: bytes.NewReader(buf)},
				Caption: "text2",
			},
		},
	}

	_, err := json.Marshal(list)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
}

func TestContextMethod_PropagatesHandlerContext(t *testing.T) {
	t.Parallel()

	type traceKey struct{}

	var sawValue atomic.Bool

	httpClient := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if v := req.Context().Value(traceKey{}); v != "trace-value" {
				t.Errorf("expected trace-value, got %v", v)
			}
			sawValue.Store(true)

			resp := gogram.Response{
				OK:     true,
				Result: json.RawMessage(`{}`),
			}

			return jsonHTTPResponse(t, http.StatusOK, &resp), nil
		}),
	}

	client, err := gogram.NewClient(testToken,
		gogram.WithHost("example.invalid"),
		gogram.WithHTTPClient(httpClient),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	baseCtx := context.WithValue(t.Context(), traceKey{}, "trace-value")
	gogramCtx := gogram.NewTestContext(baseCtx, client, nil)

	err = gogramCtx.GetMe()
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if !sawValue.Load() {
		t.Error("transport did not observe the trace value")
	}
}

func TestContextMethod_WithoutCancel_PreservesValuesAfterParentCancel(t *testing.T) {
	t.Parallel()

	type traceKey struct{}

	var sawValue atomic.Bool
	var sawDone atomic.Bool

	httpClient := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if v := req.Context().Value(traceKey{}); v != "trace-value" {
				t.Errorf("expected trace-value, got %v", v)
			}
			if err := req.Context().Err(); err != nil {
				t.Errorf("expected no context error, got %v", err)
			}

			select {
			case <-req.Context().Done():
				sawDone.Store(true)
			default:
			}

			sawValue.Store(true)

			resp := gogram.Response{
				OK:     true,
				Result: json.RawMessage(`{}`),
			}

			return jsonHTTPResponse(t, http.StatusOK, &resp), nil
		}),
	}

	client, err := gogram.NewClient(testToken,
		gogram.WithHost("example.invalid"),
		gogram.WithHTTPClient(httpClient),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	baseCtx, cancel := context.WithCancel(context.WithValue(t.Context(), traceKey{}, "trace-value"))
	cancel()

	gogramCtx := gogram.NewTestContext(baseCtx, client, nil)

	err = gogramCtx.GetMe()
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if !sawValue.Load() {
		t.Error("transport did not observe the trace value")
	}
	if sawDone.Load() {
		t.Error("request context should not have been done during transport")
	}
}

func TestDownloadFile_PreservesTargetOnReadError(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "document.txt")

	if err := os.WriteFile(targetPath, []byte("original"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	httpClient := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if !strings.Contains(req.URL.Path, "/file/bot") {
				t.Errorf("expected URL path to contain /file/bot, got %s", req.URL.Path)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     http.StatusText(http.StatusOK),
				Header:     make(http.Header),
				Body:       &failingReadCloser{payload: []byte("partial")},
			}, nil
		}),
	}

	client, err := gogram.NewClient(testToken,
		gogram.WithHost("example.invalid"),
		gogram.WithHTTPClient(httpClient),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	file := &gogram.File{FilePath: "documents/test.txt"}

	err = client.DownloadFile(t.Context(), file, targetPath)
	if err == nil {
		t.Fatal("expected error from DownloadFile")
	}

	content, readErr := os.ReadFile(targetPath) //nolint:gosec // G304: Potential file inclusion via variable
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	if string(content) != "original" {
		t.Errorf("expected file content %q, got %q", "original", string(content))
	}
}

// mustMarshal is a test helper to simplify JSON marshaling checks.
func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("mustMarshal: %v", err)
	}
	return b
}

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type failingReadCloser struct {
	payload []byte
	sent    bool
}

func (r *failingReadCloser) Read(p []byte) (int, error) {
	if r.sent {
		return 0, io.ErrUnexpectedEOF
	}

	r.sent = true
	return copy(p, r.payload), nil
}

func (*failingReadCloser) Close() error { return nil }

func jsonHTTPResponse(t *testing.T, statusCode int, payload any) *http.Response {
	t.Helper()

	return &http.Response{
		StatusCode: statusCode,
		Status:     http.StatusText(statusCode),
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(mustMarshal(t, payload))),
	}
}
