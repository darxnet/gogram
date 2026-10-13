package gogram

// Router returns the client's [Router]: the one passed with [WithRouter] or
// the client's own default.
func (c *Client) Router() *Router {
	return c.cfg.router
}

func (c *Client) processUpdate(gogramCtx *Context) {
	defer c.releaseContext(gogramCtx)
	c.cfg.router.Process(gogramCtx)
}
