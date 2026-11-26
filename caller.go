package gogram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptrace"
	"sync"
	"sync/atomic"
	"time"
)

// Request describes one Bot API call as a [Caller] sees it.
type Request struct {
	// Method is the Bot API method name, e.g. "sendMessage".
	Method string
	// Params is the typed *<Method>Params the call was built from, or nil
	// for [Call] and [Client.Raw]. Transformers may inspect it (chat id for
	// throttling, text for logging) but must not modify it: Body is
	// already encoded.
	Params any
	// Body is the encoded request body. It implements io.Seeker when the
	// call can be retried.
	Body io.Reader
	// ContentType of Body.
	ContentType string
	// Timeout bounds one attempt; zero means no per-attempt deadline.
	Timeout time.Duration
}

// Caller performs a Bot API call and decodes its result into result,
// a pointer to the method's return type.
//
// The client's Caller is a chain: the transformers from [WithTransformers]
// wrap the base caller, which is the HTTP transport unless replaced with
// [WithCaller].
type Caller interface {
	Call(ctx context.Context, req Request, result any) error
}

// CallerFunc adapts a function to the [Caller] interface.
type CallerFunc func(ctx context.Context, req Request, result any) error

// Call implements [Caller].
func (f CallerFunc) Call(ctx context.Context, req Request, result any) error {
	return f(ctx, req, result)
}

// Transformer wraps a [Caller], like HTTP middleware wraps a handler:
//
//	func logCalls(next gogram.Caller) gogram.Caller {
//		return gogram.CallerFunc(func(ctx context.Context, req gogram.Request, result any) error {
//			err := next.Call(ctx, req, result)
//			log.Println(req.Method, err)
//			return err
//		})
//	}
type Transformer func(next Caller) Caller

// envelope is the API response. Result holds a pointer to the typed
// destination, so encoding/json decodes straight into it: one pass, no
// intermediate RawMessage copy. Pooled, see envelopePool.
type envelope struct {
	Result      any                 `json:"result"`
	Parameters  *ResponseParameters `json:"parameters"`
	Description string              `json:"description"`
	ErrorCode   int                 `json:"error_code"`
	OK          bool                `json:"ok"`
}

var envelopePool = sync.Pool{
	New: func() any { return new(envelope) },
}

// headerJSON is the shared Content-Type value for JSON requests: assigning it
// to req.Header skips the per-request []string allocation of Header.Set.
var headerJSON = []string{contentTypeJSON}

// httpCaller is the base [Caller]: one HTTP round trip per attempt, with
// retries of transient errors up to retryLimit times when the body can be
// rewound. A transport error is transient only while the request has not
// been written in full: Telegram has not seen the call, so a retry cannot
// send a message twice.
type httpCaller struct {
	client *Client
}

// Call implements [Caller].
func (h httpCaller) Call(ctx context.Context, req Request, result any) error {
	for attempt := 0; ; attempt++ {
		err := h.roundTrip(ctx, req, result)
		if err == nil {
			return nil
		}

		var unsent *unsentError
		if errors.As(err, &unsent) {
			// The caller sees the transport error itself, never the marker.
			err = unsent.err

			if attempt >= retryLimit || ctx.Err() != nil {
				return err
			}

			if rewindErr := rewind(req.Body); rewindErr != nil {
				return errors.Join(err, rewindErr)
			}

			if sleepErr := sleep(ctx, defaultTimeoutOnError); sleepErr != nil {
				return sleepErr
			}

			continue
		}

		// Both transport-level errors (empty body, HTML 5xx) and API errors
		// go through the same retry classification.
		var retryErr *RetryError
		if !errors.As(err, &retryErr) || attempt >= retryLimit {
			return err
		}

		if rewindErr := rewind(req.Body); rewindErr != nil {
			return errors.Join(retryErr, rewindErr)
		}

		if sleepErr := sleep(ctx, retryErr.RetryAfter); sleepErr != nil {
			return sleepErr
		}
	}
}

// roundTrip performs one HTTP attempt and decodes the result into result.
func (h httpCaller) roundTrip(ctx context.Context, req Request, result any) error {
	c := h.client

	// One trace per attempt: GotConn of the client plus WroteRequest, which
	// tells a transport error before Telegram got the call from one after.
	at := &attemptTrace{}
	at.trace.GotConn = c.trace.GotConn
	at.trace.WroteRequest = func(info httptrace.WroteRequestInfo) {
		if info.Err == nil {
			at.wrote.Store(true)
		}
	}

	ctx = httptrace.WithClientTrace(ctx, &at.trace)

	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.linkPrefix+req.Method, req.Body)
	if err != nil {
		return c.redact(err)
	}

	if req.ContentType == contentTypeJSON {
		httpReq.Header["Content-Type"] = headerJSON
	} else {
		httpReq.Header.Set("Content-Type", req.ContentType)
	}

	resp, err := c.do(httpReq)
	if err != nil {
		if !at.wrote.Load() && !bodyFailed(req.Body) {
			return &unsentError{err: err}
		}

		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	buffer := acquireBuffer()
	defer releaseBuffer(buffer)

	if _, err = buffer.ReadFrom(resp.Body); err != nil {
		return err
	}

	// Telegram occasionally answers with an empty body or the bare text
	// "EOF", and proxies in front of it with an HTML error page: none of
	// these is JSON, all are transient.
	if body := bytes.TrimSpace(buffer.B); len(body) == 0 || string(body) == errTextEOF {
		return &RetryError{Err: ErrEOF, RetryAfter: defaultTimeoutOnError}
	}

	env := envelopePool.Get().(*envelope)
	defer func() {
		*env = envelope{}
		envelopePool.Put(env)
	}()

	env.Result = result

	if err = json.Unmarshal(buffer.B, env); err != nil {
		if resp.StatusCode >= http.StatusInternalServerError {
			return genError(resp.StatusCode, resp.Status, "", nil)
		}

		return err
	}

	if !env.OK {
		return genError(env.ErrorCode, resp.Status, env.Description, env.Parameters)
	}

	return nil
}

// attemptTrace keeps the trace and its flag in one allocation.
type attemptTrace struct {
	trace httptrace.ClientTrace
	wrote atomic.Bool
}

// unsentError marks a transport error that happened before the request was
// written in full, so retrying it cannot repeat the call. httpCaller.Call
// unwraps it; callers never see it.
type unsentError struct {
	err error
}

func (e *unsentError) Error() string { return e.err.Error() }

func (e *unsentError) Unwrap() error { return e.err }

// bodyFailed reports whether the transport error comes from producing the
// body itself (InputFile.Open failed, a reader broke): a local failure that
// a retry would only repeat.
func bodyFailed(body io.Reader) bool {
	b, ok := body.(*multipartBody)

	return ok && b.Err() != nil
}

// rewinder is a body that can start over for a retry, see multipartBody.
type rewinder interface {
	Rewind() error
}

// rewind resets body to its start for a retry.
func rewind(body io.Reader) error {
	switch b := body.(type) {
	case nil:
		return nil
	case rewinder:
		return b.Rewind()
	case io.Seeker:
		_, err := b.Seek(0, io.SeekStart)
		return err
	default:
		return ErrBodyNotRewindable
	}
}

// sleep waits d or until ctx is done.
func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
