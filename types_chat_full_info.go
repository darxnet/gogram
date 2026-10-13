package gogram

// Target returns the value for a chat_id parameter.
func (c *ChatFullInfo) Target() ChatID {
	if c.ID != 0 {
		return ChatID{ID: c.ID}
	}

	return ChatID{Username: c.Username}
}
