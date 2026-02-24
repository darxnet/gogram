package gogram

import "strconv"

func (c *ChatFullInfo) Identifier() string {
	if c.ID != 0 {
		return strconv.FormatInt(c.ID, 10)
	}

	return "@" + c.Username
}
