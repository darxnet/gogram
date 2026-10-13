package gogram_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/darxnet/gogram"
)

func TestChatID_JSON(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   gogram.ChatID
		json string
	}{
		{gogram.ChatID{ID: 7}, `7`},
		{gogram.ChatID{ID: -1001}, `-1001`},
		{gogram.Username("@telegram"), `"@telegram"`},
		{gogram.Username("telegram"), `"@telegram"`},
		{gogram.ChatID{}, `0`},
	}

	for _, tc := range cases {
		b, err := json.Marshal(tc.in)
		if err != nil || string(b) != tc.json {
			t.Fatalf("Marshal(%+v) = %s, %v; want %s", tc.in, b, err, tc.json)
		}

		var out gogram.ChatID
		if err = json.Unmarshal(b, &out); err != nil || out != tc.in {
			t.Fatalf("Unmarshal(%s) = %+v, %v; want %+v", b, out, err, tc.in)
		}
	}

	var fromString gogram.ChatID
	if err := json.Unmarshal([]byte(`"42"`), &fromString); err != nil || fromString.ID != 42 {
		t.Fatalf("numeric string: %+v, %v", fromString, err)
	}

	if err := json.Unmarshal([]byte(`true`), &fromString); err == nil {
		t.Fatal("bool must be rejected")
	}

	if gogram.Username("x").String() != "@x" || (gogram.ChatID{ID: 5}).String() != "5" {
		t.Fatal("String() mismatch")
	}
}

func TestChatID_OmitZero(t *testing.T) {
	t.Parallel()

	// Required chat_id is always present; optional ones only when set.
	b, err := json.Marshal(gogram.SendMessageParams{Text: "x"})
	if err != nil || !strings.Contains(string(b), `"chat_id":0`) {
		t.Fatalf("sendMessage: %s, %v", b, err)
	}

	b, err = json.Marshal(gogram.CopyMessageParams{FromChatID: gogram.ChatID{ID: 1}})
	if err != nil || !strings.Contains(string(b), `"from_chat_id":1`) {
		t.Fatalf("copyMessage: %s, %v", b, err)
	}

	b, err = json.Marshal(gogram.AnswerCallbackQueryParams{})
	if err != nil || strings.Contains(string(b), "chat_id") {
		t.Fatalf("optional chat_id must be omitted: %s, %v", b, err)
	}
}
