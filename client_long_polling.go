package gogram

import (
	"context"
	"errors"
	"sync"
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

	numWorkers := local.Limit
	if c.cfg.numWorkers > 0 {
		numWorkers = int64(c.cfg.numWorkers)
	}

	workers := make([]chan *Context, numWorkers)
	for i := range workers {
		workers[i] = make(chan *Context, max(local.Limit/numWorkers, 1))
	}

	var wg sync.WaitGroup
	for _, ch := range workers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for gogramCtx := range ch {
				c.processUpdate(gogramCtx)
			}
		}()
	}

	// The caller's goroutine polls; no extra goroutine, no close-on-defer dance.
	err = c.poll(innerCtx, &local, workers)

	for _, ch := range workers {
		close(ch)
	}
	wg.Wait()

	// Stop and a cancelled ctx are a normal shutdown; a deadline and
	// fatal API errors are reported.
	if errors.Is(err, context.Canceled) {
		return nil
	}

	return err
}
