package gogram

import (
	"context"
	"errors"
	"sync"
)

// Start starts the client and listens for updates using long polling.
func (c *Client) Start(ctx context.Context, params *GetUpdatesParams) error {
	innerCtx, state, err := c.beginRun(ctx)
	if err != nil {
		return err
	}
	defer c.finishRun(state)

	var localParams GetUpdatesParams

	if params != nil {
		localParams = *params
	}

	if localParams.Limit == 0 {
		localParams.Limit = defaultUpdates
	}

	numWorkers := localParams.Limit
	if c.cfg.numWorkers > 0 {
		numWorkers = int64(c.cfg.numWorkers)
	}

	workerChans := make([]chan *Context, numWorkers)
	for i := range numWorkers {
		workerChans[i] = make(chan *Context, max(localParams.Limit/numWorkers, 1))
	}

	var wg sync.WaitGroup

	wg.Go(func() {
		defer func() {
			for _, ch := range workerChans {
				close(ch)
			}
		}()
		c.startPolling(innerCtx, &localParams, workerChans)
	})

	for i := range numWorkers {
		ch := workerChans[i]
		wg.Go(func() {
			for ctx := range ch {
				c.processUpdate(ctx)
			}
		})
	}

	wg.Wait()

	if err = ctx.Err(); !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}
