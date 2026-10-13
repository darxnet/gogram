package gogram_test

import (
	"testing"

	"github.com/darxnet/gogram"
)

func newBenchRouter() *gogram.Router {
	r := gogram.NewRouter()
	r.Use(func(next gogram.HandlerFunc) gogram.HandlerFunc { return next })
	r.HandleCommand("/start", func(*gogram.Context, *gogram.Message) error { return nil })
	r.HandleCallback("page", func(*gogram.Context, *gogram.CallbackQuery) error { return nil })
	r.HandleOnMessage(func(*gogram.Context, *gogram.Message) error { return nil },
		gogram.FilterPrefix("hello"), gogram.FilterChat(7))
	r.SetHandlerDefault(func(*gogram.Context) error { return nil })

	return r
}

func benchProcess(b *testing.B, update *gogram.Update) {
	b.Helper()

	r := newBenchRouter()

	client, err := gogram.NewClient(testToken)
	if err != nil {
		b.Fatal(err)
	}

	ctx := gogram.NewTestContext(b.Context(), client, update)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		r.Process(ctx)
	}
}

func BenchmarkRouter_Command(b *testing.B) {
	benchProcess(b, &gogram.Update{Message: &gogram.Message{Text: "/start payload", Chat: gogram.Chat{ID: 7}}})
}

func BenchmarkRouter_CallbackPayload(b *testing.B) {
	benchProcess(b, &gogram.Update{CallbackQuery: &gogram.CallbackQuery{Data: "page 3"}})
}

func BenchmarkRouter_FilterSlowPath(b *testing.B) {
	benchProcess(b, &gogram.Update{Message: &gogram.Message{Text: "hello there", Chat: gogram.Chat{ID: 7}}})
}

func BenchmarkRouter_Default(b *testing.B) {
	benchProcess(b, &gogram.Update{Message: &gogram.Message{Text: "nope", Chat: gogram.Chat{ID: 7}}})
}

func BenchmarkContext_SendMessage(b *testing.B) {
	client := newStubClient(b,
		`{"ok":true,"result":{"message_id":1,"date":1700000000,"chat":{"id":7,"type":"private"},"text":"hi"}}`)

	update := &gogram.Update{Message: &gogram.Message{MessageID: 5, Text: "hi", Chat: gogram.Chat{ID: 7}}}
	ctx := gogram.NewTestContext(b.Context(), client, update)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if err := ctx.SendMessage("hi"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkContext_Reply(b *testing.B) {
	client := newStubClient(b,
		`{"ok":true,"result":{"message_id":1,"date":1700000000,"chat":{"id":7,"type":"private"},"text":"hi"}}`)

	update := &gogram.Update{Message: &gogram.Message{MessageID: 5, Text: "hi", Chat: gogram.Chat{ID: 7}}}
	ctx := gogram.NewTestContext(b.Context(), client, update)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if err := ctx.Reply("hi"); err != nil {
			b.Fatal(err)
		}
	}
}
