package gogram

// WithSendMessageInlineKeyboard
// helps add keyboard to ReplyMarkup.
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

// WithSendMessageInlineButtons
// helps add bunch of buttons in one column.
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
