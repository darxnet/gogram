package gogram

func (cq *CallbackQuery) Payload() string {
	return ExtractPayload(cq.Data)
}

func (cq *CallbackQuery) Args() []string {
	return ExtractArgs(cq.Data)
}
