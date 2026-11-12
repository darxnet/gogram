package gogram_test

import (
	"testing"

	"github.com/darxnet/gogram"
	"github.com/darxnet/gogram/tgtest"
)

func TestMessageThreadID_TopicsOnly(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		topic bool
		want  int64
	}{
		{"forum topic", true, 5},
		{"reply thread without topics", false, 0}, // "message thread not found" otherwise
	} {
		rec := tgtest.NewRecorder()

		client, err := gogram.NewClient(testToken, gogram.WithCaller(rec))
		if err != nil {
			t.Fatal(err)
		}

		group := gogram.Chat{ID: -100, Type: gogram.ChatSupergroup}
		update := tgtest.Message(&group, "hi")
		update.Message.MessageThreadID = 5
		update.Message.IsTopicMessage = tc.topic

		ctx := tgtest.NewContext(t, client, update)
		if err = ctx.SendMessage("x"); err != nil {
			t.Fatal(err)
		}

		if err = ctx.Reply("x"); err != nil {
			t.Fatal(err)
		}

		for _, call := range rec.Calls() {
			if got := call.Params.(*gogram.SendMessageParams).MessageThreadID; got != tc.want {
				t.Errorf("%s: %s message_thread_id = %d, want %d", tc.name, call.Method, got, tc.want)
			}
		}
	}
}
