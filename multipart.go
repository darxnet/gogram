package gogram

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"sync"
)

func bytesReader(data []byte) io.Reader {
	return bytes.NewReader(data)
}

// errBodyRewound closes the pipe of an abandoned attempt.
var errBodyRewound = errors.New("gogram: multipart body rewound")

// multipartBody streams a multipart/form-data request body through a pipe:
// write runs in a goroutine per attempt and fills the form while the HTTP
// transport reads it, so an upload is never buffered in memory. Rewind
// starts the body over for a retry; it fails when an [InputFile.File]
// reader was already consumed, see [InputFile.Open].
type multipartBody struct {
	ctx      context.Context //nolint:containedctx // the request context, for InputFile.Open per attempt
	write    func(ctx context.Context, w *multipart.Writer, b *multipartBody) error
	boundary string

	mu       sync.Mutex
	pr       *io.PipeReader
	done     chan error
	err      error // write error of the last attempt
	consumed bool  // a one-shot reader was streamed: no rewind possible
}

func newMultipartBody(
	ctx context.Context,
	write func(ctx context.Context, w *multipart.Writer, b *multipartBody) error,
) *multipartBody {
	var seed [15]byte
	_, _ = rand.Read(seed[:])

	return &multipartBody{ctx: ctx, write: write, boundary: "gogram" + hex.EncodeToString(seed[:])}
}

// ContentType returns the multipart content type with the boundary, which
// stays the same across attempts.
func (b *multipartBody) ContentType() string {
	return "multipart/form-data; boundary=" + b.boundary
}

// Read implements io.Reader, starting the writer goroutine on first use.
func (b *multipartBody) Read(p []byte) (int, error) {
	b.mu.Lock()
	if b.pr == nil {
		b.start()
	}
	pr := b.pr
	b.mu.Unlock()

	return pr.Read(p)
}

func (b *multipartBody) start() {
	pr, pw := io.Pipe()
	w := multipart.NewWriter(pw)
	_ = w.SetBoundary(b.boundary)

	done := make(chan error, 1)
	b.pr, b.done = pr, done

	go func() {
		err := b.write(b.ctx, w, b)
		if err == nil {
			err = w.Close()
		}

		_ = pw.CloseWithError(err)
		done <- err
	}()
}

// Rewind abandons the current attempt and prepares the next one.
func (b *multipartBody) Rewind() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.finish(errBodyRewound)

	if b.consumed {
		return ErrBodyNotRewindable
	}

	return nil
}

// Close stops the writer goroutine of an unfinished attempt.
func (b *multipartBody) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.finish(io.ErrClosedPipe)

	return nil
}

// finish closes the read side so the writer goroutine ends, and records its error.
func (b *multipartBody) finish(reason error) {
	if b.pr == nil {
		return
	}

	_ = b.pr.CloseWithError(reason)
	b.err = <-b.done
	b.pr, b.done = nil, nil
}

// Err returns the error of the last write attempt, if any. A write error
// is the root cause when the transport reports a broken body.
func (b *multipartBody) Err() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.pr != nil {
		b.finish(io.ErrClosedPipe)
	}

	if errors.Is(b.err, errBodyRewound) || errors.Is(b.err, io.ErrClosedPipe) {
		return nil
	}

	return b.err
}

// createFormFileFromInputFile streams an uploaded file into the form.
// Open is preferred: it can be called again on a retry. A plain File
// reader is consumed by the first attempt and marks the body as not
// rewindable.
func createFormFileFromInputFile(
	ctx context.Context,
	w *multipart.Writer,
	b *multipartBody,
	f *InputFile,
	name string,
) error {
	if f.File == nil && f.Open == nil {
		return nil
	}

	f.fieldName = name

	var src io.Reader

	if f.Open != nil {
		rc, err := f.Open(ctx)
		if err != nil {
			return err
		}
		defer rc.Close() //nolint:errcheck

		src = rc
	} else {
		b.consumed = true
		src = f.File
	}

	part, err := w.CreateFormFile(name, f.FileName)
	if err != nil {
		return err
	}

	_, err = io.Copy(part, src)

	return err
}
