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
