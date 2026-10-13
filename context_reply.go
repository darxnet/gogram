package gogram

// Reply sends a text reply to the current message in the same chat.
// If the context has a message, it is quoted via ReplyParameters.
// Additional options (e.g. parse mode, inline keyboard) can be passed as opts.
func (ctx *Context) Reply(text string, opts ...SendMessageOption) error {
	m := ctx.Message()
	if m != nil && m.MessageID != 0 {
		reply := WithSendMessageReplyParameters(&ReplyParameters{MessageID: m.MessageID})
		opts = append([]SendMessageOption{reply}, opts...)
	}
	return ctx.SendMessage(text, opts...)
}
