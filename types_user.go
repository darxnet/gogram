package gogram

import "strconv"

func (u *User) Identifier() string {
	if u.ID != 0 {
		return strconv.FormatInt(u.ID, 10)
	}

	return "@" + u.Username
}
