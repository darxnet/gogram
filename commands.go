package gogram

const (
	CommandStart    = "/start"
	CommandHelp     = "/help"
	CommandSettings = "/settings"
)

var RequiredCommands = [...]string{
	CommandStart,
	CommandHelp,
	CommandSettings,
}

const BusinessStartPayloadPrefix = "bizChat" // bizChat<user_chat_id>
