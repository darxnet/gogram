package gogram

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// UnmarshalJSON implements [json.Unmarshaler]. The Bot API server sends
// draft_id as a JSON string although it is documented as an Integer
// (tdlib/telegram-bot-api#945), so both forms are accepted; without this
// the whole stopped_message_generation update fails to decode and is
// skipped.
func (r *MessageGenerationStopped) UnmarshalJSON(data []byte) error {
	type plain MessageGenerationStopped

	var v struct {
		*plain

		DraftID json.RawMessage `json:"draft_id"`
	}

	v.plain = (*plain)(r)

	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	r.DraftID = 0

	raw := v.DraftID
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}

	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}

		raw = []byte(s)
	}

	id, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil {
		return fmt.Errorf("gogram: draft_id %s: %w", v.DraftID, err)
	}

	r.DraftID = id

	return nil
}
