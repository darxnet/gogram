package gogram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Optional is a value that is either set or left to Telegram. It is used
// for the fields whose default on Telegram's side is not Go's zero value:
// with a plain bool, false could not be sent, as the field is omitted when
// zero. The zero Optional is unset: the field is not sent, and Telegram
// applies its default.
//
//	gogram.ChatPermissions{CanPinMessages: true, CanManageTopics: gogram.Some(false)}
//
// Optional is comparable when T is, and copying it copies the value.
type Optional[T any] struct {
	Value T
	Set   bool
}

// Some returns an Optional set to v.
func Some[T any](v T) Optional[T] {
	return Optional[T]{Value: v, Set: true}
}

// Or returns the value if it is set, def otherwise.
func (o Optional[T]) Or(def T) T {
	if o.Set {
		return o.Value
	}

	return def
}

// IsZero reports whether o is unset; with the omitzero option, encoding/json
// leaves an unset field out.
func (o Optional[T]) IsZero() bool {
	return !o.Set
}

// MarshalJSON encodes the value; an unset Optional encodes as null, which a
// field tagged omitzero never gets to.
func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if !o.Set {
		return []byte("null"), nil
	}

	return json.Marshal(o.Value)
}

// UnmarshalJSON sets o to the decoded value; null leaves it unset.
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		*o = Optional[T]{}
		return nil
	}

	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	*o = Some(v)

	return nil
}

// Format prints the value as fmt would, or <unset>; with %#v it prints Go
// syntax: gogram.Some(false) or gogram.Optional[bool]{}.
func (o Optional[T]) Format(f fmt.State, verb rune) {
	if verb == 'v' && f.Flag('#') {
		if o.Set {
			_, _ = fmt.Fprintf(f, "gogram.Some(%#v)", o.Value)
		} else {
			_, _ = fmt.Fprintf(f, "gogram.Optional[%T]{}", o.Value)
		}

		return
	}

	if !o.Set {
		_, _ = io.WriteString(f, "<unset>")
		return
	}

	_, _ = fmt.Fprintf(f, fmt.FormatString(f, verb), o.Value)
}
