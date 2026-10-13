package gogram

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	"github.com/darxnet/goshard"
	"golang.org/x/time/rate"
)

// ChatTargeter is implemented by every generated params type with a chat_id field.
type ChatTargeter interface {
	ChatTarget() ChatID
}

// Telegram's per-chat flood limits, see https://core.telegram.org/bots/faq#my-bot-is-hitting-limits-how-do-i-avoid-this
const (
	defaultPrivateRate  = rate.Limit(1)        // one message per second in a private chat
	defaultGroupRate    = rate.Limit(20.0 / 60) // 20 messages per minute in a group or channel
	defaultGroupBurst   = 20
	defaultThrottleIdle = 5 * time.Minute
)

// ThrottleConfig configures [Throttle]. Zero fields take Telegram's limits.
type ThrottleConfig struct {
	// PrivateRate and PrivateBurst limit messages to one private chat.
	// Default: 1 per second, burst 1.
	PrivateRate  rate.Limit
	PrivateBurst int
	// GroupRate and GroupBurst limit messages to one group, supergroup or
	// channel. Default: 20 per minute, burst 20.
	GroupRate  rate.Limit
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
	limiter  *rate.Limiter
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
	if cfg.PrivateRate == 0 {
		cfg.PrivateRate = defaultPrivateRate
	}

	if cfg.PrivateBurst == 0 {
		cfg.PrivateBurst = 1
	}

	if cfg.GroupRate == 0 {
		cfg.GroupRate = defaultGroupRate
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
		return &chatLimiter{limiter: rate.NewLimiter(t.cfg.GroupRate, t.cfg.GroupBurst)}
	}

	return &chatLimiter{limiter: rate.NewLimiter(t.cfg.PrivateRate, t.cfg.PrivateBurst)}
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
