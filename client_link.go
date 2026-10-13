package gogram

import (
	"context"
	"errors"
)

// ErrInvalidStartParam indicates a deep-link parameter outside
// 1–64 characters of A-Z, a-z, 0-9, _ and -.
var ErrInvalidStartParam = errors.New("gogram: invalid start parameter")

// StartLink returns a deep link that opens a private chat with the bot and
// sends "/start param": https://t.me/<bot>?start=<param>. An empty param
// gives the plain bot link. The bot's username comes from [Client.Me].
//
// Reference: https://core.telegram.org/bots/features#deep-linking
func (c *Client) StartLink(ctx context.Context, param string) (string, error) {
	return c.deepLink(ctx, "?start=", param)
}

// StartGroupLink returns a deep link that prompts the user to add the bot
// to a group and sends "/start param" there: https://t.me/<bot>?startgroup=<param>.
func (c *Client) StartGroupLink(ctx context.Context, param string) (string, error) {
	return c.deepLink(ctx, "?startgroup=", param)
}

func (c *Client) deepLink(ctx context.Context, query, param string) (string, error) {
	if !ValidStartParam(param) && param != "" {
		return "", ErrInvalidStartParam
	}

	me, err := c.Me(ctx)
	if err != nil {
		return "", err
	}

	link := "https://t.me/" + me.Username
	if param != "" {
		link += query + param
	}

	return link, nil
}

// ValidStartParam reports whether param is a valid deep-link parameter:
// 1–64 characters of A-Z, a-z, 0-9, _ and -.
func ValidStartParam(param string) bool {
	if param == "" || len(param) > StartParamMaxLen {
		return false
	}

	for i := range len(param) {
		switch ch := param[i]; {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch >= '0' && ch <= '9', ch == '_', ch == '-':
		default:
			return false
		}
	}

	return true
}
