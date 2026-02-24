package gogram

const (
	MessageEntityMention              = "mention"               // @username
	MessageEntityHashtag              = "hashtag"               // #hashtag or #hashtag@chatusername
	MessageEntityCashtag              = "cashtag"               // $USD or $USD@chatusername
	MessageEntityBotCommand           = "bot_command"           // /start@jobs_bot
	MessageEntityBotURL               = "url"                   // https://telegram.org
	MessageEntityEmail                = "email"                 // do-not-reply@telegram.org
	MessageEntityPhoneNumber          = "phone_number"          // +1-212-555-0123
	MessageEntityBold                 = "bold"                  // bold text
	MessageEntityItalic               = "italic"                // italic text
	MessageEntityUnderline            = "underline"             // underlined text
	MessageEntityStrikethrough        = "strikethrough"         // strikethrough text
	MessageEntitySpoiler              = "spoiler"               // spoiler message
	MessageEntityBlockquote           = "blockquote"            // block quotation
	MessageEntityExpandableBlockquote = "expandable_blockquote" // collapsed-by-default block quotation
	MessageEntityCode                 = "code"                  // monowidth string
	MessageEntityPre                  = "pre"                   // mmonowidth block
	MessageEntityTextLink             = "text_link"             // for clickable text URLs
	MessageEntityTextMention          = "text_mention"          // for users [without usernames]
	MessageEntityCustomEmoji          = "custom_emoji"          // for inline custom emoji stickers
)
