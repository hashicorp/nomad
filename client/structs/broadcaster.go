package structs

import (
	"errors"
	"sync"

	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// listenerCap is the capacity of the listener chans. Must be exactly 1
	// to prevent Sends from blocking and allows them to pop old pending
	// updates from the chan before enqueueing the latest update.
	listenerCap = 1
)

var ErrAllocBroadcasterClosed = errors.New("alloc broadcaster closed")

// AllocBroadcaster implements an allocation broadcast channel where each
// listener receives allocation updates. Pending updates are dropped and
// replaced by newer allocation updates, so listeners may not receive every
// allocation update. However this ensures Sends never block and listeners only
// receive the latest allocation update -- never a stale version.
type AllocBroadcaster struct {
	m         sync.Mutex
	alloc     *structs.Allocation
	listeners map[int]chan *structs.Allocation // lazy init
	nextId    int
	closed    bool
}

// NewAllocBroadcaster returns a new AllocBroadcaster with the given initial
// allocation.
func NewAllocBroadcaster(initial *structs.Allocation) *AllocBroadcaster {
	return &AllocBroadcaster{
		alloc: initial,
	}
}

// Send broadcasts an allocation update. Any pending updates are replaced with
// this version of the allocation to prevent blocking on slow receivers.
// Returns ErrAllocBroadcasterClosed if called after broadcaster is closed.
func (b *AllocBroadcaster) Send(v *structs.Allocation) error {
	b.m.Lock()
	defer b.m.Unlock()
	if b.closed {
		return ErrAllocBroadcasterClosed
	}

	// Update alloc on broadcaster to send to newly created listeners
	b.alloc = v

	// Send alloc to already created listeners
	for _, l := range b.listeners {
		select {
		case l <- v:
		case <-l:
			// Pop pending update and replace with new update
			l <- v
		}
	}

	return nil
}

// Close closes the channel, disabling the sending of further allocation
// updates. Safe to call concurrently and more than once.
func (b *AllocBroadcaster) Close() {
	b.m.Lock()
	defer b.m.Unlock()
	if b.closed {
		return
	}

	// Close all listener chans
	for _, l := range b.listeners {
		close(l)
	}

	// Clear all references and mark broadcaster as closed
	b.listeners = nil
	b.alloc = nil
	b.closed = true
}

// stop an individual listener
func (b *AllocBroadcaster) stop(id int) {
	b.m.Lock()
	defer b.m.Unlock()

	// If broadcaster has been closed there's nothing more to do.
	if b.closed {
		return
	}

	l, ok := b.listeners[id]
	if !ok {
		// If this listener has been stopped already there's nothing
		// more to do.
		return
	}

	close(l)
	delete(b.listeners, id)
}

// Listen returns a Listener for the broadcast channel.
func (b *AllocBroadcaster) Listen() *AllocListener {
	b.m.Lock()
	defer b.m.Unlock()
	if b.listeners == nil {
		b.listeners = make(map[int]chan *structs.Allocation)
	}

	for b.listeners[b.nextId] != nil {
		b.nextId++
	}

	ch := make(chan *structs.Allocation, listenerCap)

	if b.closed {
		// Broadcaster is already closed, close this listener
		close(ch)
	} else {
		// Send the current allocation to the listener
		ch <- b.alloc
	}

	b.listeners[b.nextId] = ch

	return &AllocListener{ch, b, b.nextId}
}

// AllocListener implements a listening endpoint for an allocation broadcast
// channel.
type AllocListener struct {
	// Ch receives the broadcast messages.
	Ch <-chan *structs.Allocation
	b  *AllocBroadcaster
	id int
}

// Close closes the Listener, disabling the receival of further messages. Safe
// to call more than once and concurrently with receiving on Ch.
func (l *AllocListener) Close() {
	l.b.stop(l.id)
}
