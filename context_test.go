package gogram_test

import (
	"context"
	"testing"

	"github.com/darxnet/gogram"
)

type parentKey struct{}

func TestContext_TypedKey(t *testing.T) {
	t.Parallel()

	client, err := gogram.NewClient(testToken)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	parent := context.WithValue(t.Context(), parentKey{}, "from-parent")
	ctx := gogram.NewTestContext(parent, client, &gogram.Update{})

	userKey := gogram.NewKey[int64]("user")

	if _, ok := userKey.Get(ctx); ok {
		t.Fatal("Get on an unset key must report ok=false")
	}

	userKey.Set(ctx, 42)

	v, ok := userKey.Get(ctx)
	if !ok || v != 42 {
		t.Fatalf("Get = %d, %v; want 42, true", v, ok)
	}

	if got := ctx.Value(userKey); got != int64(42) {
		t.Fatalf("Value(key) = %v, want 42", got)
	}

	if got := ctx.Value(parentKey{}); got != "from-parent" {
		t.Fatalf("Value(parent) = %v, want from-parent", got)
	}

	if userKey.String() != "user" {
		t.Fatalf("String() = %q", userKey.String())
	}
}

func TestContext_NewParamsBuilder(t *testing.T) {
	t.Parallel()

	client, err := gogram.NewClient(testToken, gogram.WithDefaultParseMode(gogram.ParseModeHTML))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	update := &gogram.Update{CallbackQuery: &gogram.CallbackQuery{
		ID:      "cb-1",
		Message: &gogram.MaybeInaccessibleMessage{Message: &gogram.Message{MessageID: 5, Chat: gogram.Chat{ID: 7}}},
	}}
	ctx := gogram.NewTestContext(t.Context(), client, update)

	params := ctx.NewSendMessageParams("hi", gogram.WithSendMessageDisableNotification(true))
	if params.ChatID != "7" || params.Text != "hi" || !params.DisableNotification {
		t.Fatalf("params = %+v", params)
	}

	if params.ParseMode != gogram.ParseModeHTML {
		t.Fatalf("ParseMode = %q, want default HTML", params.ParseMode)
	}

	answer := ctx.NewAnswerCallbackQueryParams()
	if answer.CallbackQueryID != "cb-1" {
		t.Fatalf("CallbackQueryID = %q, want cb-1", answer.CallbackQueryID)
	}

	// A builder for an update type the context does not carry yields zero values, not a panic.
	if ctx.NewAnswerInlineQueryParams(nil).InlineQueryID != "" {
		t.Fatal("InlineQueryID must be empty without an inline query")
	}
}
