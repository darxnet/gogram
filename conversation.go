package gogram

import (
	"errors"
	"slices"
)

// ErrNoUser is returned by the state helpers when the update carries no
// user, so there is nobody whose state could be read or changed.
var ErrNoUser = errors.New("gogram: update has no user")

// State returns the FSM state of the current user in the current chat, ""
// when there is none. The value is memoized for the rest of the update.
func (ctx *Context) State() (State, error) {
	if state, ok := ctx.loadState(); ok {
		return state, nil
	}

	f, ok := ctx.FSM()
	if !ok {
		return "", ErrNoUser
	}

	return f.State(ctx.context)
}

// SetState moves the current user to state; "" is the same as FinishState.
func (ctx *Context) SetState(state State) error {
	if state == "" {
		return ctx.FinishState()
	}

	f, ok := ctx.FSM()
	if !ok {
		return ErrNoUser
	}

	return f.Set(ctx, state)
}

// FinishState clears the current user's state.
func (ctx *Context) FinishState() error {
	f, ok := ctx.FSM()
	if !ok {
		return ErrNoUser
	}

	return f.Finish(ctx)
}

// FilterStateIn matches when the user is in any of the given states.
func FilterStateIn(states ...State) Filter {
	states = slices.Clone(states)

	return func(ctx *Context) bool {
		state, ok := ctx.loadState()
		return ok && slices.Contains(states, state)
	}
}

// Conversation registers message handlers per FSM state:
//
//	conv := router.Conversation()
//
//	router.HandleCommand("/register", func(ctx *gogram.Context, _ *gogram.Message) error {
//		return errors.Join(ctx.SetState("name"), ctx.SendMessage("Your name?"))
//	}, "Register")
//
//	conv.On("name", func(ctx *gogram.Context, m *gogram.Message) error {
//		nameKey.Set(ctx, m.Text)
//		return errors.Join(ctx.SetState("age"), ctx.SendMessage("Your age?"))
//	})
//
//	conv.On("age", func(ctx *gogram.Context, m *gogram.Message) error {
//		return errors.Join(ctx.FinishState(), ctx.SendMessage("Done"))
//	}, gogram.FilterRegexp(`^\d+$`))
//
//	conv.Fallback(func(ctx *gogram.Context, _ *gogram.Message) error {
//		return ctx.SendMessage("Please answer the question")
//	})
//
// Commands registered on the router keep priority over conversation states,
// so a "/cancel" command works from any state.
type Conversation struct {
	group  *RouterGroup
	states []State
}

// Conversation returns a conversation builder on this group.
func (rg *RouterGroup) Conversation() *Conversation {
	return &Conversation{group: rg}
}

// On registers handler for messages arriving while the user is in state,
// optionally narrowed by filters.
func (c *Conversation) On(state State, handler func(*Context, *Message) error, filters ...Filter) {
	if !slices.Contains(c.states, state) {
		c.states = append(c.states, state)
	}

	c.group.HandleOnMessage(handler, FilterState(state).And(filters...))
}

// Fallback registers handler for messages arriving in any state registered
// with On that no On handler accepted. Call it after the On handlers.
func (c *Conversation) Fallback(handler func(*Context, *Message) error) {
	c.group.HandleOnMessage(handler, FilterStateIn(c.states...))
}
