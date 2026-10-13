package gogram

import (
	"context"
	"errors"
)

// Start starts the client and listens for updates using long polling.
// It returns nil when ctx is cancelled or Stop is called, the context error
// on deadline, and the API error when the bot is unauthorized or banned.
func (c *Client) Start(ctx context.Context, params *GetUpdatesParams) error {
	innerCtx, state, err := c.beginRun(ctx)
	if err != nil {
		return err
	}
	defer c.finishRun(state)

	var local GetUpdatesParams
	if params != nil {
		local = *params
	}

	if local.Limit <= 0 || local.Limit > defaultUpdates {
		local.Limit = defaultUpdates
	}

	if local.Timeout <= 0 {
		local.Timeout = int64(defaultTimeout.Seconds())
	}

	numWorkers := c.workerCount(local.Limit)
	pool := c.startWorkers(numWorkers, max(local.Limit/numWorkers, 1))

	// The caller's goroutine polls; the workers drain after it returns.
	err = c.poll(innerCtx, &local, pool)

	pool.stop()

	// Stop and a cancelled ctx are a normal shutdown; a deadline and
	// fatal API errors are reported.
	if errors.Is(err, context.Canceled) {
		return nil
	}

	return err
}
