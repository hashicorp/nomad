package structs

import (
	"sync"

	"github.com/hashicorp/nomad/nomad/structs"
)

// AllocBroadcaster implements an allocation broadcast channel.
// The zero value is a usable unbuffered channel.
type AllocBroadcaster struct {
	m         sync.Mutex
	listeners map[int]chan<- *structs.Allocation // lazy init
	nextId    int
	capacity  int
	closed    bool
}

// NewAllocBroadcaster returns a new AllocBroadcaster with the given capacity (0 means unbuffered).
func NewAllocBroadcaster(n int) *AllocBroadcaster {
	return &AllocBroadcaster{capacity: n}
}

// AllocListener implements a listening endpoint for an allocation broadcast channel.
type AllocListener struct {
	// Ch receives the broadcast messages.
	Ch <-chan *structs.Allocation
	b  *AllocBroadcaster
	id int
}

// Send broadcasts a message to the channel. Send returns whether the message
// was sent to all channels.
func (b *AllocBroadcaster) Send(v *structs.Allocation) bool {
	b.m.Lock()
	defer b.m.Unlock()
	if b.closed {
		return false
	}
	sent := true
	for _, l := range b.listeners {
		select {
		case l <- v:
		default:
			sent = false
		}
	}

	return sent
}

// Close closes the channel, disabling the sending of further messages.
func (b *AllocBroadcaster) Close() {
	b.m.Lock()
	defer b.m.Unlock()
	if b.closed {
		return
	}

	b.closed = true
	for _, l := range b.listeners {
		close(l)
	}
}

// Listen returns a Listener for the broadcast channel.
func (b *AllocBroadcaster) Listen() *AllocListener {
	b.m.Lock()
	defer b.m.Unlock()
	if b.listeners == nil {
		b.listeners = make(map[int]chan<- *structs.Allocation)
	}
	for b.listeners[b.nextId] != nil {
		b.nextId++
	}
	ch := make(chan *structs.Allocation, b.capacity)
	if b.closed {
		close(ch)
	}
	b.listeners[b.nextId] = ch
	return &AllocListener{ch, b, b.nextId}
}

// Close closes the Listener, disabling the receival of further messages.
func (l *AllocListener) Close() {
	l.b.m.Lock()
	defer l.b.m.Unlock()
	delete(l.b.listeners, l.id)
}
