package gogram_test

import (
	"slices"
	"testing"

	"github.com/darxnet/gogram"
	"github.com/darxnet/gogram/tgtest"
)

func TestCommand_AddressedToOtherBot(t *testing.T) {
	t.Parallel()

	rec := tgtest.NewRecorder()
	rec.WillReturn("getMe", &gogram.User{ID: 1, IsBot: true, Username: "my_bot"})

	client, err := gogram.NewClient(testToken, gogram.WithCaller(rec))
	if err != nil {
		t.Fatal(err)
	}

	var commands, messages []string

	router := gogram.NewRouter()
	router.HandleCommand("/start", func(_ *gogram.Context, m *gogram.Message) error {
		commands = append(commands, m.Text)
		return nil
	})
	router.HandleOnMessage(func(_ *gogram.Context, m *gogram.Message) error {
		messages = append(messages, m.Text)
		return nil
	})

	group := gogram.Chat{ID: -5, Type: gogram.ChatSupergroup}

	for _, text := range []string{"/start", "/start@my_bot", "/start@MY_BOT x", "/start@other_bot", "/start\nx"} {
		router.Process(tgtest.NewContext(t, client, tgtest.Message(&group, text)))
	}

	if want := []string{"/start", "/start@my_bot", "/start@MY_BOT x", "/start\nx"}; !slices.Equal(commands, want) {
		t.Fatalf("commands = %q, want %q", commands, want)
	}

	// A command for another bot is an ordinary message for this one.
	if want := []string{"/start@other_bot"}; !slices.Equal(messages, want) {
		t.Fatalf("messages = %q, want %q", messages, want)
	}

	// getMe is fetched once, on the first @-addressed command.
	if calls := rec.Calls(); len(calls) != 1 || calls[0].Method != "getMe" {
		t.Fatalf("calls = %+v", calls)
	}

	ctx := tgtest.NewContext(t, client, tgtest.Message(&group, "/start@other_bot"))
	if ctx.Command() != "" || gogram.FilterCommand("start")(ctx) {
		t.Fatalf("Command() = %q for another bot", ctx.Command())
	}
}

func TestCommand_GetMeFails(t *testing.T) {
	t.Parallel()

	rec := tgtest.NewRecorder()
	rec.WillFail("getMe", errDenied)

	client, err := gogram.NewClient(testToken, gogram.WithCaller(rec))
	if err != nil {
		t.Fatal(err)
	}

	// Without the bot's username the command is accepted, as before the check.
	if ctx := tgtest.NewContext(t, client, tgtest.Text(1, "/start@other_bot")); ctx.Command() != "/start" {
		t.Fatalf("Command() = %q, want /start", ctx.Command())
	}
}

func TestPayload_LineBreak(t *testing.T) {
	t.Parallel()

	for text, want := range map[string]string{
		"/start abc":    "abc",
		"/start\nabc":   "abc",
		"/start\tabc d": "abc d",
		"/start":        "",
	} {
		if got := (&gogram.Message{Text: text}).Payload(); got != want {
			t.Errorf("Payload(%q) = %q, want %q", text, got, want)
		}
	}

	if args := (&gogram.Message{Text: "/start\na b"}).Args(); !slices.Equal(args, []string{"a", "b"}) {
		t.Errorf("Args = %q", args)
	}
}

func TestFSM_SenderChat(t *testing.T) {
	t.Parallel()

	client, err := gogram.NewClient(testToken, gogram.WithCaller(tgtest.NewRecorder()))
	if err != nil {
		t.Fatal(err)
	}

	group := gogram.Chat{ID: -100, Type: gogram.ChatSupergroup}
	anonymousAdmin := gogram.User{ID: 1087968824, IsBot: true, Username: "GroupAnonymousBot"}
	channelBot := gogram.User{ID: 136817688, IsBot: true, Username: "Channel_Bot"}

	message := func(from gogram.User, senderChat *gogram.Chat) *gogram.Context {
		return tgtest.NewContext(t, client, &gogram.Update{Message: &gogram.Message{
			MessageID: 1, From: &from, SenderChat: senderChat, Chat: group, Text: "x",
		}})
	}

	channelA := &gogram.Chat{ID: -200, Type: gogram.ChatChannel}
	channelB := &gogram.Chat{ID: -300, Type: gogram.ChatChannel}

	if err = message(channelBot, channelA).SetState("a"); err != nil {
		t.Fatal(err)
	}

	if err = message(anonymousAdmin, &group).SetState("admin"); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		ctx  *gogram.Context
		want gogram.State
	}{
		{"same channel", message(channelBot, channelA), "a"},
		{"other channel, same service user", message(channelBot, channelB), ""},
		{"anonymous admin", message(anonymousAdmin, &group), "admin"},
		{"the service user itself", message(channelBot, nil), ""},
		{"regular user", message(tgtest.User, nil), ""},
	} {
		if got, stateErr := tc.ctx.State(); stateErr != nil || got != tc.want {
			t.Errorf("%s: State() = %q, %v; want %q", tc.name, got, stateErr, tc.want)
		}
	}

	// A callback query belongs to the user who pressed the button, even on
	// a message sent on behalf of a chat.
	cb := tgtest.Callback(group.ID, "x")
	cb.CallbackQuery.Message.SenderChat = channelA

	if got, _ := tgtest.NewContext(t, client, cb).State(); got != "" {
		t.Errorf("callback on a channel message: State() = %q, want \"\"", got)
	}
}

func FuzzEntityText(f *testing.F) {
	f.Add("😀 /start é @user", int64(3), int64(6))
	f.Add("abc", int64(-1), int64(10))
	f.Add("😀", int64(1), int64(1))

	f.Fuzz(func(t *testing.T, text string, offset, length int64) {
		got := gogram.EntityText(text, &gogram.MessageEntity{Offset: offset, Length: length})
		if len(got) > len(text) {
			t.Fatalf("EntityText longer than text: %q", got)
		}
	})
}
