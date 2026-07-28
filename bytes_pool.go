package gogram

import (
	"github.com/valyala/bytebufferpool"
)

func acquireBuffer() *bytebufferpool.ByteBuffer {
	return bytebufferpool.Get()
}

func releaseBuffer(buffer *bytebufferpool.ByteBuffer) {
	bytebufferpool.Put(buffer)
}
