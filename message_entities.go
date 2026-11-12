package gogram

// Message entity types.
const (
	// MessageEntityMention represents a mention of a user (e.g., @username).
	MessageEntityMention = "mention"
	// MessageEntityHashtag represents a hashtag (e.g., #hashtag).
	MessageEntityHashtag = "hashtag"
	// MessageEntityCashtag represents a cashtag (e.g., $USD).
	MessageEntityCashtag = "cashtag"
	// MessageEntityBotCommand represents a bot command (e.g., /start).
	MessageEntityBotCommand = "bot_command"
	// MessageEntityBotURL represents a URL (e.g., https://telegram.org).
	MessageEntityBotURL = "url"
	// MessageEntityEmail represents an email address (e.g., do-not-reply@telegram.org).
	MessageEntityEmail = "email"
	// MessageEntityPhoneNumber represents a phone number (e.g., +1-212-555-0123).
	MessageEntityPhoneNumber = "phone_number"
	// MessageEntityBankCardNumber represents a bank card number.
	MessageEntityBankCardNumber = "bank_card_number"
	// MessageEntityTonAddress represents an address in the TON blockchain.
	MessageEntityTonAddress = "ton_address"
	// MessageEntityBold represents bold text.
	MessageEntityBold = "bold"
	// MessageEntityItalic represents italic text.
	MessageEntityItalic = "italic"
	// MessageEntityUnderline represents underlined text.
	MessageEntityUnderline = "underline"
	// MessageEntityStrikethrough represents strikethrough text.
	MessageEntityStrikethrough = "strikethrough"
	// MessageEntitySpoiler represents spoiler message.
	MessageEntitySpoiler = "spoiler"
	// MessageEntityBlockquote represents a block quotation.
	MessageEntityBlockquote = "blockquote"
	// MessageEntityExpandableBlockquote represents a collapsed-by-default block quotation.
	MessageEntityExpandableBlockquote = "expandable_blockquote"
	// MessageEntityCode represents a monowidth string.
	MessageEntityCode = "code"
	// MessageEntityPre represents a monowidth block.
	MessageEntityPre = "pre"
	// MessageEntityTextLink represents a clickable text URL.
	MessageEntityTextLink = "text_link"
	// MessageEntityTextMention represents a mention of a user without a username.
	MessageEntityTextMention = "text_mention"
	// MessageEntityCustomEmoji represents an inline custom emoji sticker.
	MessageEntityCustomEmoji = "custom_emoji"
	// MessageEntityDateTime represents a formatted date and time.
	MessageEntityDateTime = "date_time"
)
