package gogram

import "time"

// Payload extracts the payload from the message text (string after the first space).
func (m *Message) Payload() string {
	return ExtractPayload(m.Text)
}

// Args extracts arguments from the message text (strings separated by spaces, excluding the first word).
func (m *Message) Args() []string {
	return ExtractArgs(m.Text)
}

// Time returns the message date as a time.Time.
func (m *Message) Time() time.Time {
	return time.Unix(m.Date, 0)
}
