package gogram

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type jsonWrapper[T any] struct {
	pointer *T
}

func (w jsonWrapper[T]) Value() (driver.Value, error) {
	return json.Marshal(w.pointer)
}

func (w jsonWrapper[T]) Scan(src any) error {
	if src == nil {
		return nil
	}

	var buf []byte

	switch v := src.(type) {
	case []byte:
		buf = v
	case string:
		buf = ConvertStringToBytes(v)
	default:
		return fmt.Errorf("unexpected type: %T", src)
	}

	return json.Unmarshal(buf, w.pointer)
}

func AsJSON[T any](pointer *T) jsonWrapper[T] {
	return jsonWrapper[T]{pointer: pointer}
}
