package gogram

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	"github.com/darxnet/goshard"
)

// ChatTargeter is implemented by every generated params type with a chat_id field.
type ChatTargeter interface {
	ChatTarget() ChatID
}

// Telegram's per-chat flood limits, see https://core.telegram.org/bots/faq#my-bot-is-hitting-limits-how-do-i-avoid-this
const (
	defaultPrivateEvery = time.Second     // one message per second in a private chat
	defaultGroupEvery   = 3 * time.Second // 20 messages per minute in a group or channel
	defaultGroupBurst   = 20
	defaultThrottleIdle = 5 * time.Minute
)

// ThrottleConfig configures [Throttle]. Zero fields take Telegram's limits.
type ThrottleConfig struct {
	// PrivateEvery is the minimum interval between messages to one private
	// chat, PrivateBurst how many may go at once. Default: 1 s, burst 1.
	PrivateEvery time.Duration
	PrivateBurst int
	// GroupEvery and GroupBurst do the same for a group, supergroup or
	// channel. Default: 3 s (20 per minute), burst 20.
	GroupEvery time.Duration
	GroupBurst int
	// IdleTTL is how long an unused chat limiter is kept. Default: 5 minutes.
	IdleTTL time.Duration
	// Methods reports whether a method counts toward the per-chat limit.
	// Default: send*, edit*, copy*, forward*, pin/unpin and reactions,
	// except sendChatAction.
	Methods func(method string) bool
}

// chatLimiter is one chat's limiter with the time of its last use.
type chatLimiter struct {
	limiter  *limiter
	lastUsed atomic.Int64 // unix nanoseconds
}

type throttler struct {
	cfg   ThrottleConfig
	chats goshard.ComparableMap[ChatID, *chatLimiter]

	lastSweep atomic.Int64 // unix nanoseconds
	next      Caller
}

// Throttle returns a [Transformer] that enforces Telegram's per-chat flood
// limits in addition to the global [WithRPS] limit: calls to the same chat
// wait for their turn instead of failing with 429. Limiters of idle chats
// are dropped after IdleTTL; no background goroutine is kept.
//
//	client, err := gogram.NewClient(token, gogram.WithTransformers(gogram.Throttle(gogram.ThrottleConfig{})))
func Throttle(cfg ThrottleConfig) Transformer {
	if cfg.PrivateEvery <= 0 {
		cfg.PrivateEvery = defaultPrivateEvery
	}

	if cfg.PrivateBurst == 0 {
		cfg.PrivateBurst = 1
	}

	if cfg.GroupEvery <= 0 {
		cfg.GroupEvery = defaultGroupEvery
	}

	if cfg.GroupBurst == 0 {
		cfg.GroupBurst = defaultGroupBurst
	}

	if cfg.IdleTTL == 0 {
		cfg.IdleTTL = defaultThrottleIdle
	}

	if cfg.Methods == nil {
		cfg.Methods = throttledMethod
	}

	return func(next Caller) Caller {
		t := &throttler{cfg: cfg, next: next}
		t.lastSweep.Store(time.Now().UnixNano())

		return t
	}
}

// throttledMethod is the default ThrottleConfig.Methods.
func throttledMethod(method string) bool {
	if method == "sendChatAction" {
		return false
	}

	for _, prefix := range [...]string{"send", "edit", "copy", "forward", "pin", "unpin", "setMessageReaction"} {
		if strings.HasPrefix(method, prefix) {
			return true
		}
	}

	return false
}

// Call implements [Caller].
func (t *throttler) Call(ctx context.Context, req Request, result any) error {
	target, ok := req.Params.(ChatTargeter)
	if !ok || !t.cfg.Methods(req.Method) {
		return t.next.Call(ctx, req, result)
	}

	chat := target.ChatTarget()
	if chat.IsZero() {
		return t.next.Call(ctx, req, result)
	}

	now := time.Now()

	limiter, _ := t.chats.Compute(chat, func(_ ChatID, current *chatLimiter, loaded bool) (*chatLimiter, bool) {
		if !loaded {
			current = t.newLimiter(chat)
		}

		return current, true
	})

	limiter.lastUsed.Store(now.UnixNano())

	t.maybeSweep(now)

	if err := limiter.limiter.Wait(ctx); err != nil {
		return err
	}

	return t.next.Call(ctx, req, result)
}

// newLimiter picks the limit by chat kind: private chats have positive ids,
// groups, supergroups and channels negative ones or a public username.
func (t *throttler) newLimiter(chat ChatID) *chatLimiter {
	if chat.ID < 0 || chat.Username != "" {
		return &chatLimiter{limiter: newLimiter(t.cfg.GroupEvery, t.cfg.GroupBurst)}
	}

	return &chatLimiter{limiter: newLimiter(t.cfg.PrivateEvery, t.cfg.PrivateBurst)}
}

// maybeSweep drops limiters idle for longer than IdleTTL, at most once per
// IdleTTL, in the calling goroutine: the map holds only recently active
// chats, so the walk is short, and no goroutine outlives the client.
func (t *throttler) maybeSweep(now time.Time) {
	last := t.lastSweep.Load()
	if now.UnixNano()-last < int64(t.cfg.IdleTTL) || !t.lastSweep.CompareAndSwap(last, now.UnixNano()) {
		return
	}

	deadline := now.Add(-t.cfg.IdleTTL).UnixNano()

	for chat, limiter := range t.chats.All() {
		if limiter.lastUsed.Load() < deadline {
			t.chats.CompareAndDelete(chat, limiter)
		}
	}
}
