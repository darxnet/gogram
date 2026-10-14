package gogram

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// limiter spaces events at least `every` apart while letting `burst` of
// them through at once (GCRA): tat is the theoretical arrival time of the
// next event, allowed up to tolerance before it. A nil limiter allows all.
type limiter struct {
	every     time.Duration
	tolerance time.Duration // (burst-1) * every

	mu  sync.Mutex
	tat time.Time
}

// newLimiter returns a limiter for one event per `every` with the given
// burst; every <= 0 returns nil (no limit).
func newLimiter(every time.Duration, burst int) *limiter {
	if every <= 0 {
		return nil
	}

	return &limiter{every: every, tolerance: time.Duration(max(burst, 1)-1) * every}
}

// reserve claims the next slot and returns how long to wait for it.
func (l *limiter) reserve(now time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	tat := l.tat
	if tat.Before(now) {
		tat = now
	}

	delay := max(tat.Sub(now)-l.tolerance, 0)

	l.tat = tat.Add(l.every)

	return delay
}

// cancel gives a reserved slot back.
func (l *limiter) cancel() {
	l.mu.Lock()
	l.tat = l.tat.Add(-l.every)
	l.mu.Unlock()
}

// Wait blocks until the next slot or ctx is done. When ctx has a deadline
// the wait cannot meet, it fails at once with context.DeadlineExceeded.
func (l *limiter) Wait(ctx context.Context) error {
	if l == nil {
		return nil
	}

	now := time.Now()

	if deadline, ok := ctx.Deadline(); ok {
		l.mu.Lock()
		tat := l.tat
		l.mu.Unlock()

		if tat.Before(now) {
			tat = now
		}

		if wait := tat.Sub(now) - l.tolerance; wait > 0 && now.Add(wait).After(deadline) {
			return fmt.Errorf("gogram: rate limit wait of %v exceeds the context deadline: %w", wait, context.DeadlineExceeded)
		}
	}

	delay := l.reserve(now)
	if delay == 0 {
		return nil
	}

	if err := sleep(ctx, delay); err != nil {
		l.cancel()
		return err
	}

	return nil
}
