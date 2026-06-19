package gogram_test

import (
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/darxnet/gogram"
)

// TestRouter_Process_Simple tests basic handler dispatching.
func TestRouter_Process_Simple(t *testing.T) {
	t.Parallel()
	r := gogram.NewRouter()
	var handled atomic.Bool

	r.HandleOnMessage(func(_ *gogram.Context, msg *gogram.Message) error {
		if msg.Text != "hello" {
			t.Errorf("expected message text %q, got %q", "hello", msg.Text)
		}
		handled.Store(true)
		return nil
	})

	update := &gogram.Update{UpdateID: 1, Message: &gogram.Message{Text: "hello"}}
	client, err := gogram.NewClient(testToken)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	gogramCtx := gogram.NewTestContext(t.Context(), client, update)

	r.Process(gogramCtx)
	if !handled.Load() {
		t.Error("handler was not invoked")
	}
}

// TestRouter_Process_NoMatch tests the default handler.
func TestRouter_Process_NoMatch(t *testing.T) {
	t.Parallel()
	r := gogram.NewRouter()
	var defaultHandled atomic.Bool

	r.SetHandlerDefault(func(_ *gogram.Context) error {
		defaultHandled.Store(true)
		return nil
	})

	update := &gogram.Update{UpdateID: 1, EditedMessage: &gogram.Message{Text: "edited"}}
	client, err := gogram.NewClient(testToken)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	gogramCtx := gogram.NewTestContext(t.Context(), client, update)

	r.Process(gogramCtx)
	if !defaultHandled.Load() {
		t.Error("default handler was not invoked")
	}
}

// TestRouter_Process_Filter tests that filters are applied correctly.
func TestRouter_Process_Filter(t *testing.T) {
	t.Parallel()
	r := gogram.NewRouter()
	var rightHandler, wrongHandler atomic.Bool

	// This handler should run
	r.HandleOnMessage(func(_ *gogram.Context, _ *gogram.Message) error {
		rightHandler.Store(true)
		return nil
	}, gogram.FilterText("correct"))

	// This handler should NOT run
	r.HandleOnMessage(func(_ *gogram.Context, _ *gogram.Message) error {
		wrongHandler.Store(true)
		return nil
	}, gogram.FilterText("incorrect"))

	update := &gogram.Update{UpdateID: 1, Message: &gogram.Message{Text: "correct"}}
	client, err := gogram.NewClient(testToken)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	gogramCtx := gogram.NewTestContext(t.Context(), client, update)

	r.Process(gogramCtx)
	if !rightHandler.Load() {
		t.Error("right handler was not invoked")
	}
	if wrongHandler.Load() {
		t.Error("wrong handler should not have been invoked")
	}
}

// TestRouter_Middleware tests middleware execution order.
func TestRouter_Middleware(t *testing.T) {
	t.Parallel()
	r := gogram.NewRouter()
	executionOrder := []string{}
	var mu sync.Mutex

	record := func(name string) {
		mu.Lock()
		defer mu.Unlock()
		executionOrder = append(executionOrder, name)
	}

	r.Use(func(next gogram.HandlerFunc) gogram.HandlerFunc {
		return func(ctx *gogram.Context) error {
			record("middleware 1 start")
			err := next(ctx)
			record("middleware 1 end")
			return err
		}
	})
	r.Use(func(next gogram.HandlerFunc) gogram.HandlerFunc {
		return func(ctx *gogram.Context) error {
			record("middleware 2 start")
			err := next(ctx)
			record("middleware 2 end")
			return err
		}
	})

	r.HandleOnMessage(func(_ *gogram.Context, _ *gogram.Message) error {
		record("handler")
		return nil
	})

	update := &gogram.Update{UpdateID: 1, Message: &gogram.Message{}}
	client, err := gogram.NewClient(testToken)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	gogramCtx := gogram.NewTestContext(t.Context(), client, update)

	r.Process(gogramCtx)

	expectedOrder := []string{
		"middleware 1 start",
		"middleware 2 start",
		"handler",
		"middleware 2 end",
		"middleware 1 end",
	}
	if !slices.Equal(executionOrder, expectedOrder) {
		t.Errorf("expected execution order %v, got %v", expectedOrder, executionOrder)
	}
}

