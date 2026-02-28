package gogram

import (
	"slices"
	"strings"
)

type (
	// HandlerFunc is a function that handles a request.
	HandlerFunc func(ctx *Context) error
	// HandlerFuncErr is a function that handles an error.
	HandlerFuncErr func(ctx *Context, err error)
	// HandlerFuncPanic is a function that handles a panic.
	HandlerFuncPanic func(ctx *Context, v any)

	// MiddlewareFunc is a function that wraps a HandlerFunc.
	MiddlewareFunc func(next HandlerFunc) HandlerFunc
)

type route struct {
	filter  Filter
	handler HandlerFunc
}

// Router dispatches updates to registered handlers.
type Router struct {
	*RouterGroup

	handlersOn [handleOnCount][]route

	handlerDefault HandlerFunc
	handlerErr     HandlerFuncErr
	handlerPanic   HandlerFuncPanic
}

// NewRouter creates a new Router.
func NewRouter() *Router {
	r := new(Router)
	r.RouterGroup = &RouterGroup{router: r}

	return r
}

// Process processes an update.
func (r *Router) Process(ctx *Context) {
	defer func() {
		if r.handlerPanic != nil {
			if v := recover(); v != nil {
				r.handlerPanic(ctx, v)
			}
		}
	}()

	for _, route := range r.handlersOn[ctx.findHandlerOn()] {
		if route.filter(ctx) {
			err := route.handler(ctx)
			if err != nil && r.handlerErr != nil {
				r.handlerErr(ctx, err)
			}
			return
		}
	}

	if r.handlerDefault != nil {
		err := r.handlerDefault(ctx)
		if err != nil && r.handlerErr != nil {
			r.handlerErr(ctx, err)
		}
	}
}

// Use adds middleware to the router.
// It must be called before handlers, as middlewares are applied in place.
func (r *Router) Use(funcs ...MiddlewareFunc) {
	r.middlewares = append(r.middlewares, funcs...)
}

// HandleKeyboardButton registers a handler for a keyboard button.
func (r *Router) HandleKeyboardButton(b *KeyboardButton, handler func(ctx *Context, m *Message) error) {
	r.HandleOnMessage(handler, FilterText(b.Text))
}

// HandleInlineKeyboardButton registers a handler for an inline keyboard button.
func (r *Router) HandleInlineKeyboardButton(
	b *InlineKeyboardButton,
	handler func(ctx *Context, cq *CallbackQuery,
	) error) {
	filter := func(ctx *Context) bool {
		cq := ctx.Update().CallbackQuery
		if cq == nil {
			return false
		}

		before, _, found := strings.Cut(cq.Data, " ")

		return cq.Data == b.CallbackData || (found && before == b.CallbackData)
	}

	r.HandleOnCallbackQuery(handler, filter)
}

// SetHandlerDefault sets the default handler for updates that don't match any route.
func (r *Router) SetHandlerDefault(handler HandlerFunc) {
	r.handlerDefault = handler
}

// SetHandlerErr sets the error handler for the router.
func (r *Router) SetHandlerErr(handler HandlerFuncErr) {
	r.handlerErr = handler
}

// SetHandlerPanic sets the panic handler for the router.
func (r *Router) SetHandlerPanic(handler HandlerFuncPanic) {
	r.handlerPanic = handler
}

// RouterGroup allows to group handlers with common middlewares and filters.
type RouterGroup struct {
	router      *Router
	filter      Filter
	middlewares []MiddlewareFunc
}

func (rg *RouterGroup) applyMiddlewares(handler HandlerFunc) HandlerFunc {
	for i := range slices.Backward(rg.middlewares) {
		handler = rg.middlewares[i](handler)
	}
	return handler
}

// Use adds middleware to the group.
func (rg *RouterGroup) Use(funcs ...MiddlewareFunc) {
	rg.middlewares = append(rg.middlewares, funcs...)
}

// Group creates a new router group with the provided filters.
func (rg *RouterGroup) Group(filters ...Filter) *RouterGroup {
	combined := func(ctx *Context) bool {
		if !rg.filter(ctx) {
			return false
		}

		for _, f := range filters {
			if !f(ctx) {
				return false
			}
		}
		return true
	}

	return &RouterGroup{
		router:      rg.router,
		filter:      combined,
		middlewares: rg.middlewares,
	}
}
