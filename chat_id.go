package gogram

import (
	"errors"
	"strconv"
)

// ChatID identifies a chat for chat_id parameters: a numeric id, or the
// @username of a public channel or supergroup. It marshals to a JSON
// number or string, as the Bot API expects ("Integer or String").
//
//	gogram.ChatID{ID: 123}
//	gogram.Username("telegram")
type ChatID struct {
	ID       int64
	Username string // without the leading "@"
}

// ErrInvalidChatID is returned when a chat_id JSON value is neither a
// number nor a string.
var ErrInvalidChatID = errors.New("gogram: invalid chat_id")

// Username returns the ChatID of a public chat by username, with or
// without the leading "@".
func Username(name string) ChatID {
	if name != "" && name[0] == '@' {
		name = name[1:]
	}

	return ChatID{Username: name}
}

// IsZero reports whether c identifies no chat.
func (c ChatID) IsZero() bool {
	return c.ID == 0 && c.Username == ""
}

// String returns the value as sent to the API: the id in decimal, or
// "@username" when the id is unknown.
func (c ChatID) String() string {
	if c.ID != 0 || c.Username == "" {
		return strconv.FormatInt(c.ID, 10)
	}

	return "@" + c.Username
}

// AppendText implements [encoding.TextAppender].
func (c ChatID) AppendText(b []byte) ([]byte, error) {
	if c.ID != 0 || c.Username == "" {
		return strconv.AppendInt(b, c.ID, 10), nil
	}

	return append(append(b, '@'), c.Username...), nil
}

// MarshalJSON writes the id as a number, or the username as a string.
func (c ChatID) MarshalJSON() ([]byte, error) {
	if c.ID != 0 || c.Username == "" {
		return strconv.AppendInt(nil, c.ID, 10), nil
	}

	b := make([]byte, 0, len(c.Username)+3)
	b = append(b, '"', '@')
	b = append(b, c.Username...)

	return append(b, '"'), nil
}

// UnmarshalJSON accepts a number, a numeric string or an "@username" string.
func (c *ChatID) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}

	if data[0] == '"' {
		if len(data) < 2 || data[len(data)-1] != '"' {
			return ErrInvalidChatID
		}

		s := string(data[1 : len(data)-1])
		if s != "" && s[0] == '@' {
			*c = ChatID{Username: s[1:]}
			return nil
		}

		data = []byte(s)
	}

	id, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return ErrInvalidChatID
	}

	*c = ChatID{ID: id}

	return nil
}
