package gogram_test

import (
	"testing"

	"github.com/darxnet/gogram"
)

func TestFilters(t *testing.T) {
	t.Parallel()

	client, err := gogram.NewClient(testToken)
	if err != nil {
		t.Fatal(err)
	}

	ctxOf := func(u *gogram.Update) *gogram.Context { return gogram.NewTestContext(t.Context(), client, u) }

	private := ctxOf(&gogram.Update{Message: &gogram.Message{
		Text: "/start@bot now", From: &gogram.User{ID: 1}, Chat: gogram.Chat{ID: 1, Type: gogram.ChatPrivate},
		Photo: []gogram.PhotoSize{{Width: 1}},
	}})
	group := ctxOf(&gogram.Update{Message: &gogram.Message{
		Text: "hi", From: &gogram.User{ID: 2}, Chat: gogram.Chat{ID: -5, Type: gogram.ChatSupergroup},
		Document: &gogram.Document{}, ReplyToMessage: &gogram.Message{},
	}})
	callback := ctxOf(&gogram.Update{CallbackQuery: &gogram.CallbackQuery{Data: "pg 2"}})

	cases := []struct {
		name   string
		filter gogram.Filter
		want   map[*gogram.Context]bool
	}{
		{"private", gogram.FilterPrivate(), map[*gogram.Context]bool{private: true, group: false, callback: false}},
		{"group", gogram.FilterGroup(), map[*gogram.Context]bool{private: false, group: true}},
		{"command", gogram.FilterCommand("start"), map[*gogram.Context]bool{private: true, group: false}},
		{
			"and", gogram.FilterPrivate().And(gogram.FilterCommand("/start")),
			map[*gogram.Context]bool{private: true, group: false},
		},
		{"or", gogram.FilterChannel().Or(gogram.FilterGroup()), map[*gogram.Context]bool{private: false, group: true}},
		{"not", gogram.FilterPrivate().Not(), map[*gogram.Context]bool{private: false, group: true, callback: true}},
		{"users", gogram.FilterUsers(2, 3), map[*gogram.Context]bool{private: false, group: true}},
		{"photo", gogram.FilterMessagePhoto(), map[*gogram.Context]bool{private: true, group: false}},
		{"media", gogram.FilterMessageMedia(), map[*gogram.Context]bool{private: true, group: true, callback: false}},
		{"reply", gogram.FilterMessageReply(), map[*gogram.Context]bool{private: false, group: true}},
		{"callback", gogram.FilterCallbackPrefix("pg"), map[*gogram.Context]bool{private: false, callback: true}},
		{"callback exact", gogram.FilterCallbackPrefix("pg 2"), map[*gogram.Context]bool{callback: true}},
		{"callback other", gogram.FilterCallbackPrefix("p"), map[*gogram.Context]bool{callback: false}},
	}

	for _, tc := range cases {
		for ctx, want := range tc.want {
			if got := tc.filter(ctx); got != want {
				t.Errorf("%s: got %v, want %v", tc.name, got, want)
			}
		}
	}

	if private.Command() != "/start" || len(private.Args()) != 1 || private.Args()[0] != "now" {
		t.Fatalf("Command/Args = %q %v", private.Command(), private.Args())
	}

	if group.Command() != "" {
		t.Fatalf("Command on plain text = %q", group.Command())
	}
}

func TestCallGeneric(t *testing.T) {
	t.Parallel()

	client, err := gogram.NewClient(testToken, gogram.WithCaller(cannedCaller(`{"id":5,"is_bot":true,"first_name":"b"}`)))
	if err != nil {
		t.Fatal(err)
	}

	me, err := gogram.Call[*gogram.User](t.Context(), client, "getMe", nil)
	if err != nil || me.ID != 5 {
		t.Fatalf("Call = %+v, %v", me, err)
	}
}
