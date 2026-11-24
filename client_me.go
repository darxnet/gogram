package gogram

import (
	"context"
	"sync"
	"sync/atomic"
)

// botInfo caches GetMe: the bot's identity never changes while it runs.
// Its settings can (SetMySettings), so Me is not the source for them.
type botInfo struct {
	me   atomic.Pointer[BotFullInfo]
	once sync.Mutex // serializes the first fetch; later calls are lock-free
}

// Me returns the bot's own [BotFullInfo] from getMe, fetched once and cached.
// A failed fetch is not cached: the next call retries.
//
// Use it for the identity (ID, Username, ActiveUsernames). The settings
// fields are as of the first fetch; call [Client.GetMe] for current ones.
func (c *Client) Me(ctx context.Context) (*BotFullInfo, error) {
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
