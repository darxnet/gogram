package gogram

// Processor is an interface for processing updates.
type Processor interface {
	Process(ctx *Context)
	HandleErr(ctx *Context, err error)
	HandlePanic(ctx *Context, v any)
}

// Router returns the update processor of the client: the one passed with
// WithRouter, or the client's own default [Router].
func (c *Client) Router() Processor {
	return c.cfg.router
}

func (c *Client) processUpdate(gogramCtx *Context) {
	defer c.releaseContext(gogramCtx)
	c.cfg.router.Process(gogramCtx)
}
