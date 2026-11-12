package gogram_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/darxnet/gogram"
	"github.com/darxnet/gogram/tgtest"
)

func newRecorderClient(t *testing.T) (*tgtest.Recorder, *gogram.Client) {
	t.Helper()

	rec := tgtest.NewRecorder()

	client, err := gogram.NewClient(testToken, gogram.WithCaller(rec))
	if err != nil {
		t.Fatal(err)
	}

	return rec, client
}

func TestWidgetQuery_ContextAndAutoAnswer(t *testing.T) {
	t.Parallel()

	rec, client := newRecorderClient(t)

	router := gogram.NewRouter()
	router.Use(gogram.AutoAnswer())
	router.HandleOnWidgetQuery(func(ctx *gogram.Context, q *gogram.WidgetQuery) error {
		if ctx.User().ID != tgtest.User.ID || ctx.Chat().ID != 7 || q.Data != "ping" {
			t.Errorf("accessors: user %+v chat %+v data %q", ctx.User(), ctx.Chat(), q.Data)
		}

		return ctx.EditMessageWidgetData(gogram.WithEditMessageWidgetDataChatData("pong"))
	}, gogram.FilterWidgetQueryData())

	update := tgtest.WidgetQuery(7, "ping")
	router.Process(tgtest.NewContext(t, client, update))

	rec.AssertSequence(t, "editMessageWidgetData", "answerWidgetQuery")
	rec.AssertCalled(t, "editMessageWidgetData", tgtest.Match(func(p *gogram.EditMessageWidgetDataParams) bool {
		return p.ChatID.ID == 7 && p.MessageID == update.WidgetQuery.MessageID && p.ChatData == "pong"
	}))
	rec.AssertCalled(t, "answerWidgetQuery", tgtest.Match(func(p *gogram.AnswerWidgetQueryParams) bool {
		return p.WidgetQueryID == update.WidgetQuery.ID
	}))

	// A handler that answers itself is not answered twice.
	rec.Reset()

	answering := gogram.NewRouter()
	answering.Use(gogram.AutoAnswer())
	answering.HandleOnWidgetQuery(func(ctx *gogram.Context, _ *gogram.WidgetQuery) error {
		return ctx.AnswerWidgetQuery(gogram.WithAnswerWidgetQueryData("ok"))
	})
	answering.Process(tgtest.NewContext(t, client, tgtest.WidgetQuery(7, "x")))

	if calls := rec.Calls(); len(calls) != 1 || calls[0].Method != "answerWidgetQuery" {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestWidgetQuery_Inline(t *testing.T) {
	t.Parallel()

	rec, client := newRecorderClient(t)

	update := &gogram.Update{WidgetQuery: &gogram.WidgetQuery{ID: "q", InlineMessageID: "im1"}}
	ctx := tgtest.NewContext(t, client, update)

	if err := ctx.EditMessageWidgetData(); err != nil {
		t.Fatal(err)
	}

	rec.AssertCalled(t, "editMessageWidgetData", tgtest.Match(func(p *gogram.EditMessageWidgetDataParams) bool {
		return p.InlineMessageID == "im1" && p.MessageID == 0 && p.ChatID.IsZero()
	}))
}

func TestSharedWidgetQuery(t *testing.T) {
	t.Parallel()

	rec, client := newRecorderClient(t)

	router := gogram.NewRouter()
	router.Use(gogram.AutoAnswer())
	router.HandleOnSharedWidgetQuery(func(ctx *gogram.Context, q *gogram.SharedWidgetQuery) error {
		return ctx.EditSharedWidgetData(q.WidgetID, q.Data)
	})
	router.Process(tgtest.NewContext(t, client, tgtest.SharedWidgetQuery("w1", "d")))

	rec.AssertCalled(t, "editSharedWidgetData", tgtest.Match(func(p *gogram.EditSharedWidgetDataParams) bool {
		return p.WidgetID == "w1"
	}))
	rec.AssertNotCalled(t, "answerWidgetQuery") // shared widget queries have no answer method
}

func TestFilterWidgetQueryType(t *testing.T) {
	t.Parallel()

	_, client := newRecorderClient(t)

	ctxOf := func(u *gogram.Update) *gogram.Context { return tgtest.NewContext(t, client, u) }

	data := ctxOf(tgtest.WidgetQuery(1, "d"))
	sign := ctxOf(&gogram.Update{SharedWidgetQuery: &gogram.SharedWidgetQuery{
		Type: gogram.WidgetQueryType{WidgetQueryTypeSignData: &gogram.WidgetQueryTypeSignData{}},
	}})
	sent := ctxOf(&gogram.Update{WidgetQuery: &gogram.WidgetQuery{
		Type: gogram.WidgetQueryType{WidgetQueryTypeSentTransaction: &gogram.WidgetQueryTypeSentTransaction{}},
	}})
	text := ctxOf(tgtest.Text(1, "hi"))

	cases := []struct {
		name   string
		filter gogram.Filter
		want   map[*gogram.Context]bool
	}{
		{"data", gogram.FilterWidgetQueryData(), map[*gogram.Context]bool{data: true, sign: false, sent: false, text: false}},
		{
			"sign", gogram.FilterWidgetQuerySignData(),
			map[*gogram.Context]bool{data: false, sign: true, sent: false, text: false},
		},
		{
			"sent", gogram.FilterWidgetQuerySentTransaction(),
			map[*gogram.Context]bool{data: false, sign: false, sent: true, text: false},
		},
	}

	for _, tc := range cases {
		for ctx, want := range tc.want {
			if got := tc.filter(ctx); got != want {
				t.Errorf("%s: got %v, want %v", tc.name, got, want)
			}
		}
	}
}

func TestWidgetQueryType_UnknownSubtype(t *testing.T) {
	t.Parallel()

	var q gogram.WidgetQuery
	if err := json.Unmarshal([]byte(`{"id":"1","type":{"type":"future"},"data":"x"}`), &q); err != nil {
		t.Fatalf("unknown widget query type must not fail the update: %v", err)
	}

	if q.Type.WidgetQueryTypeData != nil || q.Data != "x" {
		t.Fatalf("q = %+v", q)
	}
}

func TestReplyEphemeral(t *testing.T) {
	t.Parallel()

	rec, client := newRecorderClient(t)

	// Callback query: an ephemeral reply to the query, which AutoAnswer still answers.
	router := gogram.NewRouter()
	router.Use(gogram.AutoAnswer())
	router.HandleOnCallbackQuery(func(ctx *gogram.Context, _ *gogram.CallbackQuery) error {
		return ctx.ReplyEphemeral("only you")
	})

	update := tgtest.Callback(7, "x")
	router.Process(tgtest.NewContext(t, client, update))

	rec.AssertSequence(t, "sendMessage", "answerCallbackQuery")
	rec.AssertCalled(t, "sendMessage", tgtest.Match(func(p *gogram.SendMessageParams) bool {
		e := p.EphemeralMessageParameters

		return e != nil && e.CallbackQueryID == update.CallbackQuery.ID && e.ReceiverUserID == 0 && p.ReplyParameters == nil
	}))

	// Incoming ephemeral command: threaded to it, addressed to its sender.
	rec.Reset()

	command := tgtest.Text(-5, "/secret")
	command.Message.EphemeralMessageID = 9

	if err := tgtest.NewContext(t, client, command).ReplyEphemeral("psst"); err != nil {
		t.Fatal(err)
	}

	rec.AssertCalled(t, "sendMessage", tgtest.Match(func(p *gogram.SendMessageParams) bool {
		e := p.EphemeralMessageParameters

		return e != nil && e.ReceiverUserID == tgtest.User.ID && e.CallbackQueryID == "" &&
			p.ReplyParameters != nil && p.ReplyParameters.EphemeralMessageID == 9
	}))

	// Explicit parameters win.
	rec.Reset()

	explicit := &gogram.EphemeralMessageParameters{ReceiverUserID: 42}
	if err := tgtest.NewContext(t, client, tgtest.Text(1, "x")).
		ReplyEphemeral("x", gogram.WithSendMessageEphemeralMessageParameters(explicit)); err != nil {
		t.Fatal(err)
	}

	rec.AssertCalled(t, "sendMessage", tgtest.Match(func(p *gogram.SendMessageParams) bool {
		return p.EphemeralMessageParameters == explicit && p.ReplyParameters == nil
	}))

	if e := tgtest.NewContext(t, client, &gogram.Update{}).Ephemeral(); e != nil {
		t.Fatalf("Ephemeral without a user = %+v, want nil", e)
	}
}

func TestOptionalChatID_Omitted(t *testing.T) {
	t.Parallel()

	// An ephemeral reply to a callback from an inline message has no chat.
	b, err := json.Marshal(gogram.SendMessageParams{
		Text: "x", EphemeralMessageParameters: &gogram.EphemeralMessageParameters{CallbackQueryID: "q"},
	})
	if err != nil || string(b) != `{"ephemeral_message_parameters":{"callback_query_id":"q"},"text":"x"}` {
		t.Fatalf("sendMessage: %s, %v", b, err)
	}
}

func TestTopicLink(t *testing.T) {
	t.Parallel()

	client, err := gogram.NewClient(testToken,
		gogram.WithCaller(cannedCaller(`{"id":1,"is_bot":true,"first_name":"b","username":"my_bot"}`)))
	if err != nil {
		t.Fatal(err)
	}

	got, err := client.TopicLink(t.Context(), 15)
	if err != nil || got != "https://t.me/my_bot/15" {
		t.Fatalf("TopicLink = %q, %v", got, err)
	}

	if _, err = client.TopicLink(t.Context(), 0); !errors.Is(err, gogram.ErrInvalidThreadID) {
		t.Fatalf("TopicLink(0) err = %v", err)
	}
}

func TestStoryAreaPosition_Alias(t *testing.T) {
	t.Parallel()

	// The old name still compiles and is the same type.
	x := func(a gogram.AreaPosition) float64 { return a.XPercentage }
	if x(gogram.StoryAreaPosition{XPercentage: 1}) != 1 {
		t.Fatal("alias mismatch")
	}
}
