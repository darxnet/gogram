package gogram

import "time"

func (m *Message) Payload() string {
	return ExtractPayload(m.Text)
}

func (m *Message) Args() []string {
	return ExtractArgs(m.Text)
}

func (m *Message) Time() time.Time {
	return time.Unix(m.Date, 0)
}
