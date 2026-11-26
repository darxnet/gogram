package gogram

import "slices"

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

// SwitchInlineCurrent appends a button that inserts the bot's username and
// query into the current chat's input field.
func (b *InlineKeyboardBuilder) SwitchInlineCurrent(text, query string) *InlineKeyboardBuilder {
	return b.Add(InlineKeyboardButton{Text: text, SwitchInlineQueryCurrentChat: Some(query)})
}

// SwitchInlineChat appends a button that prompts the user to pick a chat,
// opens it and inserts the bot's username and query into the input field.
func (b *InlineKeyboardBuilder) SwitchInlineChat(text, query string) *InlineKeyboardBuilder {
	return b.Add(InlineKeyboardButton{Text: text, SwitchInlineQuery: Some(query)})
}

// SwitchInlineChosenChat appends a button that prompts the user to pick a
// chat of the types allowed by chosen, opens it and inserts the bot's
// username and chosen.Query into the input field.
func (b *InlineKeyboardBuilder) SwitchInlineChosenChat(
	text string,
	chosen SwitchInlineQueryChosenChat,
) *InlineKeyboardBuilder {
	return b.Add(InlineKeyboardButton{Text: text, SwitchInlineQueryChosenChat: &chosen})
}

// LoginURL appends a button that authorizes the user on login.URL, a
// replacement for the Telegram Login Widget.
func (b *InlineKeyboardBuilder) LoginURL(text string, login LoginUrl) *InlineKeyboardBuilder {
	return b.Add(InlineKeyboardButton{Text: text, LoginUrl: &login})
}

// CopyText appends a button that copies clipboard to the clipboard.
func (b *InlineKeyboardBuilder) CopyText(text, clipboard string) *InlineKeyboardBuilder {
	return b.Add(InlineKeyboardButton{Text: text, CopyText: &CopyTextButton{Text: clipboard}})
}

// Pay appends a Pay button. "⭐" and "XTR" in text are shown as a Telegram
// Star icon. The amount comes from the invoice, not from the button.
// Telegram accepts it only in invoice messages, as the first button of the
// first row.
func (b *InlineKeyboardBuilder) Pay(text string) *InlineKeyboardBuilder {
	return b.Add(InlineKeyboardButton{Text: text, Pay: true})
}

// Game appends a button that launches the game of a game message.
// Telegram accepts it only as the first button of the first row.
func (b *InlineKeyboardBuilder) Game(text string) *InlineKeyboardBuilder {
	return b.Add(InlineKeyboardButton{Text: text, CallbackGame: &CallbackGame{}})
}

// Disabled appends a button that does nothing when pressed: a label such
// as a page counter or a placeholder cell.
func (b *InlineKeyboardBuilder) Disabled(text string) *InlineKeyboardBuilder {
	return b.Add(InlineKeyboardButton{Text: text, Disabled: &DisabledButton{}})
}

// Button styles for [InlineKeyboardBuilder.Style].
const (
	ButtonStyleDanger  = "danger"  // red
	ButtonStyleSuccess = "success" // green
	ButtonStylePrimary = "primary" // blue
)

// Style sets the style of the last added button, one of the ButtonStyle
// constants. It panics when no button has been added.
func (b *InlineKeyboardBuilder) Style(style string) *InlineKeyboardBuilder {
	b.last("Style").Style = style
	return b
}

// Icon sets the custom emoji shown before the text of the last added
// button. It panics when no button has been added.
func (b *InlineKeyboardBuilder) Icon(customEmojiID string) *InlineKeyboardBuilder {
	b.last("Icon").IconCustomEmojiID = customEmojiID
	return b
}

// last returns the last added button.
func (b *InlineKeyboardBuilder) last(method string) *InlineKeyboardButton {
	if n := len(b.row); n != 0 {
		return &b.row[n-1]
	}

	if n := len(b.rows); n != 0 {
		row := b.rows[n-1]
		return &row[len(row)-1]
	}

	panic("gogram: InlineKeyboardBuilder." + method + " called before any button was added")
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

// AddRows finishes the current row and appends copies of rows, so later
// changes through the builder (Style, Icon) do not touch the caller's
// slices. Empty rows are skipped.
func (b *InlineKeyboardBuilder) AddRows(rows ...InlineKeyboardRow) *InlineKeyboardBuilder {
	b.Row()

	for _, row := range rows {
		if len(row) != 0 {
			b.rows = append(b.rows, slices.Clone(row))
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
