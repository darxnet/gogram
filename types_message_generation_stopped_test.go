package gogram_test

import (
	"encoding/json"
	"testing"

	"github.com/darxnet/gogram"
)

func TestMessageGenerationStopped_DraftID(t *testing.T) {
	t.Parallel()

	for raw, want := range map[string]int64{
		`"42"`:                  42, // what the Bot API server sends (tdlib/telegram-bot-api#945)
		`42`:                    42, // what the documentation promises
		`"-7"`:                  -7,
		`"9223372036854775807"`: 9223372036854775807,
		`null`:                  0,
	} {
		var update gogram.Update

		data := `{"update_id":1,"stopped_message_generation":{"chat":{"id":5,"type":"private"},` +
			`"message_thread_id":3,"draft_id":` + raw + `}}`
		if err := json.Unmarshal([]byte(data), &update); err != nil {
			t.Fatalf("draft_id %s: %v", raw, err)
		}

		got := update.StoppedMessageGeneration
		if got == nil || got.DraftID != want || got.Chat.ID != 5 || got.MessageThreadID != 3 {
			t.Fatalf("draft_id %s: got %+v, want DraftID %d", raw, got, want)
		}
	}

	var stopped gogram.MessageGenerationStopped
	if err := json.Unmarshal([]byte(`{"chat":{"id":5,"type":"private"}}`), &stopped); err != nil || stopped.DraftID != 0 {
		t.Fatalf("missing draft_id: %+v, %v", stopped, err)
	}

	for _, raw := range []string{`"x"`, `""`, `"4.2"`, `4.2`, `true`, `"9223372036854775808"`} {
		data := `{"chat":{"id":5,"type":"private"},"draft_id":` + raw + `}`
		if err := json.Unmarshal([]byte(data), &stopped); err == nil {
			t.Fatalf("draft_id %s: no error", raw)
		}
	}

	// Marshalling keeps the documented number.
	out, err := json.Marshal(&gogram.MessageGenerationStopped{DraftID: 42})
	if err != nil {
		t.Fatal(err)
	}

	var back map[string]any
	if err := json.Unmarshal(out, &back); err != nil || back["draft_id"] != float64(42) {
		t.Fatalf("marshal = %s", out)
	}
}

func TestStoppedMessageGeneration_StringDraftIDReachesHandler(t *testing.T) {
	t.Parallel()

	_, client := newRecorderClient(t)

	var draftID int64

	router := gogram.NewRouter()
	router.HandleOnStoppedMessageGeneration(func(_ *gogram.Context, m *gogram.MessageGenerationStopped) error {
		draftID = m.DraftID
		return nil
	})

	var update gogram.Update
	if err := json.Unmarshal([]byte(`{"update_id":1,"stopped_message_generation":`+
		`{"chat":{"id":5,"type":"private"},"draft_id":"42"}}`), &update); err != nil {
		t.Fatal(err)
	}

	router.Process(gogram.NewTestContext(t.Context(), client, &update))

	if draftID != 42 {
		t.Fatalf("handler got draft_id %d, want 42", draftID)
	}
}
