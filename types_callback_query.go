package gogram

// Payload extracts the payload from the callback data (string after the first space).
func (cq *CallbackQuery) Payload() string {
	return ExtractPayload(cq.Data)
}

// Args extracts arguments from the callback data (strings separated by spaces, excluding the first word).
func (cq *CallbackQuery) Args() []string {
	return ExtractArgs(cq.Data)
}
