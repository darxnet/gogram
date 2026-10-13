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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/darxnet/gogram"
)

const testToken = "123456:ABC-DEF1234567890"

// testResponse is the Bot API response envelope as test servers produce it.
type testResponse struct {
	Parameters  *gogram.ResponseParameters `json:"parameters,omitempty"`
	Description string                     `json:"description,omitempty"`
	Result      json.RawMessage            `json:"result,omitempty"`
	ErrorCode   int                        `json:"error_code,omitempty"`
	OK          bool                       `json:"ok"`
}

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
	if !client.SameToken(testToken) || client.SameToken("other") {
		t.Error("SameToken must match only the client's token")
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

		var params gogram.GetUpdatesParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			t.Errorf("decode getUpdates params: %v", err)
		} else if params.Timeout == 0 {
			t.Errorf("expected non zero timeout")
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

		resp := testResponse{
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
			resp := testResponse{
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
		resp := testResponse{OK: true, Result: json.RawMessage(`[]`)}
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

			resp := testResponse{
				OK:     true,
				Result: json.RawMessage(`{}`),
			}

			return jsonHTTPResponse(t, &resp), nil
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

			resp := testResponse{
				OK:     true,
				Result: json.RawMessage(`{}`),
			}

			return jsonHTTPResponse(t, &resp), nil
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

func TestClient_Stop_DrainsWorkerQueues(t *testing.T) {
	t.Parallel()

	const updatesCount = 3

	var served atomic.Bool
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		var updates []gogram.Update
		if served.CompareAndSwap(false, true) {
			updates = make([]gogram.Update, updatesCount)
			for i := range updates {
				updates[i] = gogram.Update{
					UpdateID: int64(i + 1),
					Message:  &gogram.Message{Text: "drain"},
				}
			}
		}

		resp := testResponse{
			OK:     true,
			Result: json.RawMessage(mustMarshal(t, updates)),
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(mustMarshal(t, &resp))
	}))
	defer server.Close()

	started := make(chan struct{})
	var startedOnce atomic.Bool
	var processed atomic.Int64
	var client *gogram.Client
	var err error

	router := gogram.NewRouter()
	router.HandleOnMessage(func(_ *gogram.Context, _ *gogram.Message) error {
		if startedOnce.CompareAndSwap(false, true) {
			close(started)
			client.Stop()
		}
		time.Sleep(10 * time.Millisecond)
		processed.Add(1)
		return nil
	}, gogram.FilterAny())

	client, err = gogram.NewClient(testToken,
		gogram.WithHost(strings.TrimPrefix(server.URL, "https://")),
		gogram.WithHTTPClient(server.Client()),
		gogram.WithRouter(router),
		gogram.WithWorkers(1),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- client.Start(t.Context(), &gogram.GetUpdatesParams{Limit: updatesCount})
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start processing")
	}

	client.Stop()

	select {
	case err = <-errCh:
		if err != nil {
			t.Fatalf("Start returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Start did not return after Stop")
	}

	if got := processed.Load(); got != updatesCount {
		t.Fatalf("Start returned before worker drained queue: processed %d/%d", got, updatesCount)
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

func jsonHTTPResponse(t *testing.T, payload any) *http.Response {
	t.Helper()

	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     http.StatusText(http.StatusOK),
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(mustMarshal(t, payload))),
	}
}

// TestClient_StartPolling_FatalError validates that Start returns the API
// error when the bot is unauthorized instead of swallowing it.
func TestClient_StartPolling_FatalError(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := testResponse{
			OK:          false,
			ErrorCode:   http.StatusUnauthorized,
			Description: "Unauthorized",
		}
		w.WriteHeader(http.StatusUnauthorized)
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

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	err = client.Start(ctx, nil)
	if !errors.Is(err, gogram.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

// TestClient_StartPolling_LongPollDeadline validates that the HTTP deadline of
// getUpdates exceeds the long-poll timeout: a server that answers exactly at
// the timeout boundary must not trip "context deadline exceeded".
func TestClient_StartPolling_LongPollDeadline(t *testing.T) {
	t.Parallel()

	const pollTimeout = 1 // seconds

	var errCount atomic.Int32

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(pollTimeout*time.Second + 200*time.Millisecond)
		resp := testResponse{OK: true, Result: json.RawMessage(`[]`)}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(mustMarshal(t, &resp))
	}))
	defer server.Close()

	router := gogram.NewRouter()
	router.SetHandlerErr(func(_ *gogram.Context, _ error) { errCount.Add(1) })

	host := strings.TrimPrefix(server.URL, "https://")

	client, err := gogram.NewClient(testToken,
		gogram.WithHost(host),
		gogram.WithHTTPClient(server.Client()),
		gogram.WithRouter(router),
		// Shorter than the poll timeout: must not apply to getUpdates.
		gogram.WithTimeout(500*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	_ = client.Start(ctx, &gogram.GetUpdatesParams{Timeout: pollTimeout})

	if n := errCount.Load(); n != 0 {
		t.Fatalf("expected no polling errors, got %d", n)
	}
}

// TestNewClient_DefaultRouterNotShared validates that clients created without
// WithRouter do not share one Router instance.
func TestNewClient_DefaultRouterNotShared(t *testing.T) {
	t.Parallel()

	a, err := gogram.NewClient(testToken)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	b, err := gogram.NewClient(testToken)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	var handledByA, handledByB atomic.Bool

	ctxA := gogram.NewTestContext(t.Context(), a, &gogram.Update{Message: &gogram.Message{Text: "/x"}})
	ctxB := gogram.NewTestContext(t.Context(), b, &gogram.Update{Message: &gogram.Message{Text: "/x"}})

	a.Router().HandleCommand("/x", func(*gogram.Context, *gogram.Message) error {
		handledByA.Store(true)
		return nil
	})

	a.Router().Process(ctxA)
	b.Router().Process(ctxB)

	if !handledByA.Load() || handledByB.Load() {
		t.Fatalf("handledByA=%v handledByB=%v; want true,false", handledByA.Load(), handledByB.Load())
	}
}

// TestClient_Call_RetriesNonJSONBody validates that an empty body and an HTML
// 5xx page from a proxy are classified as transient: the client sleeps before
// retrying instead of failing on the JSON error.
func TestClient_Call_RetriesNonJSONBody(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status int
		body   string
	}{
		{name: "empty body", status: http.StatusOK, body: ""},
		{name: "bare EOF", status: http.StatusOK, body: "EOF"},
		{name: "json EOF", status: http.StatusBadRequest, body: `{"ok":false,"error_code":400,"description":"EOF"}`},
		{name: "html 502", status: http.StatusBadGateway, body: "<html>502 Bad Gateway</html>"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var requestCount atomic.Int32

			client, err := gogram.NewClient(testToken, gogram.WithHTTPClient(&http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					requestCount.Add(1)

					return &http.Response{
						StatusCode: tc.status,
						Status:     http.StatusText(tc.status),
						Body:       io.NopCloser(strings.NewReader(tc.body)),
						Header:     http.Header{},
						Request:    req,
					}, nil
				}),
			}))
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}

			// The retry sleeps defaultTimeoutOnError; a short deadline cuts it
			// and proves the error was retryable rather than returned at once.
			ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
			defer cancel()

			_, err = client.GetMe(ctx, nil)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("expected the retry sleep to hit the deadline, got %v", err)
			}

			if n := requestCount.Load(); n != 1 {
				t.Fatalf("requests = %d, want 1", n)
			}
		})
	}
}

