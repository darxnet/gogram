package gogram

// Target returns the value for a chat_id parameter.
func (u *User) Target() ChatID {
	if u.ID != 0 {
		return ChatID{ID: u.ID}
	}

	return ChatID{Username: u.Username}
}
