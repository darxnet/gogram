package gogram

type InlineKeyboardBuilder struct {
	keyboard   [][]InlineKeyboardButton
	currentRow []InlineKeyboardButton
}

func NewInlineKeyboard() *InlineKeyboardBuilder {
	return &InlineKeyboardBuilder{
		keyboard:   make([][]InlineKeyboardButton, 0),
		currentRow: make([]InlineKeyboardButton, 0),
	}
}

func (b *InlineKeyboardBuilder) AddButton(text, data string) *InlineKeyboardBuilder {
	b.currentRow = append(b.currentRow, InlineKeyboardButton{
		Text:         text,
		CallbackData: data,
	})
	return b
}

func (b *InlineKeyboardBuilder) AddURL(text, url string) *InlineKeyboardBuilder {
	b.currentRow = append(b.currentRow, InlineKeyboardButton{
		Text: text,
		URL:  url,
	})
	return b
}

func (b *InlineKeyboardBuilder) Row() *InlineKeyboardBuilder {
	if len(b.currentRow) > 0 {
		b.keyboard = append(b.keyboard, b.currentRow)
		b.currentRow = make([]InlineKeyboardButton, 0)
	}
	return b
}

func (b *InlineKeyboardBuilder) Build() InlineKeyboardMarkup {
	// Якщо в останньому ряду залишилися кнопки, додаємо їх
	if len(b.currentRow) > 0 {
		b.keyboard = append(b.keyboard, b.currentRow)
	}
	return InlineKeyboardMarkup{
		InlineKeyboard: b.keyboard,
	}
}

///

type PaginationItem struct {
	Text         string
	CallbackData string
}

func (b *InlineKeyboardBuilder) Paginate(
	items []PaginationItem,
	page int,
	limit int,
	navPrefix string,
) *InlineKeyboardBuilder {
	total := len(items)
	if total == 0 {
		return b // Якщо список пустий, нічого не робимо
	}

	return nil
}
