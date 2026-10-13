package gogram

// Chat types.
const (
	// ChatPrivate represents a private chat.
	ChatPrivate = "private"
	// ChatGroup represents a basic group chat.
	ChatGroup = "group"
	// ChatSupergroup represents a supergroup chat.
	ChatSupergroup = "supergroup"
	// ChatChannel represents a channel chat.
	ChatChannel = "channel"
)

// IsPrivate returns true if the chat is a private chat.
func (c *Chat) IsPrivate() bool {
	return c.Type == ChatPrivate
}

// IsGroup returns true if the chat is a group.
func (c *Chat) IsGroup() bool {
	return c.Type == ChatGroup
}

// IsSupergroup returns true if the chat is a supergroup.
func (c *Chat) IsSupergroup() bool {
	return c.Type == ChatSupergroup
}

// IsChannel returns true if the chat is a channel.
func (c *Chat) IsChannel() bool {
	return c.Type == ChatChannel
}

// Target returns the value for a chat_id parameter.
func (c *Chat) Target() ChatID {
	if c.ID != 0 {
		return ChatID{ID: c.ID}
	}

	return ChatID{Username: c.Username}
}
