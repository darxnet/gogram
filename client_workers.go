package gogram

import (
	"context"
	"sync"
)

// workerPool processes updates on a fixed set of goroutines. An update is
// routed by chat id (or update id when there is no chat), so handlers for
// one chat run in order while different chats run in parallel.
type workerPool struct {
	queues []chan *Context
	wg     sync.WaitGroup
}

// startWorkers starts n workers with a queue of depth queue each.
func (c *Client) startWorkers(n, queue int64) *workerPool {
	p := &workerPool{queues: make([]chan *Context, n)}

	for i := range p.queues {
		ch := make(chan *Context, queue)
		p.queues[i] = ch

		p.wg.Add(1)

		go func() {
			defer p.wg.Done()

			for gogramCtx := range ch {
				c.processUpdate(gogramCtx)
			}
		}()
	}

	return p
}

// dispatch queues gogramCtx on the worker of its chat. It blocks while the
// queue is full (back-pressure) and releases the context when ctx is done
// first, reporting false.
func (p *workerPool) dispatch(ctx context.Context, gogramCtx *Context) bool {
	key := gogramCtx.update.UpdateID
	if chat := gogramCtx.Chat(); chat != nil {
		key = chat.ID
	}

	idx := key % int64(len(p.queues))
	if idx < 0 {
		idx = -idx
	}

	select {
	case p.queues[idx] <- gogramCtx:
		return true
	case <-ctx.Done():
		gogramCtx.client.releaseContext(gogramCtx)
		return false
	}
}

// stop closes the queues and waits for the workers to drain them.
func (p *workerPool) stop() {
	for _, ch := range p.queues {
		close(ch)
	}

	p.wg.Wait()
}

// workerCount returns the configured number of workers, or fallback.
func (c *Client) workerCount(fallback int64) int64 {
	if c.cfg.numWorkers > 0 {
		return int64(c.cfg.numWorkers)
	}

	return fallback
}
