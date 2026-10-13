package gogram_test

import (
	"testing"

	"github.com/darxnet/gogram"
	"github.com/darxnet/gogram/tgtest"
)

func TestClient_EditHelpers(t *testing.T) {
	t.Parallel()

	rec := tgtest.NewRecorder()

	client, err := gogram.NewClient(testToken, gogram.WithCaller(rec), gogram.WithDefaultParseMode(gogram.ParseModeHTML))
	if err != nil {
		t.Fatal(err)
	}

	msg := &gogram.Message{MessageID: 9, Chat: gogram.Chat{ID: 7}}
	ctx := t.Context()

	if _, err = client.EditText(ctx, msg, "<b>x</b>"); err != nil {
		t.Fatal(err)
	}

	if _, err = client.EditRichText(ctx, msg, &gogram.InputRichMessage{}); err != nil {
		t.Fatal(err)
	}

	if _, err = client.EditReplyMarkup(ctx, msg, nil); err != nil {
		t.Fatal(err)
	}

	if err = client.Delete(ctx, msg); err != nil {
		t.Fatal(err)
	}

	rec.AssertSequence(t, "editMessageText", "editMessageText", "editMessageReplyMarkup", "deleteMessage")
	rec.AssertCalled(t, "editMessageText", tgtest.Match(func(p *gogram.EditMessageTextParams) bool {
		return p.ChatID.ID == 7 && p.MessageID == 9 && p.Text == "<b>x</b>" && p.ParseMode == gogram.ParseModeHTML
	}))
	rec.AssertCalled(t, "editMessageText", tgtest.Match(func(p *gogram.EditMessageTextParams) bool {
		return p.RichMessage != nil && p.Text == "" && p.ParseMode == ""
	}))
}

func TestContext_EditMessageTextVariants(t *testing.T) {
	t.Parallel()

	rec := tgtest.NewRecorder()

	client, err := gogram.NewClient(testToken, gogram.WithCaller(rec))
	if err != nil {
		t.Fatal(err)
	}

	ctx := tgtest.NewContext(t, client, tgtest.Callback(7, "x"))

	if err = ctx.EditMessageText("plain"); err != nil {
		t.Fatal(err)
	}

	if err = ctx.EditMessageTextWithRichMessage(&gogram.InputRichMessage{}); err != nil {
		t.Fatal(err)
	}

	rec.AssertCalled(t, "editMessageText", tgtest.Match(func(p *gogram.EditMessageTextParams) bool {
		return p.Text == "plain" && p.MessageID != 0
	}))
	rec.AssertCalled(t, "editMessageText", tgtest.Match(func(p *gogram.EditMessageTextParams) bool {
		return p.RichMessage != nil && p.Text == ""
	}))
}
