package gogram

// WithEditMessageTextInlineKeyboard sets the inline keyboard for the message to be edited.
func WithEditMessageTextInlineKeyboard(keyboard [][]InlineKeyboardButton) EditMessageTextOption {
	return func(params *EditMessageTextParams) {
		if params.ReplyMarkup == nil {
			params.ReplyMarkup = &InlineKeyboardMarkup{}
		}

		params.ReplyMarkup.InlineKeyboard = keyboard
	}
}

// WithEditMessageTextInlineButtons sets the inline keyboard
// with a single column of buttons for the message to be edited.
func WithEditMessageTextInlineButtons(buttons ...InlineKeyboardButton) EditMessageTextOption {
	return func(params *EditMessageTextParams) {
		keyboard := make(InlineKeyboard, 0, len(buttons))
		for i := range buttons {
			keyboard = append(keyboard, InlineKeyboardRow{buttons[i]})
		}

		WithEditMessageTextInlineKeyboard(keyboard)(params)
	}
}
