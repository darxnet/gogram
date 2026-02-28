package gogram

// WithSendMessageInlineKeyboard sets the inline keyboard for the message.
func WithSendMessageInlineKeyboard(keyboard [][]InlineKeyboardButton) SendMessageOption {
	return func(params *SendMessageParams) SendMessageOption {
		if params.ReplyMarkup == nil || params.ReplyMarkup.InlineKeyboardMarkup == nil {
			params.ReplyMarkup = &ReplyMarkup{InlineKeyboardMarkup: &InlineKeyboardMarkup{}}
		}

		previous := params.ReplyMarkup.InlineKeyboard
		params.ReplyMarkup.InlineKeyboard = keyboard

		return WithSendMessageInlineKeyboard(previous)
	}
}

// WithSendMessageInlineButtons sets the inline keyboard with a single column of buttons for the message.
func WithSendMessageInlineButtons(buttons ...InlineKeyboardButton) SendMessageOption {
	return func(params *SendMessageParams) SendMessageOption {
		keyboard := make(InlineKeyboard, 0, len(buttons))
		for i := range buttons {
			keyboard = append(keyboard, InlineKeyboardRow{buttons[i]})
		}

		if params.ReplyMarkup == nil || params.ReplyMarkup.InlineKeyboardMarkup == nil {
			params.ReplyMarkup = &ReplyMarkup{InlineKeyboardMarkup: &InlineKeyboardMarkup{}}
		}

		previous := params.ReplyMarkup.InlineKeyboard
		params.ReplyMarkup.InlineKeyboard = keyboard

		return WithSendMessageInlineKeyboard(previous)
	}
}
