package gogram

import (
	"context"
	"encoding"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// InputFile is a file to send: an existing Telegram file by FileID, a
// public URL Telegram downloads itself (FileURL), or an upload streamed
// from the bot. Uploads come from Open, called once per attempt so that a
// failed request can be retried, or from File, a reader that is consumed
// by the first attempt and cannot be retried.
//
//	gogram.FromDisk("/data/report.pdf")
//	gogram.FromOpener("photo.jpg", func(ctx context.Context) (io.ReadCloser, error) {
//		return s3.GetObject(ctx, bucket, key) // any S3-compatible client: AWS, MinIO, Backblaze
//	})
type InputFile struct {
	FileID   string
	FileURL  string
	File     io.Reader
	Open     func(ctx context.Context) (io.ReadCloser, error)
	FileName string

	fieldName string
}

var (
	_ encoding.TextAppender  = (*InputFile)(nil)
	_ encoding.TextMarshaler = (*InputFile)(nil)
)

// FromDisk uploads the file at path; the file name is its base name.
func FromDisk(path string) InputFile {
	return InputFile{
		FileName: filepath.Base(path),
		Open: func(context.Context) (io.ReadCloser, error) {
			return os.Open(path) //nolint:gosec // G304: the caller chooses the path
		},
	}
}

// FromURL streams the file at url through the bot, for URLs Telegram
// cannot fetch itself (private networks, signed links). client may be nil
// for http.DefaultClient. For public URLs prefer InputFile{FileURL: url}.
func FromURL(url, name string, client *http.Client) InputFile {
	if client == nil {
		client = http.DefaultClient
	}

	return InputFile{
		FileName: name,
		Open: func(ctx context.Context) (io.ReadCloser, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
			if err != nil {
				return nil, err
			}

			resp, err := client.Do(req)
			if err != nil {
				return nil, err
			}

			if resp.StatusCode != http.StatusOK {
				_ = resp.Body.Close()
				return nil, errors.Join(ErrFileDownload, NewError(resp.StatusCode, resp.Status))
			}

			return resp.Body, nil
		},
	}
}

// FromOpener uploads whatever open returns; open is called once per attempt.
func FromOpener(name string, open func(ctx context.Context) (io.ReadCloser, error)) InputFile {
	return InputFile{FileName: name, Open: open}
}

// FromReader uploads r once; a failed request is not retried. Use it for
// data that only exists in a stream, see FromOpener otherwise.
func FromReader(name string, r io.Reader) InputFile {
	return InputFile{FileName: name, File: r}
}

// FromBytes uploads data; retries re-read it.
func FromBytes(name string, data []byte) InputFile {
	return FromOpener(name, func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(bytesReader(data)), nil
	})
}

// ErrFileDownload indicates that FromURL could not fetch the file.
var ErrFileDownload = errors.New("gogram: file download failed")

// AppendText implements encoding.TextAppender interface.
func (r *InputFile) AppendText(buf []byte) ([]byte, error) {
	switch {
	case r.FileID != "":
		return append(buf, r.FileID...), nil

	case r.FileURL != "":
		return append(buf, r.FileURL...), nil

	case r.File != nil || r.Open != nil:
		return append(append(buf, "attach://"...), r.fieldName...), nil

	default:
		return buf, nil
	}
}

// MarshalText implements encoding.TextMarshaler interface.
func (r *InputFile) MarshalText() ([]byte, error) {
	return r.AppendText(nil)
}
