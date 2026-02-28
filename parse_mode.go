package gogram

// Parse modes for text formatting.
const (
	// ParseModeHTML enables Telegram HTML formatting.
	ParseModeHTML = "HTML" // https://core.telegram.org/bots/api#html-style
	// ParseModeMarkdown enables legacy Telegram Markdown formatting.
	ParseModeMarkdown = "Markdown" // https://core.telegram.org/bots/api#markdown-style
	// ParseModeMarkdownV2 enables Telegram MarkdownV2 formatting.
	ParseModeMarkdownV2 = "MarkdownV2" // https://core.telegram.org/bots/api#markdownv2-style
)
