package gogram_test

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/darxnet/gogram"
)

// uploadServer answers the first request with 429 (retry after 1 s) and
// the rest with success, recording the uploaded file content each time.
func uploadServer(t *testing.T, uploads *[]string) *http.Client {
	t.Helper()

	var calls atomic.Int32

	return &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		_, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
		if err != nil {
			t.Errorf("content type: %v", err)
		}

		// A body error (Open failed) is reported by the transport, as net/http does.
		form, err := multipart.NewReader(req.Body, params["boundary"]).ReadForm(1 << 20)
		if err != nil {
			return nil, err
		}

		f, err := form.File["document"][0].Open()
		if err != nil {
			return nil, err
		}

		content, _ := io.ReadAll(f)
		_ = f.Close()

		*uploads = append(*uploads, string(content))

		if calls.Add(1) == 1 {
			resp := testResponse{OK: false, ErrorCode: http.StatusTooManyRequests, Description: "retry",
				Parameters: &gogram.ResponseParameters{RetryAfter: 1}}

			return jsonHTTPResponse(t, &resp), nil
		}

		resp := testResponse{OK: true, Result: []byte(`{"message_id":1,"date":1,"chat":{"id":7,"type":"private"}}`)}

		return jsonHTTPResponse(t, &resp), nil
	})}
}

func TestMultipart_RetryReopensFile(t *testing.T) {
	t.Parallel()

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
		return io.NopCloser(strings.NewReader("payload")), nil
	})

	msg, err := client.SendDocument(t.Context(), &gogram.SendDocumentParams{ChatID: gogram.ChatID{ID: 7}, Document: doc})
	if err != nil || msg == nil || msg.MessageID != 1 {
		t.Fatalf("SendDocument = %+v, %v", msg, err)
	}

	if opens.Load() != 2 || len(uploads) != 2 || uploads[0] != "payload" || uploads[1] != "payload" {
		t.Fatalf("opens = %d, uploads = %q; want the file streamed twice", opens.Load(), uploads)
	}
}

func TestMultipart_OneShotReaderIsNotRetried(t *testing.T) {
	t.Parallel()

	var uploads []string

	client, err := gogram.NewClient(testToken, gogram.WithHTTPClient(uploadServer(t, &uploads)))
	if err != nil {
		t.Fatal(err)
	}

	doc := gogram.FromReader("a.txt", strings.NewReader("once"))

	_, err = client.SendDocument(t.Context(), &gogram.SendDocumentParams{ChatID: gogram.ChatID{ID: 7}, Document: doc})
	if !errors.Is(err, gogram.ErrBodyNotRewindable) || !errors.Is(err, gogram.ErrTooManyRequests) {
		t.Fatalf("err = %v, want ErrBodyNotRewindable joined with the retryable error", err)
	}

	if len(uploads) != 1 || uploads[0] != "once" {
		t.Fatalf("uploads = %q", uploads)
	}
}

func TestMultipart_OpenErrorSurfaces(t *testing.T) {
	t.Parallel()

	var uploads []string

	client, err := gogram.NewClient(testToken, gogram.WithHTTPClient(uploadServer(t, &uploads)))
	if err != nil {
		t.Fatal(err)
	}

	doc := gogram.FromOpener("a.txt", func(context.Context) (io.ReadCloser, error) { return nil, errDenied })

	_, err = client.SendDocument(t.Context(), &gogram.SendDocumentParams{ChatID: gogram.ChatID{ID: 7}, Document: doc})
	if !errors.Is(err, errDenied) {
		t.Fatalf("err = %v, want the Open error", err)
	}
}

func TestFromBytesAndDisk(t *testing.T) {
	t.Parallel()

	f := gogram.FromBytes("b.bin", []byte{1, 2, 3})

	rc, err := f.Open(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	data, _ := io.ReadAll(rc)
	if len(data) != 3 || f.FileName != "b.bin" {
		t.Fatalf("FromBytes = %v %q", data, f.FileName)
	}

	if d := gogram.FromDisk("/tmp/dir/report.pdf"); d.FileName != "report.pdf" || d.Open == nil {
		t.Fatalf("FromDisk = %+v", d)
	}
}
