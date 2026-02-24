package gogram

import "strconv"

const (
	ChatPrivate    = "private"
	ChatGroup      = "group"
	ChatSupergroup = "supergroup"
	ChatChannel    = "channel"
)

func (c *Chat) IsPrivate() bool {
	return c.Type == ChatPrivate
}

func (c *Chat) IsGroup() bool {
	return c.Type == ChatGroup
}

func (c *Chat) IsSupergroup() bool {
	return c.Type == ChatSupergroup
}

func (c *Chat) IsChannel() bool {
	return c.Type == ChatChannel
}

func (c *Chat) Identifier() string {
	if c.ID != 0 {
		return strconv.FormatInt(c.ID, 10)
	}

	return "@" + c.Username
}
