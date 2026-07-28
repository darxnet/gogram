package gogram

import (
	"log"
	"slices"
	"strings"
)

const commandHandlersMask = 1<<handleOnMessage | 1<<handleOnChannelPost | 1<<handleOnBusinessMessage

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

var _ Processor = (*Router)(nil)

// Router dispatches updates to registered handlers.
//
// Router is not thread-safe. Register all routes before calling [Client.Start].
type Router struct {
	*RouterGroup

	handlersCommands  map[string][]route
	handlersCallbacks map[string][]route

	handlersOn [handleOnCount][]route

	handlerDefault HandlerFunc
	handlerErr     HandlerFuncErr
	handlerPanic   HandlerFuncPanic
}

// NewRouter creates a new Router.
func NewRouter() *Router {
	r := &Router{
		handlersCommands:  make(map[string][]route),
		handlersCallbacks: make(map[string][]route),
	}

	r.RouterGroup = &RouterGroup{
		router: r,
		filter: func(*Context) bool { return true },
	}

	return r
}

// HandleErr implements [Processor].
func (r *Router) HandleErr(ctx *Context, err error) {
	if r.handlerErr != nil {
		r.handlerErr(ctx, err)
	}
}

// HandlePanic implements [Processor].
func (r *Router) HandlePanic(ctx *Context, v any) {
	if r.handlerPanic != nil {
		r.handlerPanic(ctx, v)
	}
}

// retrieveCommand extracts the command name from a message text.
//
//	"/start"           → "/start"
//	"/start payload"   → "/start"
//	"/start@bot"       → "/start"
func (r *Router) retrieveCommand(text string) string {
	if text == "" || text[0] != '/' {
		return ""
	}

	for i := 1; i < len(text); i++ {
		if c := text[i]; c == ' ' || c == '@' {
			return text[:i]
		}
	}

	return text
}

// Process processes an update.
//
//nolint:gocognit // Dispatching Telegram's mutually exclusive update variants requires one explicit decision chain.
func (r *Router) Process(ctx *Context) {
	defer r.handlePanic(ctx)

	on := ctx.findHandlerOn()

	// fast path: command map lookup.
	if len(r.handlersCommands) != 0 && (1<<on)&commandHandlersMask != 0 {
		if command := r.retrieveCommand(ctx.Text()); command != "" {
			if routes, ok := r.handlersCommands[command]; ok {
				for i := range routes {
					if routes[i].filter(ctx) {
						r.handleErr(ctx, routes[i].handler(ctx))
						return
					}
				}
			}
		}
	}

	// fast path: callback data map lookup.
	if len(r.handlersCallbacks) != 0 && on == handleOnCallbackQuery {
		cq := ctx.Update().CallbackQuery

		if routes, ok := r.handlersCallbacks[cq.Data]; ok {
			for i := range routes {
				if routes[i].filter(ctx) {
					r.handleErr(ctx, routes[i].handler(ctx))
					return
				}
			}
		}

		key, _, _ := strings.Cut(cq.Data, " ")
		if routes, ok := r.handlersCallbacks[key]; ok {
			for i := range routes {
				if routes[i].filter(ctx) {
					r.handleErr(ctx, routes[i].handler(ctx))
					return
				}
			}
		}
	}

	// slow path: linear filter scan.
	for i := range r.handlersOn[on] {
		if r.handlersOn[on][i].filter(ctx) {
			r.handleErr(ctx, r.handlersOn[on][i].handler(ctx))
			return
		}
	}

	// fallback.
	if r.handlerDefault != nil {
		r.handleErr(ctx, r.handlerDefault(ctx))
	}
}

func (r *Router) handleErr(ctx *Context, err error) {
	if err != nil && r.handlerErr != nil {
		r.handlerErr(ctx, err)
	}
}

