package gogram

import (
	"io"
	"sync"
)

// maxPooledBuffer is the largest buffer returned to the pool: a rare huge
// response must not pin a megabyte in every pooled slot.
const maxPooledBuffer = 1 << 20

// byteBuffer is a growable byte slice with the io.Writer and io.ReaderFrom
// methods the client needs; B is the content.
type byteBuffer struct {
	B []byte
}

var bufferPool = sync.Pool{
	New: func() any { return &byteBuffer{B: make([]byte, 0, 4096)} },
}

func acquireBuffer() *byteBuffer {
	return bufferPool.Get().(*byteBuffer)
}

func releaseBuffer(b *byteBuffer) {
	if cap(b.B) > maxPooledBuffer {
		return
	}

	b.B = b.B[:0]
	bufferPool.Put(b)
}

// Write implements io.Writer.
func (b *byteBuffer) Write(p []byte) (int, error) {
	b.B = append(b.B, p...)
	return len(p), nil
}

// ReadFrom implements io.ReaderFrom: it reads r to EOF, growing B as needed.
func (b *byteBuffer) ReadFrom(r io.Reader) (int64, error) {
	var total int64

	for {
		if len(b.B) == cap(b.B) {
			b.B = append(b.B, 0)[:len(b.B)] // grow by append's policy
		}

		n, err := r.Read(b.B[len(b.B):cap(b.B)])
		b.B = b.B[:len(b.B)+n]
		total += int64(n)

		if err == io.EOF {
			return total, nil
		}

		if err != nil {
			return total, err
		}
	}
}
