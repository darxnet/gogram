package gogram

// InlineKeyboardRow is a row of inline keyboard buttons.
type InlineKeyboardRow = []InlineKeyboardButton

// InlineKeyboard is an inline keyboard: rows of buttons, the type of
// [InlineKeyboardMarkup].InlineKeyboard.
type InlineKeyboard = []InlineKeyboardRow

// InlineKeyboardBuilder builds an [InlineKeyboardMarkup] row by row:
//
//	markup := gogram.NewInlineKeyboardBuilder().
//		Callback("Yes", "confirm 1").Callback("No", "confirm 0").Row().
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

// Callback appends a callback button; see [Callback] for typed callback data.
func (b *InlineKeyboardBuilder) Callback(text, callbackData string) *InlineKeyboardBuilder {
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

// AddColumn finishes the current row and adds each button as a row of its
// own, so the buttons form a single column.
func (b *InlineKeyboardBuilder) AddColumn(buttons ...InlineKeyboardButton) *InlineKeyboardBuilder {
	b.Row()

	for i := range buttons {
		b.rows = append(b.rows, InlineKeyboardRow{buttons[i]})
	}

	return b
}

// AddRows finishes the current row and appends rows. Empty rows are skipped.
func (b *InlineKeyboardBuilder) AddRows(rows ...InlineKeyboardRow) *InlineKeyboardBuilder {
	b.Row()

	for _, row := range rows {
		if len(row) != 0 {
			b.rows = append(b.rows, row)
		}
	}

	return b
}

// AddKeyboards finishes the current row and appends the rows of each
// keyboard in order. Empty rows are skipped.
func (b *InlineKeyboardBuilder) AddKeyboards(keyboards ...InlineKeyboard) *InlineKeyboardBuilder {
	for _, kb := range keyboards {
		b.AddRows(kb...)
	}

	return b.Row()
}

// Row starts a new row: buttons added next go to it. Calling Row on an
// empty row does nothing, so no empty rows are produced.
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
