package gogram

// Limits documented by Telegram.
//
// References:
//   - https://core.telegram.org/bots/features#commands
//   - https://core.telegram.org/bots/features#deep-linking
//   - https://core.telegram.org/bots/api
const (
	// CommandMaxLen is the maximum command length accepted by Telegram.
	CommandMaxLen = 32
	// StartParamMaxLen is the maximum deep-link start parameter length
	// (allowed characters: A-Z, a-z, 0-9, _ and -).
	StartParamMaxLen = 64
	// CallbackDataMaxLen is the maximum InlineKeyboardButton.CallbackData size in bytes.
	CallbackDataMaxLen = 64

	// MessageMaxLen is the maximum text message length.
	MessageMaxLen = 4096
	// CaptionMaxLen is the maximum media caption length.
	CaptionMaxLen = 1024
	// MediaGroupMaxLen is the maximum number of media items in an album.
	MediaGroupMaxLen = 10
)
