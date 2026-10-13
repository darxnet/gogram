package gogram

// Global commands Telegram requires every bot to support.
//
// Reference: https://core.telegram.org/bots/features#global-commands
const (
	// CommandStart begins the interaction with the user.
	CommandStart = "/start"
	// CommandHelp returns a help message.
	CommandHelp = "/help"
	// CommandSettings shows the bot's settings for this user, if applicable.
	CommandSettings = "/settings"
)

// RequiredCommands lists the global commands every bot must support.
var RequiredCommands = [...]string{
	CommandStart,
	CommandHelp,
	CommandSettings,
}

// BusinessStartPayloadPrefix is the /start payload prefix a bot receives when
// a business account owner taps "Manage Bot": "/start bizChat<user_chat_id>".
//
// Reference: https://core.telegram.org/bots/features#business-mode
const BusinessStartPayloadPrefix = "bizChat"
