package gogram

// Standard bot commands.
const (
	// CommandStart is the default start command.
	CommandStart = "/start"
	// CommandHelp is the default help command.
	CommandHelp = "/help"
	// CommandSettings is the default settings command.
	CommandSettings = "/settings"
)

// RequiredCommands lists the commands that are typically required for a bot.
var RequiredCommands = [...]string{
	CommandStart,
	CommandHelp,
	CommandSettings,
}

// BusinessStartPayloadPrefix is the payload prefix used for business chat start links.
const BusinessStartPayloadPrefix = "bizChat" // bizChat<user_chat_id>
