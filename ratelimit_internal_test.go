package gogram

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

func TestLimiter_BurstThenSpacing(t *testing.T) {
	t.Parallel()

	const every = 30 * time.Millisecond

	l := newLimiter(every, 3)
	ctx := t.Context()
	start := time.Now()

	for range 3 { // the burst goes through at once
		if err := l.Wait(ctx); err != nil {
			t.Fatal(err)
		}
	}

	if time.Since(start) > every/2 {
		t.Fatalf("burst waited %v", time.Since(start))
	}

	if err := l.Wait(ctx); err != nil { // the fourth waits one interval
		t.Fatal(err)
	}

	if elapsed := time.Since(start); elapsed < every-2*time.Millisecond {
		t.Fatalf("fourth event after %v, want >= %v", elapsed, every)
	}
}

func TestLimiter_DeadlineAndCancel(t *testing.T) {
	t.Parallel()

	l := newLimiter(time.Hour, 1)

	if err := l.Wait(t.Context()); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()

	start := time.Now()

	err := l.Wait(ctx)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 5*time.Millisecond {
		t.Fatalf("Wait = %v after %v, want an immediate DeadlineExceeded", err, time.Since(start))
	}

	// A cancelled wait gives its slot back: the next caller inherits the same delay, not a longer one.
	before := l.tat

	cancelCtx, cancelNow := context.WithCancel(t.Context())
	cancelNow()

	if err = l.Wait(cancelCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait on cancelled ctx = %v", err)
	}

	if !l.tat.Equal(before) {
		t.Fatalf("slot not returned: tat moved from %v to %v", before, l.tat)
	}

	var unlimited *limiter
	if err = unlimited.Wait(t.Context()); err != nil {
		t.Fatalf("nil limiter must allow: %v", err)
	}
}

func TestByteBuffer_ReadFromAndPool(t *testing.T) {
	t.Parallel()

	b := acquireBuffer()
	defer releaseBuffer(b)

	src := make([]byte, 100_000)
	for i := range src {
		src[i] = byte(i)
	}

	n, err := b.ReadFrom(bytesReader(src))
	if err != nil || n != int64(len(src)) || !bytes.Equal(b.B, src) {
		t.Fatalf("ReadFrom = %d, %v", n, err)
	}

	if _, err = b.Write([]byte("tail")); err != nil || len(b.B) != len(src)+4 {
		t.Fatalf("Write: %v, len %d", err, len(b.B))
	}

	huge := &byteBuffer{B: make([]byte, 0, maxPooledBuffer+1)}
	releaseBuffer(huge) // dropped, not pooled
	releaseBuffer(&byteBuffer{B: []byte("x")})

	if got := acquireBuffer(); cap(got.B) > maxPooledBuffer || len(got.B) != 0 {
		t.Fatalf("pooled buffer cap %d len %d", cap(got.B), len(got.B))
	}
}
