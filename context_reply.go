package gogram

// Reply sends a text reply to the current message in the same chat.
// If the context has a message, it is quoted via ReplyParameters.
// Additional options (e.g. parse mode, inline keyboard) can be passed as opts.
func (ctx *Context) Reply(text string, opts ...SendMessageOption) error {
	params := ctx.NewSendMessageParams(text, opts...)

	if m := ctx.Message(); m != nil && m.MessageID != 0 && params.ReplyParameters == nil {
		params.ReplyParameters = &ReplyParameters{MessageID: m.MessageID}
	}

	_, err := ctx.client.SendMessage(ctx.Detached(), params)

	return err
}

// Command returns the bot command of the current message ("/start" for
// "/start@bot payload"), or "" when the message is not a command.
func (ctx *Context) Command() string {
	return retrieveCommand(ctx.Text())
}

// Args returns the space-separated arguments after the command or first
// word of the current message, or nil.
func (ctx *Context) Args() []string {
	return extractArgs(ctx.Text())
}
