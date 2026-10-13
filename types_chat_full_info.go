package gogram

import "strconv"

// Target returns the value for a chat_id parameter: the chat ID in decimal,
// or the username with an '@' prefix when the ID is unknown.
func (c *ChatFullInfo) Target() string {
	if c.ID != 0 {
		return strconv.FormatInt(c.ID, 10)
	}

	return "@" + c.Username
}
