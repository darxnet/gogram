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
	chatIdent   string          // Chat().Identifier()
	fsm         fsmMemo
}

// detached returns ctx without cancellation but with its values, so API calls
// made from a handler finish even if the update's context is cancelled.
// Built once per update: every generated Context method shares it.
func (ctx *Context) detached() context.Context {
	if ctx.detachedCtx == nil {
		ctx.detachedCtx = context.WithoutCancel(ctx.context)
	}

	return ctx.detachedCtx
}

// chatIdentifier returns Chat().Identifier() memoized per update: the
// strconv result is one allocation shared by every API call of the handler.
func (ctx *Context) chatIdentifier() string {
	if ctx.chatIdent == "" {
		if c := ctx.Chat(); c != nil {
			ctx.chatIdent = c.Identifier()
		}
	}

	return ctx.chatIdent
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

// SetValue sets a value in the context.
func (ctx *Context) SetValue(key, value any) {
	if ctx.values == nil {
		ctx.values = make(map[any]any, 1)
	}

	ctx.values[key] = value
}

// Value returns the value associated with this context for key, or nil
// if no value is associated with key.
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
