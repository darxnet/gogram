package gogram

import (
	"encoding"
	"io"
)

// InputFile represents a file upload or an existing file reference.
type InputFile struct {
	FileID   string
	FileURL  string
	File     io.Reader
	FileName string

	fieldName string
}

var (
	_ encoding.TextAppender  = (*InputFile)(nil)
	_ encoding.TextMarshaler = (*InputFile)(nil)
)

// AppendText implements encoding.TextAppender interface.
func (r *InputFile) AppendText(buf []byte) ([]byte, error) {
	switch {
	case r.FileID != "":
		return append(buf, r.FileID...), nil

	case r.FileURL != "":
		return append(buf, r.FileURL...), nil

	case r.File != nil:
		return append(append(buf, "attach://"...), r.fieldName...), nil

	default:
		return nil, nil
	}
}

// MarshalText implements encoding.TextMarshaler interface.
func (r *InputFile) MarshalText() ([]byte, error) {
	return r.AppendText(nil)
}
