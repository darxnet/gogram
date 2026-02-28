package gogram

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSONWrapper is a generic wrapper for handling JSON serialization/deserialization in SQL databases.
type JSONWrapper[T any] struct {
	pointer *T
}

// Value implements the driver.Valuer interface for JSON serialization.
func (w JSONWrapper[T]) Value() (driver.Value, error) {
	return json.Marshal(w.pointer)
}

// Scan implements the sql.Scanner interface for JSON deserialization.
func (w JSONWrapper[T]) Scan(src any) error {
	if src == nil {
		return nil
	}

	var buf []byte

	switch v := src.(type) {
	case []byte:
		buf = v
	case string:
		buf = []byte(v)
	default:
		//nolint:err113
		return fmt.Errorf("unexpected type: %T", src)
	}

	return json.Unmarshal(buf, w.pointer)
}

// AsJSON wraps a pointer to a struct to implement sql.Scanner and driver.Valuer interfaces.
// This allows storing complex types as JSON in the database.
func AsJSON[T any](pointer *T) JSONWrapper[T] {
	return JSONWrapper[T]{pointer: pointer}
}
