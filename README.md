# gogram

Telegram Bot API client for Go, generated from the official documentation,
built for high-load bots: one JSON pass per call, pooled buffers, an
allocation-free router, per-chat flood control and `context.Context`
everywhere.

```
go get github.com/darxnet/gogram
```

Requires Go 1.26.

## Quick start

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/darxnet/gogram"
)

func main() {
	router := gogram.NewRouter()

	router.HandleCommand("/start", func(ctx *gogram.Context, m *gogram.Message) error {
		return ctx.Reply("Hello, " + m.From.FirstName)
	})

	router.HandleOnMessage(func(ctx *gogram.Context, m *gogram.Message) error {
		return ctx.SendMessage(m.Text) // chat_id, thread and business connection come from the update
	})

	client, err := gogram.NewClient(os.Getenv("BOT_TOKEN"), gogram.WithRouter(router))
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := client.Start(ctx, nil); err != nil {
		log.Fatal(err)
	}
}
```

`Start` long-polls with 100 workers; updates of one chat always land on the
same worker, so handlers for a chat run in order. `StartWebhook` serves the
same router over HTTPS.

## Context

Every handler receives a `*gogram.Context`: it is a `context.Context` for the
update and carries typed accessors (`User()`, `Chat()`, `Message()`, `Text()`)
for any update type. Each Bot API method has a Context form that fills
`chat_id`, `message_id`, `callback_query_id`, thread and business ids from the
update and applies the client's default parse mode:

```go
ctx.SendChatAction(gogram.ChatActionTyping)
ctx.EditMessageText("done", gogram.WithEditMessageTextReplyMarkup(kb.Markup()))
ctx.AnswerCallbackQuery(gogram.WithAnswerCallbackQueryText("saved"))
ctx.DeleteMessage()
```

Context methods return only `error`, which keeps handlers one-liners. When
the result is needed, build the same params and call the client:

```go
msg, err := ctx.Client().SendMessage(ctx.Detached(), ctx.NewSendMessageParams("working…"))
```

`Detached()` is the update's context without cancellation: an API call
started by a handler finishes even while the bot shuts down.

Options are plain closures and cost no allocation:

```go
ctx.SendMessage(text,
	gogram.WithSendMessageParseMode(gogram.ParseModeHTML),
	gogram.WithSendMessageReplyMarkup(kb.ReplyMarkup()),
)
```

## Router

Commands and callback buttons are dispatched with a map lookup, other
routes with filters in registration order; groups share filters and
middleware:

```go
admin := router.Group(gogram.FilterChat(adminChatID))
admin.Use(audit)
admin.HandleCommand("/stats", stats)

router.HandleOnMessage(onDocument, gogram.FilterMessageDocument())
router.SetHandlerErr(func(ctx *gogram.Context, err error) { log.Println(err) })
```

## Inline keyboards and typed callback data

```go
type Page struct {
	List string
	N    int
}

var pageCB = gogram.NewCallback[Page]("pg") // callback_data: "pg users 2"

kb := gogram.NewInlineKeyboard().
	Add(pageCB.Button("‹", Page{"users", 1}), pageCB.Button("›", Page{"users", 3})).Row().
	URL("Docs", "https://example.com")

pageCB.Handle(router.RouterGroup, func(ctx *gogram.Context, cq *gogram.CallbackQuery, p Page) error {
	return ctx.EditMessageText(render(p.List, p.N), gogram.WithEditMessageTextReplyMarkup(kb.Markup()))
})
```

Fields are encoded positionally and checked against the 64-byte limit when
the button is built; data from an older build fails with
`ErrInvalidCallbackData` and goes to the error handler.

## State machine

```go
router.HandleCommand("/register", func(ctx *gogram.Context, _ *gogram.Message) error {
	fsm, _ := ctx.FSM()
	return errors.Join(fsm.Set(ctx, "name"), ctx.SendMessage("Your name?"))
})

router.HandleOnMessage(func(ctx *gogram.Context, m *gogram.Message) error {
	fsm, _ := ctx.FSM()
	return errors.Join(fsm.Finish(ctx), ctx.SendMessage("Hi, "+m.Text))
}, gogram.FilterState("name"))

client, err := gogram.NewClient(token,
	gogram.WithRouter(router),
	gogram.WithStorer(gogram.NewInMemoryStorer(gogram.WithTTL(time.Hour))), // or sqlstore.New(ctx, sqlstore.PostgreSQL, db)
)
```

Without `WithStorer` an in-memory storer is used. The state is loaded once
per update however many `FilterState` routes there are.

## Flood control, logging, custom transports

API calls go through a chain of transformers ending in the HTTP transport:

```go
client, err := gogram.NewClient(token,
	gogram.WithRPS(30),                       // global limit, default
	gogram.WithTransformers(
		gogram.Throttle(gogram.ThrottleConfig{}), // 1 msg/s per private chat, 20/min per group
		gogram.Logging(slog.Default(), slog.LevelDebug),
	),
)
```

Transient errors (429 with `retry_after`, 5xx, empty or `EOF` bodies) are
retried up to five times when the request body can be rewound. A
`Transformer` is a `func(next Caller) Caller`; tracing, metrics or a mock
fit the same shape. `WithCaller` replaces the transport altogether.

## Testing handlers

```go
rec := tgtest.NewRecorder()
client, _ := gogram.NewClient("123:abc", gogram.WithCaller(rec))

router.Process(tgtest.NewContext(t, client, tgtest.Text(7, "/start")))

rec.AssertSequence(t, "sendChatAction", "sendMessage")
rec.AssertCalled(t, "sendMessage", tgtest.Match(func(p *gogram.SendMessageParams) bool {
	return p.Text == "Hello"
}))
```

## Files

```go
file, _ := client.GetFile(ctx, &gogram.GetFileParams{FileID: id})
err := client.DownloadFile(ctx, file, "/tmp/photo.jpg")

_, err = client.SendDocument(ctx, &gogram.SendDocumentParams{
	ChatID:   "7",
	Document: gogram.InputFile{File: f, FileName: "report.pdf"}, // streamed, not buffered
})
```

## Regenerating the API

`make generate` fetches https://core.telegram.org/bots/api and rewrites
`types.gen.go`, `methods.gen.go`, `context.gen.go` and `router.gen.go`.
Unknown subtypes in a newer Bot API decode to empty values instead of
failing the update.

## License

Apache 2.0, see LICENSE.
