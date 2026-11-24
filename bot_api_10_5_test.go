package gogram_test

import (
	"encoding/json"
	"testing"

	"github.com/darxnet/gogram"
	"github.com/darxnet/gogram/tgtest"
)

// setMySettings changes only the fields that are passed, so false, 0 and ""
// must be sent when set and nothing must be sent for the rest.
func TestSetMySettings_SendsZeroValuesAndOmitsUnset(t *testing.T) {
	t.Parallel()

	params := new(gogram.SetMySettingsParams)
	params.Option(
		gogram.WithSetMySettingsSupportsInlineQueries(false),
		gogram.WithSetMySettingsCanReadAllGroupMessages(false),
		gogram.WithSetMySettingsInlineQueryPlaceholder(""),
		gogram.WithSetMySettingsInlineFeedbackShareDenominator(0),
		gogram.WithSetMySettingsCallbackQueryMode("inline"),
	)

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err = json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}

	want := map[string]any{
		"supports_inline_queries":           false,
		"can_read_all_group_messages":       false,
		"inline_query_placeholder":          "",
		"inline_feedback_share_denominator": float64(0),
		"callback_query_mode":               "inline",
	}

	if len(got) != len(want) {
		t.Fatalf("sent %s, want exactly %v", data, want)
	}

	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %#v, want %#v (sent %s)", k, got[k], v, data)
		}
	}

	empty, err := json.Marshal(new(gogram.SetMySettingsParams))
	if err != nil {
		t.Fatal(err)
	}

	if string(empty) != "{}" {
		t.Errorf("empty params sent %s, want {}", empty)
	}
}

// getMe returns BotFullInfo; Me caches it and the router reads the
// username from it.
func TestMe_BotFullInfo(t *testing.T) {
	t.Parallel()

	rec, client := newRecorderClient(t)
	rec.WillReturn("getMe", &gogram.BotFullInfo{
		ID:              42,
		IsBot:           true,
		Username:        "gogram_bot",
		ActiveUsernames: []string{"gogram_bot"},
		HasTopicSidebar: true,
	})

	me, err := client.Me(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if me.ID != 42 || me.Username != "gogram_bot" || !me.HasTopicSidebar {
		t.Fatalf("Me() = %+v", me)
	}

	if _, err = client.Me(t.Context()); err != nil {
		t.Fatal(err)
	}

	if n := len(rec.Calls()); n != 1 {
		t.Fatalf("getMe called %d times, want 1 (cached)", n)
	}
}

// With callback_query_mode "inline" a press on a button of the bot's own
// message carries inline_message_id and no message: Context methods edit by
// inline_message_id and the accessors do not panic.
func TestCallbackQuery_InlineMode(t *testing.T) {
	t.Parallel()

	rec, client := newRecorderClient(t)

	update := &gogram.Update{CallbackQuery: &gogram.CallbackQuery{
		ID:              "cq",
		From:            tgtest.User,
		InlineMessageID: "im1",
		ChatInstance:    "ci",
		Data:            "ping",
	}}

	router := gogram.NewRouter()
	router.Use(gogram.AutoAnswer())
	router.HandleOnCallbackQuery(func(ctx *gogram.Context, _ *gogram.CallbackQuery) error {
		if ctx.Message() != nil || ctx.Chat() != nil {
			t.Errorf("message %+v chat %+v, want nil", ctx.Message(), ctx.Chat())
		}

		if ctx.User().ID != tgtest.User.ID {
			t.Errorf("user = %+v", ctx.User())
		}

		return ctx.EditMessageText("pong")
	})
	router.Process(tgtest.NewContext(t, client, update))

	rec.AssertSequence(t, "editMessageText", "answerCallbackQuery")
	rec.AssertCalled(t, "editMessageText", tgtest.Match(func(p *gogram.EditMessageTextParams) bool {
		return p.InlineMessageID == "im1" && p.MessageID == 0 && p.ChatID.IsZero() && p.Text == "pong"
	}))
}

func TestWriteAccessAllowed_Inviter(t *testing.T) {
	t.Parallel()

	var w gogram.WriteAccessAllowed
	if err := json.Unmarshal([]byte(`{"inviter":{"id":7,"is_bot":false,"first_name":"A"}}`), &w); err != nil {
		t.Fatal(err)
	}

	if w.Inviter == nil || w.Inviter.ID != 7 {
		t.Fatalf("inviter = %+v", w.Inviter)
	}
}
