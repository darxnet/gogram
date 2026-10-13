package tgtest_test

import (
	"errors"
	"testing"

	"github.com/darxnet/gogram"
	"github.com/darxnet/gogram/tgtest"
)

var errBoom = errors.New("boom")

func TestRecorder(t *testing.T) {
	t.Parallel()

	rec := tgtest.NewRecorder()
	rec.WillReturn("sendMessage", &gogram.Message{MessageID: 42})

	client, err := gogram.NewClient("123:abc", gogram.WithCaller(rec))
	if err != nil {
		t.Fatal(err)
	}

	router := gogram.NewRouter()
	router.HandleCommand("/start", func(ctx *gogram.Context, _ *gogram.Message) error {
		if actionErr := ctx.SendChatAction(gogram.ChatActionTyping); actionErr != nil {
			return actionErr
		}

		return ctx.Reply("Hello")
	})
	router.HandleOnCallbackQuery(func(ctx *gogram.Context, cq *gogram.CallbackQuery) error {
		return ctx.AnswerCallbackQuery(gogram.WithAnswerCallbackQueryText(cq.Data))
	})

	router.Process(tgtest.NewContext(t, client, tgtest.Text(7, "/start")))

	rec.AssertSequence(t, "sendChatAction", "sendMessage")
	rec.AssertCalled(t, "sendMessage", tgtest.Match(func(p *gogram.SendMessageParams) bool {
		return p.ChatID == "7" && p.Text == "Hello" && p.ReplyParameters != nil
	}))
	rec.AssertNotCalled(t, "answerCallbackQuery")

	router.Process(tgtest.NewContext(t, client, tgtest.Callback(7, "pg 2")))

	rec.AssertCalled(t, "answerCallbackQuery", func(params any) bool {
		return params.(*gogram.AnswerCallbackQueryParams).Text == "pg 2"
	})

	// Queued result was consumed by the first sendMessage; a direct call sees it.
	rec.Reset()
	rec.WillReturn("getMe", &gogram.User{ID: 123, IsBot: true})

	me, err := client.GetMe(t.Context(), nil)
	if err != nil || me == nil || me.ID != 123 {
		t.Fatalf("GetMe = %+v, %v", me, err)
	}

	rec.WillFail("getMe", errBoom)

	if _, err = client.GetMe(t.Context(), nil); !errors.Is(err, errBoom) {
		t.Fatalf("GetMe err = %v, want errBoom", err)
	}

	if calls := rec.Calls(); len(calls) != 2 || calls[0].Method != "getMe" {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestMessage_CommandEntity(t *testing.T) {
	t.Parallel()

	u := tgtest.Text(1, "/start payload")
	if len(u.Message.Entities) != 1 || u.Message.Entities[0].Length != 6 {
		t.Fatalf("entities = %+v", u.Message.Entities)
	}

	if tgtest.Text(1, "hello").Message.Entities != nil {
		t.Fatal("plain text must have no entities")
	}
}

func TestAutoAnswerAndSyncCommands(t *testing.T) {
	t.Parallel()

	rec := tgtest.NewRecorder()

	router := gogram.NewRouter()
	router.Use(gogram.AutoAnswer())

	router.HandleCommand("/start", func(*gogram.Context, *gogram.Message) error { return nil }, "Start")
	router.HandleCallback("silent", func(*gogram.Context, *gogram.CallbackQuery) error { return nil })
	router.HandleCallback("loud", func(ctx *gogram.Context, _ *gogram.CallbackQuery) error {
		return ctx.AnswerCallbackQuery(gogram.WithAnswerCallbackQueryText("done"))
	})

	client, err := gogram.NewClient("123:abc", gogram.WithCaller(rec), gogram.WithRouter(router))
	if err != nil {
		t.Fatal(err)
	}

	router.Process(tgtest.NewContext(t, client, tgtest.Callback(7, "silent")))
	router.Process(tgtest.NewContext(t, client, tgtest.Callback(7, "loud")))
	router.Process(tgtest.NewContext(t, client, tgtest.Text(7, "/start")))

	// One answer per callback: the middleware answers "silent", the handler answers "loud".
	rec.AssertSequence(t, "answerCallbackQuery", "answerCallbackQuery")
	rec.AssertCalled(t, "answerCallbackQuery", tgtest.Match(func(p *gogram.AnswerCallbackQueryParams) bool {
		return p.Text == "done"
	}))

	if err = client.SyncCommands(t.Context()); err != nil {
		t.Fatal(err)
	}

	rec.AssertCalled(t, "setMyCommands", tgtest.Match(func(p *gogram.SetMyCommandsParams) bool {
		return len(p.Commands) == 1 && p.Commands[0].Command == "start"
	}))
}
