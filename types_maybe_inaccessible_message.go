package gogram

import "encoding/json"

func (r *MaybeInaccessibleMessage) UnmarshalJSON(bs []byte) error {
	v := new(Message)

	err := json.Unmarshal(bs, v)
	if err != nil {
		return err
	}

	if v.Date != 0 {
		r.Message = v
	} else {
		r.InaccessibleMessage = &InaccessibleMessage{
			Chat:      v.Chat,
			MessageID: v.MessageID,
		}
	}

	return nil
}

func (r *MaybeInaccessibleMessage) MessageID() int64 {
	if r.Message != nil {
		return r.Message.MessageID
	}

	return r.InaccessibleMessage.MessageID
}

func (r *MaybeInaccessibleMessage) Chat() *Chat {
	if r.Message != nil {
		return &r.Message.Chat
	}

	return &r.InaccessibleMessage.Chat
}
