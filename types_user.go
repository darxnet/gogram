package gogram

import "strconv"

// Target returns the value for a chat_id parameter: the user ID in decimal,
// or the username with an '@' prefix when the ID is unknown.
func (u *User) Target() string {
	if u.ID != 0 {
		return strconv.FormatInt(u.ID, 10)
	}

	return "@" + u.Username
}
