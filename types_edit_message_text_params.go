package gogram

func WithEditMessageTextInlineKeyboard(keyboard [][]InlineKeyboardButton) EditMessageTextOption {
	return func(params *EditMessageTextParams) EditMessageTextOption {
		if params.ReplyMarkup == nil {
			params.ReplyMarkup = &InlineKeyboardMarkup{}
		}

		previous := params.ReplyMarkup.InlineKeyboard
		params.ReplyMarkup.InlineKeyboard = keyboard

		return WithEditMessageTextInlineKeyboard(previous)
	}
}

func WithEditMessageTextInlineButtons(buttons ...InlineKeyboardButton) EditMessageTextOption {
	return func(params *EditMessageTextParams) EditMessageTextOption {
		keyboard := make(InlineKeyboard, 0, len(buttons))
		for i := range buttons {
			keyboard = append(keyboard, InlineKeyboardRow{buttons[i]})
		}

		if params.ReplyMarkup == nil {
			params.ReplyMarkup = &InlineKeyboardMarkup{}
		}

		previous := params.ReplyMarkup.InlineKeyboard
		params.ReplyMarkup.InlineKeyboard = keyboard

		return WithEditMessageTextInlineKeyboard(previous)
	}
}
