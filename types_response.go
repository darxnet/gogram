package gogram

import "encoding/json"

// Response represents a root API JSON object.
type Response struct {
	Parameters  *ResponseParameters `json:"parameters,omitempty"`
	Description string              `json:"description,omitempty"`
	Result      json.RawMessage     `json:"result,omitempty"`
	ErrorCode   int                 `json:"error_code,omitempty"`
	OK          bool                `json:"ok"`
}
