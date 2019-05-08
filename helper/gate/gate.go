// Package gate implements a simple on/off latch or gate: it blocks waiters
// until opened. Waiters may receive on a chan which is closed when the gate is
// open.
package gate

import "sync"

// closedCh is a chan initialized as closed
var closedCh chan struct{}

func init() {
	closedCh = make(chan struct{})
	close(closedCh)
}

// G is a gate which blocks waiters until opened and is safe for concurrent
// use. Must be created via New.
type G struct {
	// open is true if the gate is open and ch is closed.
	open bool

	// ch is closed if the gate is open.
	ch chan struct{}

	mu sync.Mutex
}

// NewClosed returns a closed gate. The chan returned by Wait will block until Open
// is called.
func NewClosed() *G {
	return &G{
		ch: make(chan struct{}),
	}
}

// NewOpen returns an open gate. The chan returned by Wait is closed and
// therefore will never block.
func NewOpen() *G {
	return &G{
		open: true,
		ch:   closedCh,
	}
}

// Open the gate. Unblocks any Waiters. Opening an opened gate is a noop. Safe
// for concurrent ues with Close and Wait.
func (g *G) Open() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.open {
		return
	}

	g.open = true
	close(g.ch)
}

// Close the gate. Blocks subsequent Wait callers. Closing a closed gate is a
// noop. Safe for concurrent use with Open and Wait.
func (g *G) Close() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.open {
		return
	}

	g.open = false
	g.ch = make(chan struct{})
}

// Wait returns a chan that blocks until the gate is open. Safe for concurrent
// use with Open and Close, but the chan should not be reused between calls to
// Open and Close.
func (g *G) Wait() <-chan struct{} {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.ch
}

// IsClosed returns true if the gate is closed.
func (g *G) IsClosed() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return !g.open
}
