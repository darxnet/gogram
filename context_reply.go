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
// "/start@this_bot payload"), or "" when the message is not a command or
// the command is addressed to another bot ("/start@other_bot"), as in a
// group with several bots.
func (ctx *Context) Command() string {
	return currentCommand(ctx)
}

// Args returns the space-separated arguments after the command or first
// word of the current message, or nil.
func (ctx *Context) Args() []string {
	return extractArgs(ctx.Text())
}

// Ephemeral returns ephemeral message parameters addressed to the current
// user, for any send method that accepts them (WithSend…EphemeralMessageParameters):
//
//   - for a callback query, an ephemeral reply to it: only the user who
//     pressed the button sees the message, in any chat except channel direct
//     messages chats, and the chat id of the method is ignored. The query
//     still has to be answered (see [AutoAnswer]);
//   - otherwise, a message to the sender of the update. Outside a reply to
//     an incoming ephemeral message this needs the bot to be a chat administrator.
//
// It returns nil when the update has neither a callback query nor a sender.
func (ctx *Context) Ephemeral() *EphemeralMessageParameters {
	if ctx.update != nil && ctx.update.CallbackQuery != nil {
		return &EphemeralMessageParameters{CallbackQueryID: ctx.update.CallbackQuery.ID}
	}

	if id := ctx.userID(); id != 0 {
		return &EphemeralMessageParameters{ReceiverUserID: id}
	}

	return nil
}

// ReplyEphemeral sends a text message that only the current user sees in
// the chat, using [Context.Ephemeral]. In reply to an incoming ephemeral
// message (an ephemeral command) it is threaded to that message, which any
// bot may do within 15 seconds; Telegram does not guarantee delivery.
func (ctx *Context) ReplyEphemeral(text string, opts ...SendMessageOption) error {
	params := ctx.NewSendMessageParams(text, opts...)

	if params.EphemeralMessageParameters == nil {
		params.EphemeralMessageParameters = ctx.Ephemeral()
	}

	if m := ctx.Message(); m != nil && m.EphemeralMessageID != 0 && params.ReplyParameters == nil &&
		(ctx.update == nil || ctx.update.CallbackQuery == nil) {
		params.ReplyParameters = &ReplyParameters{EphemeralMessageID: m.EphemeralMessageID}
	}

	_, err := ctx.client.SendMessage(ctx.Detached(), params)

	return err
}
