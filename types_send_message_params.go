package gogram

// WithSendMessageInlineKeyboard sets the inline keyboard for the message.
func WithSendMessageInlineKeyboard(keyboard [][]InlineKeyboardButton) SendMessageOption {
	return func(params *SendMessageParams) {
		if params.ReplyMarkup == nil || params.ReplyMarkup.InlineKeyboardMarkup == nil {
			params.ReplyMarkup = &ReplyMarkup{InlineKeyboardMarkup: &InlineKeyboardMarkup{}}
		}

		params.ReplyMarkup.InlineKeyboard = keyboard
	}
}

// WithSendMessageInlineButtons sets the inline keyboard with a single column of buttons for the message.
func WithSendMessageInlineButtons(buttons ...InlineKeyboardButton) SendMessageOption {
	return func(params *SendMessageParams) {
		keyboard := make(InlineKeyboard, 0, len(buttons))
		for i := range buttons {
			keyboard = append(keyboard, InlineKeyboardRow{buttons[i]})
		}

		WithSendMessageInlineKeyboard(keyboard)(params)
	}
}
