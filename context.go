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

type Context struct {
	context context.Context
	client  *Client
	update  *Update
	values  map[any]any
}

func (ctx *Context) Deadline() (deadline time.Time, ok bool) {
	return ctx.context.Deadline()
}

func (ctx *Context) Done() <-chan struct{} {
	return ctx.context.Done()
}

func (ctx *Context) Err() error {
	return ctx.context.Err()
}

func (ctx *Context) SetValue(key, value any) {
	ctx.values[key] = value
}

func (ctx *Context) Value(key any) any {
	if value, ok := ctx.values[key]; ok {
		return value
	}

	return ctx.context.Value(key)
}

func (ctx *Context) Client() *Client {
	return ctx.client
}

func (ctx *Context) Update() *Update {
	return ctx.update
}

func (ctx *Context) Context() context.Context {
	return ctx.context
}
