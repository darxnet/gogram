package gogram

// // InlineKeyboardBuilder is a helper for building InlineKeyboardMarkup.
// type InlineKeyboardBuilder struct {
// 	keyboard   [][]InlineKeyboardButton
// 	currentRow []InlineKeyboardButton
// }

// // NewInlineKeyboard creates a new InlineKeyboardBuilder.
// func NewInlineKeyboard() *InlineKeyboardBuilder {
// 	return &InlineKeyboardBuilder{
// 		keyboard:   make([][]InlineKeyboardButton, 0),
// 		currentRow: make([]InlineKeyboardButton, 0),
// 	}
// }

// // AddButton adds a button with callback data to the current row.
// func (b *InlineKeyboardBuilder) AddButton(text, data string) *InlineKeyboardBuilder {
// 	b.currentRow = append(b.currentRow, InlineKeyboardButton{
// 		Text:         text,
// 		CallbackData: data,
// 	})
// 	return b
// }

// // AddURL adds a button with a URL to the current row.
// func (b *InlineKeyboardBuilder) AddURL(text, url string) *InlineKeyboardBuilder {
// 	b.currentRow = append(b.currentRow, InlineKeyboardButton{
// 		Text: text,
// 		URL:  url,
// 	})
// 	return b
// }

// // Row completes the current row and starts a new one.
// func (b *InlineKeyboardBuilder) Row() *InlineKeyboardBuilder {
// 	if len(b.currentRow) > 0 {
// 		b.keyboard = append(b.keyboard, b.currentRow)
// 		b.currentRow = make([]InlineKeyboardButton, 0)
// 	}
// 	return b
// }

// // Build constructs the InlineKeyboardMarkup.
// func (b *InlineKeyboardBuilder) Build() InlineKeyboardMarkup {
// 	// If there are buttons in the last row, add them
// 	if len(b.currentRow) > 0 {
// 		b.keyboard = append(b.keyboard, b.currentRow)
// 	}
// 	return InlineKeyboardMarkup{
// 		InlineKeyboard: b.keyboard,
// 	}
// }

// ///

// // PaginationItem represents an item in a paginated list.
// type PaginationItem struct {
// 	Text         string
// 	CallbackData string
// }

// // Paginate creates a pagination keyboard.
// // Note: This method is currently not implemented.
// func (b *InlineKeyboardBuilder) Paginate(
// 	items []PaginationItem,
// 	page int,
// 	limit int,
// 	navPrefix string,
// ) *InlineKeyboardBuilder {
// 	total := len(items)
// 	if total == 0 {
// 		return b // If list is empty, do nothing
// 	}

// 	return nil
// }