// TestClient_Start_DeletesStaleWebhook validates that polling recovers from a
// webhook left by a previous deployment: getUpdates conflicts, the client
// deletes the webhook and polls again without reporting an error.
func TestClient_Start_DeletesStaleWebhook(t *testing.T) {
	t.Parallel()

	var (
		calls    []string
		mu       sync.Mutex
		polls    atomic.Int32
		errCount atomic.Int32
	)

	caller := gogram.CallerFunc(func(_ context.Context, req gogram.Request, result any) error {
		mu.Lock()
		calls = append(calls, req.Method)
		mu.Unlock()

		switch req.Method {
		case "getUpdates":
			if polls.Add(1) == 1 {
				return &gogram.APIError{Err: gogram.ErrConflictWebhookActive, Description: "webhook is active"}
			}

			time.Sleep(10 * time.Millisecond)

			return json.Unmarshal([]byte(`[]`), result)
		case "deleteWebhook":
			return json.Unmarshal([]byte(`true`), result)
		}

		return nil
	})

	router := gogram.NewRouter()
	router.SetHandlerErr(func(*gogram.Context, error) { errCount.Add(1) })

	client, err := gogram.NewClient(testToken, gogram.WithCaller(caller), gogram.WithRouter(router), gogram.WithWorkers(1))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	if err = client.Start(ctx, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Start = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(calls) < 3 || calls[0] != "getUpdates" || calls[1] != "deleteWebhook" || calls[2] != "getUpdates" {
		t.Fatalf("calls = %v", calls)
	}

	if errCount.Load() != 0 {
		t.Fatalf("conflict must be handled silently, got %d errors", errCount.Load())
	}
}
