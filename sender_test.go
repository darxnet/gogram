package gogram_test

import (
	"testing"

	"github.com/darxnet/gogram"
	"github.com/darxnet/gogram/tgtest"
)

func TestSender(t *testing.T) {
	t.Parallel()

	client, err := gogram.NewClient(testToken, gogram.WithCaller(tgtest.NewRecorder()))
	if err != nil {
		t.Fatal(err)
	}

	group := gogram.Chat{ID: -100, Type: gogram.ChatSupergroup}
	channel := gogram.Chat{ID: -200, Type: gogram.ChatChannel}
	user := tgtest.User
	anonymousAdmin := gogram.User{ID: 1087968824, IsBot: true, Username: "GroupAnonymousBot"}
	channelBot := gogram.User{ID: 136817688, IsBot: true, Username: "Channel_Bot"}

	cbOnChannelMessage := tgtest.Callback(group.ID, "x")
	cbOnChannelMessage.CallbackQuery.Message.SenderChat = &channel

	for _, tc := range []struct {
		name     string
		update   *gogram.Update
		wantUser *gogram.User
		wantChat *gogram.Chat
	}{
		{
			name:     "user in a group",
			update:   &gogram.Update{Message: &gogram.Message{From: &user, Chat: group}},
			wantUser: &user,
		},
		{
			name:     "anonymous admin",
			update:   &gogram.Update{Message: &gogram.Message{From: &anonymousAdmin, SenderChat: &group, Chat: group}},
			wantChat: &group,
		},
		{
			name: "user posting as a channel",
			update: &gogram.Update{EditedMessage: &gogram.Message{
				From: &channelBot, SenderChat: &channel, Chat: group,
			}},
			wantChat: &channel,
		},
		{
			name:     "channel post",
			update:   &gogram.Update{ChannelPost: &gogram.Message{SenderChat: &channel, Chat: channel}},
			wantChat: &channel,
		},
		{
			name:     "callback query on a message sent as a channel",
			update:   cbOnChannelMessage,
			wantUser: &cbOnChannelMessage.CallbackQuery.From,
		},
		{
			name:     "reaction by a user",
			update:   &gogram.Update{MessageReaction: &gogram.MessageReactionUpdated{Chat: group, User: &user}},
			wantUser: &user,
		},
		{
			name:     "anonymous reaction",
			update:   &gogram.Update{MessageReaction: &gogram.MessageReactionUpdated{Chat: group, ActorChat: &group}},
			wantChat: &group,
		},
		{
			name:     "poll answer",
			update:   &gogram.Update{PollAnswer: &gogram.PollAnswer{User: &user}},
			wantUser: &user,
		},
		{
			name:     "anonymous poll answer",
			update:   &gogram.Update{PollAnswer: &gogram.PollAnswer{VoterChat: &channel}},
			wantChat: &channel,
		},
		{
			name:     "inline query",
			update:   &gogram.Update{InlineQuery: &gogram.InlineQuery{From: user}},
			wantUser: &user,
		},
		{
			name:   "no sender",
			update: &gogram.Update{ChatBoost: &gogram.ChatBoostUpdated{Chat: channel}},
		},
	} {
		got := tgtest.NewContext(t, client, tc.update).Sender()

		if !sameUser(got.User, tc.wantUser) || !sameChat(got.Chat, tc.wantChat) {
			t.Errorf("%s: Sender() = %+v, %+v; want %+v, %+v", tc.name, got.User, got.Chat, tc.wantUser, tc.wantChat)
		}

		wantID := int64(0)
		if tc.wantChat != nil {
			wantID = tc.wantChat.ID
		} else if tc.wantUser != nil {
			wantID = tc.wantUser.ID
		}

		if id := got.ID(); id != wantID {
			t.Errorf("%s: Sender().ID() = %d, want %d", tc.name, id, wantID)
		}
	}
}

func sameUser(a, b *gogram.User) bool {
	return a == nil && b == nil || a != nil && b != nil && a.ID == b.ID
}

func sameChat(a, b *gogram.Chat) bool {
	return a == nil && b == nil || a != nil && b != nil && a.ID == b.ID
}

// TestFSM_AnonymousVoter checks that an update without a user but with a
// sender chat (an anonymous poll answer) has a state of its own.
func TestFSM_AnonymousVoter(t *testing.T) {
	t.Parallel()

	client, err := gogram.NewClient(testToken, gogram.WithCaller(tgtest.NewRecorder()))
	if err != nil {
		t.Fatal(err)
	}

	answer := func(chatID int64) *gogram.Context {
		return tgtest.NewContext(t, client, &gogram.Update{PollAnswer: &gogram.PollAnswer{
			PollID: "p", VoterChat: &gogram.Chat{ID: chatID, Type: gogram.ChatChannel},
		}})
	}

	if err = answer(-200).SetState("voted"); err != nil {
		t.Fatal(err)
	}

	if got, _ := answer(-200).State(); got != "voted" {
		t.Errorf("same voter chat: State() = %q, want \"voted\"", got)
	}

	if got, _ := answer(-300).State(); got != "" {
		t.Errorf("other voter chat: State() = %q, want \"\"", got)
	}

	if _, ok := tgtest.NewContext(t, client, &gogram.Update{
		ChatBoost: &gogram.ChatBoostUpdated{Chat: gogram.Chat{ID: -200}},
	}).FSM(); ok {
		t.Error("FSM() ok on an update without a sender")
	}
}
