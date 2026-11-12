package gogram

import (
	"encoding/json"
	"fmt"
)

// UndecodableUpdateError reports an update that could not be decoded, for
// example because Telegram changed a field's type. The update is skipped
// so that it does not stop getUpdates or the webhook; the error reaches
// the router's error handler (see [Router.SetHandlerErr]).
type UndecodableUpdateError struct {
	UpdateID int64           // 0 when even update_id could not be read
	Raw      json.RawMessage // the update as received
	Err      error           // the decoding error
}

func (e *UndecodableUpdateError) Error() string {
	return fmt.Sprintf("gogram: skipped update %d that could not be decoded: %v", e.UpdateID, e.Err)
}

func (e *UndecodableUpdateError) Unwrap() error {
	return e.Err
}

// newUndecodableUpdateError reads what it can of an update that failed to decode.
func newUndecodableUpdateError(raw []byte, err error) *UndecodableUpdateError {
	var id struct {
		UpdateID int64 `json:"update_id"`
	}

	_ = json.Unmarshal(raw, &id) //nolint:errcheck // best effort: the update is already broken

	return &UndecodableUpdateError{UpdateID: id.UpdateID, Raw: json.RawMessage(raw), Err: err}
}

// updateBatch is the result of getUpdates. The whole batch is decoded in
// one pass; only when that fails is it decoded update by update, so one
// update with an unexpected shape is skipped instead of failing the batch
// forever (the offset would never move past it).
type updateBatch struct {
	updates []Update
	skipped []*UndecodableUpdateError
}

// UnmarshalJSON implements [json.Unmarshaler].
func (b *updateBatch) UnmarshalJSON(data []byte) error {
	b.skipped = nil

	err := json.Unmarshal(data, &b.updates)
	if err == nil {
		return nil
	}

	var raws []json.RawMessage
	if json.Unmarshal(data, &raws) != nil {
		return err // not even a JSON array: a real error
	}

	b.updates = make([]Update, 0, len(raws))

	for _, raw := range raws {
		var update Update
		if updateErr := json.Unmarshal(raw, &update); updateErr != nil {
			b.skipped = append(b.skipped, newUndecodableUpdateError(raw, updateErr))
			continue
		}

		b.updates = append(b.updates, update)
	}

	return nil
}
