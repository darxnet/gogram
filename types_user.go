package gogram

import "strconv"

// Identifier returns the user identifier.
// It returns the user ID as a string if available, otherwise it returns the username with an '@' prefix.
func (u *User) Identifier() string {
	if u.ID != 0 {
		return strconv.FormatInt(u.ID, 10)
	}

	return "@" + u.Username
}
