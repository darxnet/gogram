package gogram

// InlineKeyboardBuilder builds an [InlineKeyboardMarkup] row by row:
//
//	markup := gogram.NewInlineKeyboardBuilder().
//		Text("Yes", "confirm 1").Text("No", "confirm 0").Row().
//		URL("Docs", "https://example.com").
//		Markup()
//
// Buttons are appended to the current row; Row starts the next one.
// The zero value is ready to use.
type InlineKeyboardBuilder struct {
	rows [][]InlineKeyboardButton
	row  []InlineKeyboardButton
}

// NewInlineKeyboardBuilder returns an empty builder.
func NewInlineKeyboardBuilder() *InlineKeyboardBuilder {
	return &InlineKeyboardBuilder{}
}

// Add appends buttons to the current row.
func (b *InlineKeyboardBuilder) Add(buttons ...InlineKeyboardButton) *InlineKeyboardBuilder {
	b.row = append(b.row, buttons...)
	return b
}

// Text appends a callback button; see [Callback] for typed callback data.
func (b *InlineKeyboardBuilder) Text(text, callbackData string) *InlineKeyboardBuilder {
	return b.Add(InlineKeyboardButton{Text: text, CallbackData: callbackData})
}

// URL appends a button that opens url.
func (b *InlineKeyboardBuilder) URL(text, url string) *InlineKeyboardBuilder {
	return b.Add(InlineKeyboardButton{Text: text, URL: url})
}

// WebApp appends a button that opens a Web App.
func (b *InlineKeyboardBuilder) WebApp(text, url string) *InlineKeyboardBuilder {
	return b.Add(InlineKeyboardButton{Text: text, WebApp: &WebAppInfo{URL: url}})
}

// SwitchInline appends a button that inserts the bot's username and query
// into the current chat's input field.
func (b *InlineKeyboardBuilder) SwitchInline(text, query string) *InlineKeyboardBuilder {
	return b.Add(InlineKeyboardButton{Text: text, SwitchInlineQueryCurrentChat: query})
}

// Row finishes the current row. An empty row is skipped.
func (b *InlineKeyboardBuilder) Row() *InlineKeyboardBuilder {
	if len(b.row) != 0 {
		b.rows = append(b.rows, b.row)
		b.row = nil
	}

	return b
}

// Rows returns the keyboard as rows of buttons, finishing the current row.
func (b *InlineKeyboardBuilder) Rows() [][]InlineKeyboardButton {
	b.Row()
	return b.rows
}

// Markup returns the keyboard as [InlineKeyboardMarkup].
func (b *InlineKeyboardBuilder) Markup() *InlineKeyboardMarkup {
	return &InlineKeyboardMarkup{InlineKeyboard: b.Rows()}
}

// ReplyMarkup returns the keyboard as [ReplyMarkup] for send methods.
func (b *InlineKeyboardBuilder) ReplyMarkup() *ReplyMarkup {
	return &ReplyMarkup{InlineKeyboardMarkup: b.Markup()}
}
