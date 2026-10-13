package gogram

// Payload extracts the payload from the callback data (string after the first space).
func (cq *CallbackQuery) Payload() string {
	return extractPayload(cq.Data)
}

// Args extracts arguments from the callback data (strings separated by spaces, excluding the first word).
func (cq *CallbackQuery) Args() []string {
	return extractArgs(cq.Data)
}
