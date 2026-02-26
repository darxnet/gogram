package gogram

import "strings"

// WithArgs return copy of button with CallbackData looks like 'callback_data args[0] args[1] ... args[N-1] args[N]'.
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

// WithPayload return copy of button with CallbackData looks like 'callback_data payload'.
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

func (b *InlineKeyboardButton) SetText(text string) *InlineKeyboardButton {
	b.Text = text
	return b
}

type InlineKeyboardRow = []InlineKeyboardButton

type InlineKeyboard = []InlineKeyboardRow
