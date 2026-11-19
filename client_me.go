package gogram

import (
	"context"
	"sync"
	"sync/atomic"
)

// botInfo caches GetMe: the bot's identity never changes while it runs.
type botInfo struct {
	me        atomic.Pointer[User]
	usernames atomic.Pointer[[]string] // active usernames from getChat, see usernames
	once      sync.Mutex               // serializes the first fetch; later calls are lock-free
}

// Me returns the bot's own [User] from getMe, fetched once and cached.
// A failed fetch is not cached: the next call retries.
func (c *Client) Me(ctx context.Context) (*User, error) {
	if me := c.info.me.Load(); me != nil {
		return me, nil
	}

	c.info.once.Lock()
	defer c.info.once.Unlock()

	if me := c.info.me.Load(); me != nil {
		return me, nil
	}

	me, err := c.GetMe(ctx, nil)
	if err != nil {
		return nil, err
	}

	c.info.me.Store(me)

	return me, nil
}

// usernames returns the bot's active usernames from getChat on its own id:
// the one from getMe plus any extra (collectible) ones, all of which
// address the bot in "/cmd@username". Fetched once and cached; a failed
// fetch is not cached.
func (c *Client) usernames(ctx context.Context, me *User) ([]string, error) {
	if names := c.info.usernames.Load(); names != nil {
		return *names, nil
	}

	c.info.once.Lock()
	defer c.info.once.Unlock()

	if names := c.info.usernames.Load(); names != nil {
		return *names, nil
	}

	chat, err := c.GetChat(ctx, &GetChatParams{ChatID: ChatID{ID: me.ID}})
	if err != nil {
		return nil, err
	}

	var names []string
	if chat != nil {
		names = chat.ActiveUsernames
	}

	c.info.usernames.Store(&names)

	return names, nil
}
