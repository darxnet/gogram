package gogram_test

import (
	"slices"
	"testing"

	"github.com/darxnet/gogram"
)

func TestRouter_HandleCommand_FastPath(t *testing.T) {
	t.Parallel()

	r := gogram.NewRouter()
	var handled bool

	// Register via fast path
	r.HandleCommand("/start", func(_ *gogram.Context, _ *gogram.Message) error {
		handled = true
		return nil
	})

	client, err := gogram.NewClient(testToken)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	// 1. Exact match
	handled = false
	update := &gogram.Update{Message: &gogram.Message{Text: "/start"}}
	ctx := gogram.NewTestContext(t.Context(), client, update)
	r.Process(ctx)
	if !handled {
		t.Error("Fast path /start failed")
	}

	// 2. With args
	handled = false
	update = &gogram.Update{Message: &gogram.Message{Text: "/start payload"}}
	ctx = gogram.NewTestContext(t.Context(), client, update)
	r.Process(ctx)
	if !handled {
		t.Error("Fast path /start payload failed")
	}

	// 3. With mention
	handled = false
	update = &gogram.Update{Message: &gogram.Message{Text: "/start@bot payload"}}
	ctx = gogram.NewTestContext(t.Context(), client, update)
	r.Process(ctx)
	if !handled {
		t.Error("Fast path /start@bot failed")
	}

	// 4. Unknown command (should not panic or error, just not handle)
	handled = false
	update = &gogram.Update{Message: &gogram.Message{Text: "/unknown"}}
	ctx = gogram.NewTestContext(t.Context(), client, update)
	r.Process(ctx)
	if handled {
		t.Error("Fast path handled unknown command")
	}
}

func TestRouter_HandleCommand_Middleware(t *testing.T) {
	t.Parallel()

	r := gogram.NewRouter()
	var events []string

	r.Use(func(next gogram.HandlerFunc) gogram.HandlerFunc {
		return func(ctx *gogram.Context) error {
			events = append(events, "mw1")
			return next(ctx)
		}
	})

	r.HandleCommand("/cmd", func(_ *gogram.Context, _ *gogram.Message) error {
		events = append(events, "handler")
		return nil
	})

	update := &gogram.Update{Message: &gogram.Message{Text: "/cmd"}}
	client, err := gogram.NewClient(testToken)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx := gogram.NewTestContext(t.Context(), client, update)
	r.Process(ctx)

	if !slices.Equal(events, []string{"mw1", "handler"}) {
		t.Errorf("expected %v, got %v", []string{"mw1", "handler"}, events)
	}
}

func TestRouter_HandleCommand_Spaces(t *testing.T) {
	t.Parallel()

	r := gogram.NewRouter()
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("HandleCommand should panic if command contains spaces")
			}
		}()
		r.HandleCommand("/start game", func(_ *gogram.Context, _ *gogram.Message) error { return nil })
	}()
}

func TestRouter_HandleCommand_MultipleHandlers(t *testing.T) {
	t.Parallel()

	r := gogram.NewRouter()
	var events []string

	// Specific handler (e.g. admin)
	adminGroup := r.Group(func(ctx *gogram.Context) bool {
		return ctx.Text() == "/secret admin_token"
	})
	adminGroup.HandleCommand("/secret", func(_ *gogram.Context, _ *gogram.Message) error {
		events = append(events, "admin")
		return nil
	})

	// General handler
	r.HandleCommand("/secret", func(_ *gogram.Context, _ *gogram.Message) error {
		events = append(events, "general")
		return nil
	})

	client, err := gogram.NewClient(testToken)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	// 1. Admin request
	events = nil
	update := &gogram.Update{Message: &gogram.Message{Text: "/secret admin_token"}}
	ctx := gogram.NewTestContext(t.Context(), client, update)
	r.Process(ctx)
	if !slices.Equal(events, []string{"admin"}) {
		t.Errorf("Admin handler should run: got %v", events)
	}

	// 2. General request (admin filter fails, should fall through to next handler in list)
	events = nil
	update = &gogram.Update{Message: &gogram.Message{Text: "/secret"}}
	ctx = gogram.NewTestContext(t.Context(), client, update)
	r.Process(ctx)
	if !slices.Equal(events, []string{"general"}) {
		t.Errorf("General handler should run when admin filter fails: got %v", events)
	}
}

func TestRouter_HandleCommand_FallbackToSlowPath(t *testing.T) {
	t.Parallel()

	r := gogram.NewRouter()
	var events []string

	// Register a command handler with a filter that will fail
	failGroup := r.Group(func(_ *gogram.Context) bool { return false })
	failGroup.HandleCommand("/try", func(_ *gogram.Context, _ *gogram.Message) error {
		events = append(events, "fast")
		return nil
	})

	// Register a slow path handler for the same text
	r.HandleOnMessage(func(_ *gogram.Context, _ *gogram.Message) error {
		events = append(events, "slow")
		return nil
	}, gogram.FilterText("/try"))

	update := &gogram.Update{Message: &gogram.Message{Text: "/try"}}
	client, err := gogram.NewClient(testToken)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx := gogram.NewTestContext(t.Context(), client, update)

	r.Process(ctx)

	// Expectation: Fast path matches "/try" in map -> loop handlers -> filter returns false -> loop finishes.
	// Code should fall through to Slow Path -> FilterText matches -> handler runs.
	if !slices.Equal(events, []string{"slow"}) {
		t.Errorf("Should fall back to slow path if fast path filters fail: got %v", events)
	}
}