func (r *Router) handlePanic(ctx *Context) {
	if v := recover(); v != nil {
		if r.handlerPanic != nil {
			r.handlerPanic(ctx, v)
		} else {
			log.Println("gogram: recovered panic:", v)
		}
	}
}

// SetHandlerDefault sets the default handler for updates that don't match any route.
func (r *Router) SetHandlerDefault(handler HandlerFunc) {
	r.handlerDefault = r.applyMiddlewares(handler)
}

// SetHandlerErr sets the error handler for the router.
func (r *Router) SetHandlerErr(handler HandlerFuncErr) {
	r.handlerErr = handler
}

// SetHandlerPanic sets the panic handler for the router.
func (r *Router) SetHandlerPanic(handler HandlerFuncPanic) {
	r.handlerPanic = handler
}

// RouterGroup allows grouping handlers under shared filters and middlewares.
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

// Use appends middleware to this group.
//
// Middlewares are applied in registration order and baked into each handler at
// the moment that handler is registered. Calling Use after registering handlers
// has no effect on those handlers.
//
// Must not be called after [Client.Start].
func (rg *RouterGroup) Use(funcs ...MiddlewareFunc) {
	rg.middlewares = append(rg.middlewares, funcs...)
}

// Group creates a child RouterGroup that inherits this group's middlewares and
// combines its filter with the provided filters (all must pass).
func (rg *RouterGroup) Group(filters ...Filter) *RouterGroup {
	combined := func(ctx *Context) bool {
		if rg.filter != nil && !rg.filter(ctx) {
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
		middlewares: slices.Clip(rg.middlewares),
	}
}

func (rg *RouterGroup) handleOn(on handleOn, handler HandlerFunc, filters ...Filter) {
	combined := func(ctx *Context) bool {
		if !rg.filter(ctx) {
			return false
		}
		for _, fn := range filters {
			if !fn(ctx) {
				return false
			}
		}
		return true
	}

	rg.router.handlersOn[on] = append(rg.router.handlersOn[on], route{
		filter:  combined,
		handler: rg.applyMiddlewares(handler),
	})
}

// HandleCommand registers a command handler using an O(1) map lookup.
//
// The command must not contain spaces. A leading slash is added automatically
// if omitted (e.g. "start" → "/start").
func (rg *RouterGroup) HandleCommand(command string, handler func(*Context, *Message) error) {
	if command == "" {
		return
	}

	if strings.Contains(command, " ") {
		panic("gogram: command cannot contain spaces")
	}

	if command[0] != '/' {
		command = "/" + command
	}

	fn := func(ctx *Context) error {
		return handler(ctx, ctx.Update().Message)
	}

	rg.router.handlersCommands[command] = append(rg.router.handlersCommands[command], route{
		filter:  rg.filter,
		handler: rg.applyMiddlewares(fn),
	})
}

// HandleKeyboardButton registers a handler triggered by a reply-keyboard button text.
func (rg *RouterGroup) HandleKeyboardButton(
	b *KeyboardButton,
	handler func(ctx *Context, m *Message) error,
) {
	if command := rg.router.retrieveCommand(b.Text); command != "" {
		rg.HandleCommand(command, handler)
		return
	}

	rg.HandleOnMessage(handler, FilterText(b.Text))
}

// HandleInlineKeyboardButton registers a handler triggered when an inline
// keyboard button is pressed. It matches callbacks whose data equals
// b.CallbackData exactly or whose data starts with b.CallbackData followed by
// a space (i.e. "data payload" pattern).
func (rg *RouterGroup) HandleInlineKeyboardButton(
	b *InlineKeyboardButton,
	handler func(ctx *Context, cq *CallbackQuery) error,
) {
	fn := func(ctx *Context) error {
		return handler(ctx, ctx.Update().CallbackQuery)
	}

	rg.router.handlersCallbacks[b.CallbackData] = append(rg.router.handlersCallbacks[b.CallbackData], route{
		filter:  rg.filter,
		handler: rg.applyMiddlewares(fn),
	})
}
