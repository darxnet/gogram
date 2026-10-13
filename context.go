package gogram

import (
	"context"
	"sync"
	"time"
)

var contextPool = sync.Pool{
	New: func() any {
		return &Context{}
	},
}

// NewTestContext creates a new context for testing purposes.
func NewTestContext(ctx context.Context, c *Client, u *Update) *Context {
	return c.acquireContext(ctx, u)
}

func (c *Client) acquireContext(ctx context.Context, update *Update) *Context {
	v := contextPool.Get().(*Context)
	v.context = ctx
	v.client = c
	v.update = update

	return v
}

func (c *Client) releaseContext(ctx *Context) {
	clear(ctx.values) // keep the map, drop the entries
	*ctx = Context{values: ctx.values}
	contextPool.Put(ctx)
}

// Context wraps context.Context and adds Telegram-specific methods.
//
// A Context is pooled: it must not be retained after the handler returns.
type Context struct {
	context context.Context
	client  *Client
	update  *Update
	values  map[any]any

	// Per-update memos: computed on first use, dropped on release.
	detachedCtx context.Context // context without cancellation for API calls
	chatTargetMemo   string          // Chat().Target()
	fsm         fsmMemo
}

// Detached returns ctx without cancellation but with its values, so API calls
// made from a handler finish even if the update's context is cancelled
// (worker shutdown, webhook client gone). Built once per update: every
// generated Context method shares it; pass it to Client methods called
// directly from a handler.
func (ctx *Context) Detached() context.Context {
	if ctx.detachedCtx == nil {
		ctx.detachedCtx = context.WithoutCancel(ctx.context)
	}

	return ctx.detachedCtx
}

// chatTarget returns Chat().Target() memoized per update: the
// strconv result is one allocation shared by every API call of the handler.
func (ctx *Context) chatTarget() string {
	if ctx.chatTargetMemo == "" {
		if c := ctx.Chat(); c != nil {
			ctx.chatTargetMemo = c.Target()
		}
	}

	return ctx.chatTargetMemo
}

// chatID returns Chat().ID or 0.
func (ctx *Context) chatID() int64 {
	if c := ctx.Chat(); c != nil {
		return c.ID
	}

	return 0
}

// userID returns User().ID or 0.
func (ctx *Context) userID() int64 {
	if u := ctx.User(); u != nil {
		return u.ID
	}

	return 0
}

// directMessagesTopicID returns the direct messages topic of the current message, or 0.
func (ctx *Context) directMessagesTopicID() int64 {
	if m := ctx.Message(); m != nil && m.DirectMessagesTopic != nil {
		return m.DirectMessagesTopic.TopicID
	}

	return 0
}

// inlineMessageID returns the inline message the callback query refers to, or "".
func (ctx *Context) inlineMessageID() string {
	if ctx.update != nil && ctx.update.CallbackQuery != nil {
		return ctx.update.CallbackQuery.InlineMessageID
	}

	return ""
}

// messageField returns get(Message()) or the zero value when there is no message.
// Generated builders use it to fill params from the current message.
func messageField[T any](ctx *Context, get func(*Message) T) T {
	if m := ctx.Message(); m != nil {
		return get(m)
	}

	var zero T

	return zero
}

// updateField returns get(Update()) or the zero value when there is no update.
func updateField[T any](ctx *Context, get func(*Update) T) T {
	if ctx.update != nil {
		return get(ctx.update)
	}

	var zero T

	return zero
}

// Deadline returns the time when work done on behalf of this context
// should be cancelled.
func (ctx *Context) Deadline() (deadline time.Time, ok bool) {
	return ctx.context.Deadline()
}

// Done returns a channel that's closed when work done on behalf of this
// context should be cancelled.
func (ctx *Context) Done() <-chan struct{} {
	return ctx.context.Done()
}

// Err returns a non-nil error value after Done is closed.
func (ctx *Context) Err() error {
	return ctx.context.Err()
}

// Key is a typed per-update value slot:
//
//	var userKey = gogram.NewKey[*User]("user")
//	userKey.Set(ctx, u)
//	u, ok := userKey.Get(ctx)
//
// Values live until the handler returns; [Context.Value] also finds them.
type Key[T any] struct {
	name string
}

// NewKey creates a typed context key; name is for debugging only.
func NewKey[T any](name string) *Key[T] {
	return &Key[T]{name: name}
}

// String returns the key name.
func (k *Key[T]) String() string {
	return k.name
}

// Set stores value in ctx under k.
func (k *Key[T]) Set(ctx *Context, value T) {
	if ctx.values == nil {
		ctx.values = make(map[any]any, 1)
	}

	ctx.values[k] = value
}

// Get returns the value stored under k and whether it was set.
func (k *Key[T]) Get(ctx *Context) (value T, ok bool) {
	value, ok = ctx.values[k].(T)
	return value, ok
}

// Value returns the value associated with this context for key, or nil
// if no value is associated with key. Keys set with [Key.Set] are found
// first, then the parent context is consulted.
func (ctx *Context) Value(key any) any {
	if value, ok := ctx.values[key]; ok {
		return value
	}

	return ctx.context.Value(key)
}

// Client returns the client that created this context.
func (ctx *Context) Client() *Client {
	return ctx.client
}

// Update returns the update that triggered this context.
func (ctx *Context) Update() *Update {
	return ctx.update
}

// Context returns the underlying context.Context.
func (ctx *Context) Context() context.Context {
	return ctx.context
}
