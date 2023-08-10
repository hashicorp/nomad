// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"errors"
	"sync"

	"github.com/hashicorp/go-hclog"
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
	mu sync.Mutex

	// listeners is a map of unique ids to listener chans. lazily
	// initialized on first listen
	listeners map[int]chan *structs.Allocation

	// nextId is the next id to assign in listener map
	nextId int

	// closed is true if broadcaster is closed.
	closed bool

	// last alloc sent to prime new listeners
	last *structs.Allocation

	logger hclog.Logger
}

// NewAllocBroadcaster returns a new AllocBroadcaster.
func NewAllocBroadcaster(l hclog.Logger) *AllocBroadcaster {
	return &AllocBroadcaster{
		logger: l,
	}
}

// Send broadcasts an allocation update. Any pending updates are replaced with
// this version of the allocation to prevent blocking on slow receivers.
// Returns ErrAllocBroadcasterClosed if called after broadcaster is closed.
func (b *AllocBroadcaster) Send(v *structs.Allocation) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return ErrAllocBroadcasterClosed
	}

	b.logger.Trace("sending updated alloc",
		"client_status", v.ClientStatus,
		"desired_status", v.DesiredStatus,
	)

	// Store last sent alloc for future listeners
	b.last = v

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
// updates. Pending updates are still received by listeners. Safe to call
// concurrently and more than once.
func (b *AllocBroadcaster) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}

	// Close all listener chans
	for _, l := range b.listeners {
		close(l)
	}

	// Clear all references and mark broadcaster as closed
	b.listeners = nil
	b.closed = true
}

// stop an individual listener
func (b *AllocBroadcaster) stop(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()

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

// Listen returns a Listener for the broadcast channel. New listeners receive
// the last sent alloc update.
func (b *AllocBroadcaster) Listen() *AllocListener {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.listeners == nil {
		b.listeners = make(map[int]chan *structs.Allocation)
	}

	for b.listeners[b.nextId] != nil {
		b.nextId++
	}

	ch := make(chan *structs.Allocation, listenerCap)

	// Send last update if there was one
	if b.last != nil {
		ch <- b.last
	}

	// Broadcaster is already closed, close this listener. Must be done
	// after the last update was sent.
	if b.closed {
		close(ch)
	}

	b.listeners[b.nextId] = ch

	return &AllocListener{ch, b, b.nextId}
}

// AllocListener implements a listening endpoint for an allocation broadcast
// channel.
type AllocListener struct {
	// ch receives the broadcast messages.
	ch <-chan *structs.Allocation
	b  *AllocBroadcaster
	id int
}

func (l *AllocListener) Ch() <-chan *structs.Allocation {
	return l.ch
}

// Close closes the Listener, disabling the receival of further messages. Safe
// to call more than once and concurrently with receiving on Ch.
func (l *AllocListener) Close() {
	l.b.stop(l.id)
}
