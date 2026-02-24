package gogram

import (
	"slices"
	"strings"
)

type (
	HandlerFunc      func(ctx *Context) error
	HandlerFuncErr   func(ctx *Context, err error)
	HandlerFuncPanic func(ctx *Context, v any)

	MiddlewareFunc func(next HandlerFunc) HandlerFunc
)

type route struct {
	filter  Filter
	handler HandlerFunc
}

type Router struct {
	*RouterGroup

	handlersOn [handleOnCount][]route

	handlerDefault HandlerFunc
	handlerErr     HandlerFuncErr
	handlerPanic   HandlerFuncPanic
}

func NewRouter() *Router {
	r := new(Router)
	r.RouterGroup = r.Group()

	return r
}

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

// Use
// must be called before handlers functions
// because middlewares applied one time in place.
func (r *Router) Use(funcs ...MiddlewareFunc) {
	r.middlewares = append(r.middlewares, funcs...)
}

func (r *Router) HandleKeyboardButton(b *KeyboardButton, handler func(ctx *Context, m *Message) error) {
	r.HandleOnMessage(handler, FilterText(b.Text))
}

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

func (r *Router) SetHandlerDefault(handler HandlerFunc) {
	r.handlerDefault = handler
}

func (r *Router) SetHandlerErr(handler HandlerFuncErr) {
	r.handlerErr = handler
}

func (r *Router) SetHandlerPanic(handler HandlerFuncPanic) {
	r.handlerPanic = handler
}

type RouterGroup struct {
	router      *Router
	filter      Filter
	middlewares []MiddlewareFunc
}

func (r *RouterGroup) applyMiddlewares(handler HandlerFunc) HandlerFunc {
	for i := range slices.Backward(r.middlewares) {
		handler = r.middlewares[i](handler)
	}
	return handler
}

func (rg *RouterGroup) Use(funcs ...MiddlewareFunc) {
	rg.middlewares = append(rg.middlewares, funcs...)
}

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