// TestRouter_PanicHandling tests that the panic handler is invoked.
func TestRouter_PanicHandling(t *testing.T) {
	t.Parallel()
	r := gogram.NewRouter()
	var panicHandled atomic.Bool
	panicPayload := "handler panic!"

	r.SetHandlerPanic(func(_ *gogram.Context, v any) {
		if v != panicPayload {
			t.Errorf("expected panic value %v, got %v", panicPayload, v)
		}
		panicHandled.Store(true)
	})

	r.HandleOnMessage(func(_ *gogram.Context, _ *gogram.Message) error {
		panic(panicPayload)
	})

	update := &gogram.Update{UpdateID: 1, Message: &gogram.Message{}}
	client, err := gogram.NewClient(testToken)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	gogramCtx := gogram.NewTestContext(t.Context(), client, update)

	r.Process(gogramCtx)

	if !panicHandled.Load() {
		t.Error("Panic handler was not invoked")
	}
}

// TestRouter_ErrorHandling tests the error handler.
func TestRouter_ErrorHandling(t *testing.T) {
	t.Parallel()
	r := gogram.NewRouter()
	var errorHandled atomic.Bool
	handlerError := errors.New("handler error") //nolint:err113

	r.SetHandlerErr(func(_ *gogram.Context, err error) {
		if !errors.Is(err, handlerError) {
			t.Errorf("expected %v, got %v", handlerError, err)
		}
		errorHandled.Store(true)
	})

	r.HandleOnMessage(func(_ *gogram.Context, _ *gogram.Message) error {
		return handlerError
	})

	update := &gogram.Update{UpdateID: 1, Message: &gogram.Message{}}
	client, err := gogram.NewClient(testToken)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	gogramCtx := gogram.NewTestContext(t.Context(), client, update)

	r.Process(gogramCtx)

	if !errorHandled.Load() {
		t.Error("Error handler was not invoked")
	}
}

// TestFilterComposition tests the And, Or, and Not filter compositions.
func TestFilterComposition(t *testing.T) {
	t.Parallel()

	update := &gogram.Update{UpdateID: 1, Message: &gogram.Message{Text: "action:start"}}
	client, err := gogram.NewClient(testToken)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx := gogram.NewTestContext(t.Context(), client, update)

	filterAction := gogram.FilterPrefix("action:")
	filterStart := gogram.FilterRegexp(`:start$`)
	filterStop := gogram.FilterRegexp(`:stop$`)

	// Test AND
	if !gogram.FilterAnd(filterAction, filterStart)(ctx) {
		t.Error("FilterAnd(action, start) should be true")
	}
	if gogram.FilterAnd(filterAction, filterStop)(ctx) {
		t.Error("FilterAnd(action, stop) should be false")
	}

	// Test OR
	if !gogram.FilterOr(filterStart, filterStop)(ctx) {
		t.Error("FilterOr(start, stop) should be true")
	}
	if gogram.FilterOr(gogram.FilterText("other"), filterStop)(ctx) {
		t.Error("FilterOr(other, stop) should be false")
	}

	// Test NOT
	if !gogram.FilterNot(filterStop)(ctx) {
		t.Error("FilterNot(stop) should be true")
	}
	if gogram.FilterNot(filterStart)(ctx) {
		t.Error("FilterNot(start) should be false")
	}
}

// TestFilterCommand tests the command filter with various formats.
func TestFilterCommand(t *testing.T) {
	t.Parallel()
	client, err := gogram.NewClient(testToken)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	filter := gogram.FilterCommand("/start")

	testCases := []struct {
		name     string
		text     string
		expected bool
	}{
		{"ExactMatch", "/start", true},
		{"WithArgs", "/start payload", true},
		{"WithBotName", "/start@my_bot payload", true},
		{"WrongCommand", "/stop", false},
		{"NotACommand", "start", false},
		{"PrefixOfOther", "/startagain", false},
		{"EmptyText", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			update := &gogram.Update{UpdateID: 1, Message: &gogram.Message{Text: tc.text}}
			ctx := gogram.NewTestContext(t.Context(), client, update)
			got := filter(ctx)
			if got != tc.expected {
				t.Errorf("filter(%q): expected %v, got %v", tc.text, tc.expected, got)
			}
		})
	}
}

// TestFilterByID tests user and chat ID filters.
func TestFilterByID(t *testing.T) {
	t.Parallel()
	client, err := gogram.NewClient(testToken)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	update := &gogram.Update{
		UpdateID: 1,
		Message: &gogram.Message{
			From: &gogram.User{ID: 123},
			Chat: gogram.Chat{ID: 456},
		},
	}
	ctx := gogram.NewTestContext(t.Context(), client, update)

	if !gogram.FilterFromUser(123)(ctx) {
		t.Error("FilterFromUser(123) should be true")
	}
	if gogram.FilterFromUser(999)(ctx) {
		t.Error("FilterFromUser(999) should be false")
	}

	if !gogram.FilterChat(456)(ctx) {
		t.Error("FilterChat(456) should be true")
	}
	if gogram.FilterChat(999)(ctx) {
		t.Error("FilterChat(999) should be false")
	}
}
