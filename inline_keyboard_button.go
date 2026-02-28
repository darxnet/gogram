package gogram

import "strings"

// WithArgs returns a copy of the button with CallbackData appended with arguments.
// Format: 'callback_data args[0] args[1] ... args[N-1] args[N]'.
func (b *InlineKeyboardButton) WithArgs(args ...string) InlineKeyboardButton {
	v := *b

	if i := strings.IndexByte(v.CallbackData, ' '); i != -1 {
		v.CallbackData = v.CallbackData[:i]
	}

	if len(args) != 0 {
		v.CallbackData += " " + strings.Join(args, " ")
	}

	return v
}

// WithPayload returns a copy of the button with CallbackData appended with a payload.
// Format: 'callback_data payload'.
func (b *InlineKeyboardButton) WithPayload(payload string) InlineKeyboardButton {
	v := *b

	if i := strings.IndexByte(v.CallbackData, ' '); i != -1 {
		v.CallbackData = v.CallbackData[:i]
	}

	if payload != "" {
		v.CallbackData += " " + payload
	}

	return v
}

// SetText sets the text of the button.
func (b *InlineKeyboardButton) SetText(text string) *InlineKeyboardButton {
	b.Text = text
	return b
}

// InlineKeyboardRow represents a row of inline keyboard buttons.
type InlineKeyboardRow = []InlineKeyboardButton

// InlineKeyboard represents an inline keyboard markup.
type InlineKeyboard = []InlineKeyboardRow
