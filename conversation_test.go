package gogram_test

import (
	"errors"
	"testing"

	"github.com/darxnet/gogram"
	"github.com/darxnet/gogram/tgtest"
)

func TestConversation(t *testing.T) {
	t.Parallel()

	rec := tgtest.NewRecorder()
	router := gogram.NewRouter()
	conv := router.Conversation()

	var errs []error

	router.SetHandlerErr(func(_ *gogram.Context, err error) { errs = append(errs, err) })

	router.HandleCommand("/register", func(ctx *gogram.Context, _ *gogram.Message) error {
		return errors.Join(ctx.SetState("name"), ctx.SendMessage("Your name?"))
	})
	router.HandleCommand("/cancel", func(ctx *gogram.Context, _ *gogram.Message) error {
		return errors.Join(ctx.FinishState(), ctx.SendMessage("Cancelled"))
	})

	conv.On("name", func(ctx *gogram.Context, m *gogram.Message) error {
		return errors.Join(ctx.SetState("age"), ctx.SendMessage("Hi "+m.Text+", your age?"))
	})
	conv.On("age", func(ctx *gogram.Context, m *gogram.Message) error {
		return errors.Join(ctx.FinishState(), ctx.SendMessage("Age "+m.Text))
	}, gogram.FilterRegexp(`^\d+$`))
	conv.Fallback(func(ctx *gogram.Context, _ *gogram.Message) error {
		return ctx.SendMessage("Please answer")
	})

	router.SetHandlerDefault(func(ctx *gogram.Context) error { return ctx.SendMessage("Default") })

	client, err := gogram.NewClient(testToken, gogram.WithCaller(rec), gogram.WithRouter(router))
	if err != nil {
		t.Fatal(err)
	}

	say := func(text string) {
		router.Process(tgtest.NewContext(t, client, tgtest.Text(7, text)))
	}

	say("hello")     // no state: default handler
	say("/register") // -> name
	say("Ann")       // -> age
	say("old")       // age filter rejects: fallback
	say("/cancel")   // command wins over state
	say("30")        // no state again: default
	say("/register")
	say("Bob")
	say("41") // finish

	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}

	var texts []string

	for _, call := range rec.Calls() {
		if p, ok := call.Params.(*gogram.SendMessageParams); ok {
			texts = append(texts, p.Text)
		}
	}

	want := []string{
		"Default", "Your name?", "Hi Ann, your age?", "Please answer", "Cancelled", "Default",
		"Your name?", "Hi Bob, your age?", "Age 41",
	}

	if len(texts) != len(want) {
		t.Fatalf("texts = %q, want %q", texts, want)
	}

	for i := range want {
		if texts[i] != want[i] {
			t.Fatalf("texts[%d] = %q, want %q (all: %q)", i, texts[i], want[i], texts)
		}
	}

	// State helpers without a user report ErrNoUser.
	ctx := tgtest.NewContext(t, client, &gogram.Update{Message: &gogram.Message{Text: "x"}})
	if err = ctx.SetState("s"); !errors.Is(err, gogram.ErrNoUser) {
		t.Fatalf("SetState without user = %v", err)
	}
}
