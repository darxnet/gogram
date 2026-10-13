package gogram_test

import (
	"context"
	"fmt"

	"github.com/darxnet/gogram"
)

// A tracing transformer opens a span around every API call; the same
// shape works with OpenTelemetry (tracer.Start / span.End) or any other
// tracer. A router middleware does the same per update.
func ExampleTransformer() {
	// Stand-in for an OTel tracer: otel.Tracer("gogram").Start(ctx, name).
	startSpan := func(ctx context.Context, name string) (context.Context, func()) {
		fmt.Println("span start:", name)
		return ctx, func() { fmt.Println("span end:", name) }
	}

	tracing := func(next gogram.Caller) gogram.Caller {
		return gogram.CallerFunc(func(ctx context.Context, req gogram.Request, result any) error {
			ctx, end := startSpan(ctx, "telegram."+req.Method)
			defer end()

			return next.Call(ctx, req, result)
		})
	}

	// Replaces the HTTP transport in this example; drop WithCaller in a real bot.
	fake := gogram.CallerFunc(func(context.Context, gogram.Request, any) error { return nil })

	client, err := gogram.NewClient("123:token", gogram.WithCaller(fake), gogram.WithTransformers(tracing))
	if err != nil {
		fmt.Println(err)
		return
	}

	router := gogram.NewRouter()

	// Per-update span: wraps every handler of the router.
	router.Use(func(next gogram.HandlerFunc) gogram.HandlerFunc {
		return func(ctx *gogram.Context) error {
			_, end := startSpan(ctx, fmt.Sprintf("update %d", ctx.Update().UpdateID))
			defer end()

			return next(ctx)
		}
	})

	router.HandleOnMessage(func(ctx *gogram.Context, _ *gogram.Message) error {
		return ctx.SendChatAction(gogram.ChatActionTyping)
	})

	update := &gogram.Update{UpdateID: 42, Message: &gogram.Message{Chat: gogram.Chat{ID: 7}, Text: "hi"}}
	router.Process(gogram.NewTestContext(context.Background(), client, update))

	// Output:
	// span start: update 42
	// span start: telegram.sendChatAction
	// span end: telegram.sendChatAction
	// span end: update 42
}
