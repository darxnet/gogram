package gogram

import (
	"log"
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

var _ Processor = (*Router)(nil)

// Router dispatches updates to registered handlers.
//
// Router is not thread-safe. Register all routes before calling [Client.Start].
type Router struct {
	*RouterGroup

	handlersCommands map[string][]route
	handlersOn       [handleOnCount][]route

	handlerDefault HandlerFunc
	handlerErr     HandlerFuncErr
	handlerPanic   HandlerFuncPanic
}

// NewRouter creates a new Router.
func NewRouter() *Router {
	r := &Router{
		handlersCommands: make(map[string][]route),
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

	end := len(text)

	for i := 1; i < end; i++ {
		if text[i] == ' ' {
			end = i
			break
		}
	}

	for i := 1; i < end; i++ {
		if text[i] == '@' {
			end = i
			break
		}
	}

	return text[:end]
}

// eventsMessages lists the update kinds that may carry a bot command.
var eventsMessages = []handleOn{
	handleOnMessage,
	handleOnChannelPost,
	handleOnBusinessMessage,
}

// Process processes an update.
func (r *Router) Process(ctx *Context) {
	on := ctx.findHandlerOn()

	var command string
	if len(r.handlersCommands) != 0 && slices.Contains(eventsMessages, on) {
		command = r.retrieveCommand(ctx.Text())
	}

	defer func() {
		if v := recover(); v != nil {
			if r.handlerPanic != nil {
				r.handlerPanic(ctx, v)
			} else {
				log.Println("gogram: recovered panic:", v)
			}
		}
	}()

	handleErr := func(ctx *Context, err error) {
		if err != nil && r.handlerErr != nil {
			r.handlerErr(ctx, err)
		}
	}

	// fast path: command map lookup.
	if command != "" {
		if routes, ok := r.handlersCommands[command]; ok {
			for i := range routes {
				if routes[i].filter(ctx) {
					handleErr(ctx, routes[i].handler(ctx))
					return
				}
			}
		}
	}

	// slow path: linear filter scan.
	for i := range r.handlersOn[on] {
		if r.handlersOn[on][i].filter(ctx) {
			handleErr(ctx, r.handlersOn[on][i].handler(ctx))
			return
		}
	}

	// fallback.
	if r.handlerDefault != nil {
		handleErr(ctx, r.handlerDefault(ctx))
	}
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

// HandleCommand registers a command handler using an O(1) map lookup.
//
// The command must not contain spaces. A leading slash is added automatically
// if omitted (e.g. "start" → "/start").
func (rg *RouterGroup) HandleCommand(command string, handler HandlerFunc) {
	if command == "" {
		return
	}

	if strings.Contains(command, " ") {
		panic("gogram: command cannot contain spaces")
	}

	if command[0] != '/' {
		command = "/" + command
	}

	rg.router.handlersCommands[command] = append(rg.router.handlersCommands[command], route{
		filter:  rg.filter,
		handler: rg.applyMiddlewares(handler),
	})
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

// HandleKeyboardButton registers a handler triggered by a reply-keyboard button text.
func (rg *RouterGroup) HandleKeyboardButton(
	b *KeyboardButton,
	handler func(ctx *Context, m *Message) error,
) {
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
	filter := func(ctx *Context) bool {
		cq := ctx.Update().CallbackQuery
		if cq == nil {
			return false
		}

		before, _, _ := strings.Cut(cq.Data, " ")
		return before == b.CallbackData || cq.Data == b.CallbackData
	}

	rg.HandleOnCallbackQuery(handler, filter)
}
