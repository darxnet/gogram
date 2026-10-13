package gogram_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/darxnet/gogram"
)

// A minimal echo bot: commands and messages are dispatched by the router,
// chat_id and friends come from the update.
//
//nolint:testableexamples // talks to Telegram
func ExampleNewClient() {
	router := gogram.NewRouter()

	router.HandleCommand("/start", func(ctx *gogram.Context, m *gogram.Message) error {
		return ctx.Reply("Hello, " + m.From.FirstName)
	}, "Start the bot")

	router.HandleOnMessage(func(ctx *gogram.Context, m *gogram.Message) error {
		return ctx.SendMessage(m.Text)
	})

	client, err := gogram.NewClient(os.Getenv("BOT_TOKEN"), gogram.WithRouter(router))
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := client.SyncCommands(ctx); err != nil { // publishes "/start — Start the bot"
		log.Println(err)
		return
	}

	if err := client.Start(ctx, nil); err != nil {
		log.Println(err)
	}
}

// Webhook delivery: the webhook is registered with Telegram and served
// until ctx is cancelled; handlers run on a worker pool.
//
//nolint:testableexamples // talks to Telegram
func ExampleClient_StartWebhook() {
	client, err := gogram.NewClient(os.Getenv("BOT_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err = client.StartWebhook(ctx, ":8443", &gogram.SetWebhookParams{
		URL:         "https://bot.example.com/telegram",
		SecretToken: os.Getenv("WEBHOOK_SECRET"),
	})
	if err != nil {
		log.Println(err)
	}
}

// Typed callback data and an inline keyboard: the struct is encoded into
// callback_data and decoded back for the handler.
func ExampleNewCallback() {
	type Page struct {
		List string
		N    int
	}

	pageCB := gogram.NewCallback[Page]("pg")

	router := gogram.NewRouter()
	router.Use(gogram.AutoAnswer()) // answer callback queries the handlers do not

	render := func(list string, n int) *gogram.InlineKeyboardBuilder {
		return gogram.NewInlineKeyboardBuilder().
			Add(pageCB.Button("‹", Page{list, n - 1}), pageCB.Button("›", Page{list, n + 1})).Row().
			URL("Docs", "https://example.com")
	}

	router.HandleCommand("/list", func(ctx *gogram.Context, _ *gogram.Message) error {
		return ctx.SendMessage("Page 1", gogram.WithSendMessageReplyMarkup(render("users", 1).ReplyMarkup()))
	})

	pageCB.Handle(router.RouterGroup, func(ctx *gogram.Context, _ *gogram.CallbackQuery, p Page) error {
		return ctx.EditMessageText("Page "+strconv.Itoa(p.N),
			gogram.WithEditMessageTextReplyMarkup(render(p.List, p.N).Markup()))
	})

	fmt.Println(pageCB.Button("›", Page{"users", 2}).CallbackData)
	// Output: pg users 2
}

// A registration dialogue: states live in the client's state store and
// the conversation groups the handlers per state.
func ExampleConversation() {
	router := gogram.NewRouter()
	nameKey := gogram.NewKey[string]("name")

	router.HandleCommand("/register", func(ctx *gogram.Context, _ *gogram.Message) error {
		return errors.Join(ctx.SetState("name"), ctx.SendMessage("Your name?"))
	}, "Register")

	router.HandleCommand("/cancel", func(ctx *gogram.Context, _ *gogram.Message) error {
		return errors.Join(ctx.FinishState(), ctx.SendMessage("Cancelled"))
	}, "Cancel")

	conv := router.Conversation()

	conv.On("name", func(ctx *gogram.Context, m *gogram.Message) error {
		nameKey.Set(ctx, m.Text)
		return errors.Join(ctx.SetState("age"), ctx.SendMessage("Your age?"))
	})

	conv.On("age", func(ctx *gogram.Context, m *gogram.Message) error {
		name, _ := nameKey.Get(ctx)
		return errors.Join(ctx.FinishState(), ctx.SendMessage("Registered "+name+", "+m.Text))
	}, gogram.FilterRegexp(`^\d+$`))

	conv.Fallback(func(ctx *gogram.Context, _ *gogram.Message) error {
		return ctx.SendMessage("Please answer the question, or /cancel")
	})

	_, err := gogram.NewClient("123:token",
		gogram.WithRouter(router),
		gogram.WithStateStore(gogram.NewMemoryStateStore(gogram.WithTTL(time.Hour))),
	)
	if err != nil {
		log.Fatal(err)
	}

	for _, cmd := range router.BotCommands() {
		fmt.Println(cmd.Command, "—", cmd.Description)
	}
	// Output:
	// register — Register
	// cancel — Cancel
}

// Flood control and logging are transformers on the API call chain.
func ExampleWithTransformers() {
	client, err := gogram.NewClient("123:token",
		gogram.WithTransformers(
			gogram.Throttle(gogram.ThrottleConfig{}), // 1 msg/s per private chat, 20/min per group
			gogram.Logging(slog.Default(), slog.LevelDebug),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(client.ID())
	// Output: 123
}

// Uploads stream from disk and are reopened when a request is retried.
//
//nolint:testableexamples // talks to Telegram
func ExampleFromDisk() {
	client, err := gogram.NewClient(os.Getenv("BOT_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}

	msg, err := client.SendDocument(context.Background(), &gogram.SendDocumentParams{
		ChatID:   gogram.ChatID{ID: 123},
		Document: gogram.FromDisk("/data/report.pdf"),
		Caption:  "Monthly report",
	})
	if err != nil {
		log.Fatal(err)
	}

	_, _ = client.EditText(context.Background(), msg, "Monthly report (sent)")
}

// Methods without a generated wrapper go through Call.
//
//nolint:testableexamples // talks to Telegram
func ExampleCall() {
	client, err := gogram.NewClient(os.Getenv("BOT_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}

	me, err := gogram.Call[*gogram.User](context.Background(), client, "getMe", nil)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(me.Username)
}
