package gogram

import (
	"context"
	"sync"
	"time"
)

var contextPool = sync.Pool{
	New: func() any {
		return &Context{
			values: map[any]any{},
		}
	},
}

func (c *Client) acquireContext(ctx context.Context, update *Update) *Context {
	v := contextPool.Get().(*Context)
	v.context = ctx
	v.client = c
	v.update = update
	return v
}

func (c *Client) releaseContext(ctx *Context) {
	ctx.context = nil
	ctx.client = nil
	ctx.update = nil
	clear(ctx.values)
	contextPool.Put(ctx)
}

// Context wraps context.Context and adds Telegram-specific methods.
type Context struct {
	context context.Context
	client  *Client
	update  *Update
	values  map[any]any
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
