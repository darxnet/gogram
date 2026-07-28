package gogram

// Processor is an interface for processing updates.
type Processor interface {
	Process(ctx *Context)
	HandleErr(ctx *Context, err error)
	HandlePanic(ctx *Context, v any)
}

func (c *Client) processUpdate(gogramCtx *Context) {
	defer c.releaseContext(gogramCtx)
	c.cfg.router.Process(gogramCtx)
}
