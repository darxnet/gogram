package gogram

import "strconv"

// Identifier returns the chat identifier.
// It returns the chat ID as a string if available, otherwise it returns the username with an '@' prefix.
func (c *ChatFullInfo) Identifier() string {
	if c.ID != 0 {
		return strconv.FormatInt(c.ID, 10)
	}

	return "@" + c.Username
}
