// Package joincontext provides a way to combine two contexts.
// For example it might be useful for grpc server to cancel all handlers in
// addition to provided handler context.
package joincontext

import (
	"sync"
	"time"

	"golang.org/x/net/context"
)

type joinContext struct {
	mu   sync.Mutex
	ctx1 context.Context
	ctx2 context.Context
	done chan struct{}
	err  error
}

// Join returns new context which is child for two passed contexts.
// It starts new goroutine which tracks both contexts.
//
// Done() channel is closed when one of parents contexts is done.
//
// Deadline() returns earliest deadline between parent contexts.
//
// Err() returns error from first done parent context.
//
// Value(key) looks for key in parent contexts. First found is returned.
func Join(ctx1, ctx2 context.Context) (context.Context, context.CancelFunc) {
	c := &joinContext{ctx1: ctx1, ctx2: ctx2, done: make(chan struct{})}
	go c.run()
	return c, c.cancel
}

func (c *joinContext) Deadline() (deadline time.Time, ok bool) {
	d1, ok1 := c.ctx1.Deadline()
	if !ok1 {
		return c.ctx2.Deadline()
	}
	d2, ok2 := c.ctx2.Deadline()
	if !ok2 {
		return d1, true
	}

	if d2.Before(d1) {
		return d2, true
	}
	return d1, true
}

func (c *joinContext) Done() <-chan struct{} {
	return c.done
}

func (c *joinContext) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *joinContext) Value(key interface{}) interface{} {
	v := c.ctx1.Value(key)
	if v == nil {
		v = c.ctx2.Value(key)
	}
	return v
}

func (c *joinContext) run() {
	var doneCtx context.Context
	select {
	case <-c.ctx1.Done():
		doneCtx = c.ctx1
	case <-c.ctx2.Done():
		doneCtx = c.ctx2
	case <-c.done:
		return
	}

	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return
	}
	c.err = doneCtx.Err()
	c.mu.Unlock()
	close(c.done)
}

func (c *joinContext) cancel() {
	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return
	}
	c.err = context.Canceled

	c.mu.Unlock()
	close(c.done)
}
