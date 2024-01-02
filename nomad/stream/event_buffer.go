// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package stream

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

// eventBuffer is a single-writer, multiple-reader, fixed length concurrent
// buffer of events that have been published. The buffer is
// the head and tail of an atomically updated single-linked list. Atomic
// accesses are usually to be suspected as premature optimization but this
// specific design has several important features that significantly simplify a
// lot of our PubSub machinery.
//
// eventBuffer is an adaptation of conuls agent/stream/event eventBuffer but
// has been updated to be a max length buffer to work for Nomad's usecase.
//
// The eventBuffer only tracks the most recent set of published events,
// up to the max configured size, older events are dropped from the buffer
// but will only be garbage collected once the slowest reader drops the item.
// Consumers are notified of new events by closing a channel on the previous head
// allowing efficient broadcast to many watchers without having to run multiple
// goroutines or deliver to O(N) separate channels.
//
// Because eventBuffer is a linked list with atomically updated pointers, readers don't
// have to take a lock and can consume at their own pace. Slow readers will eventually
// be forced to reconnect to the lastest head by being notified via a bufferItem's droppedCh.
//
// A new buffer is constructed with a sentinel "empty" bufferItem that has a nil
// Events array. This enables subscribers to start watching for the next update
// immediately.
//
// The zero value eventBuffer is _not_ usable, as it has not been
// initialized with an empty bufferItem so can not be used to wait for the first
// published event. Call newEventBuffer to construct a new buffer.
//
// Calls to Append or purne that mutate the head must be externally
// synchronized. This allows systems that already serialize writes to append
// without lock overhead.
type eventBuffer struct {
	size *int64

	head atomic.Value
	tail atomic.Value

	maxSize int64
}

// newEventBuffer creates an eventBuffer ready for use.
func newEventBuffer(size int64) *eventBuffer {
	zero := int64(0)
	b := &eventBuffer{
		maxSize: size,
		size:    &zero,
	}

	item := newBufferItem(&structs.Events{Index: 0, Events: nil})

	b.head.Store(item)
	b.tail.Store(item)

	return b
}

// Append a set of events from one raft operation to the buffer and notify
// watchers. After calling append, the caller must not make any further
// mutations to the events as they may have been exposed to subscribers in other
// goroutines. Append only supports a single concurrent caller and must be
// externally synchronized with other Append calls.
func (b *eventBuffer) Append(events *structs.Events) {
	b.appendItem(newBufferItem(events))
}

func (b *eventBuffer) appendItem(item *bufferItem) {
	// Store the next item to the old tail
	oldTail := b.Tail()
	oldTail.link.next.Store(item)

	// Update the tail to the new item
	b.tail.Store(item)

	// Increment the buffer size
	atomic.AddInt64(b.size, 1)

	// Advance Head until we are under allowable size
	for atomic.LoadInt64(b.size) > b.maxSize {
		b.advanceHead()
	}

	// notify waiters next event is available
	close(oldTail.link.nextCh)
}

func newSentinelItem() *bufferItem {
	return newBufferItem(&structs.Events{})
}

// advanceHead drops the current Head buffer item and notifies readers
// that the item should be discarded by closing droppedCh.
// Slow readers will prevent the old head from being GC'd until they
// discard it.
func (b *eventBuffer) advanceHead() {
	old := b.Head()

	next := old.link.next.Load()
	// if the next item is nil replace it with a sentinel value
	if next == nil {
		next = newSentinelItem()
	}

	// notify readers that old is being dropped
	close(old.link.droppedCh)

	// store the next value to head
	b.head.Store(next)

	// If the old head is equal to the tail
	// update the tail value as well
	if old == b.Tail() {
		b.tail.Store(next)
	}

	// In the case of there being a sentinel item or advanceHead being called
	// on a sentinel item, only decrement if there are more than sentinel
	// values
	if atomic.LoadInt64(b.size) > 0 {
		// update the amount of events we have in the buffer
		atomic.AddInt64(b.size, -1)
	}
}

// Head returns the current head of the buffer. It will always exist but it may
// be a "sentinel" empty item with a nil Events slice to allow consumers to
// watch for the next update. Consumers should always check for empty Events and
// treat them as no-ops. Will panic if eventBuffer was not initialized correctly
// with NewEventBuffer
func (b *eventBuffer) Head() *bufferItem {
	return b.head.Load().(*bufferItem)
}

// Tail returns the current tail of the buffer. It will always exist but it may
// be a "sentinel" empty item with a Nil Events slice to allow consumers to
// watch for the next update. Consumers should always check for empty Events and
// treat them as no-ops. Will panic if eventBuffer was not initialized correctly
// with NewEventBuffer
func (b *eventBuffer) Tail() *bufferItem {
	return b.tail.Load().(*bufferItem)
}

// StarStartAtClosest returns the closest bufferItem to a requested starting
// index as well as the offset between the requested index and returned one.
func (b *eventBuffer) StartAtClosest(index uint64) (*bufferItem, int) {
	item := b.Head()
	if index < item.Events.Index {
		return item, int(item.Events.Index) - int(index)
	}
	if item.Events.Index == index {
		return item, 0
	}

	for {
		prev := item
		item = item.NextNoBlock()
		if item == nil {
			return prev, int(index) - int(prev.Events.Index)
		}
		if index < item.Events.Index {
			return item, int(item.Events.Index) - int(index)
		}
		if index == item.Events.Index {
			return item, 0
		}
	}
}

// Len returns the current length of the buffer
func (b *eventBuffer) Len() int {
	return int(atomic.LoadInt64(b.size))
}

// bufferItem represents a set of events published by a single raft operation.
// The first item returned by a newly constructed buffer will have nil Events.
// It is a sentinel value which is used to wait on the next events via Next.
//
// To iterate to the next event, a Next method may be called which may block if
// there is no next element yet.
//
// Holding a pointer to the item keeps all the events published since in memory
// so it's important that subscribers don't hold pointers to buffer items after
// they have been delivered except where it's intentional to maintain a cache or
// trailing store of events for performance reasons.
//
// Subscribers must not mutate the bufferItem or the Events or Encoded payloads
// inside as these are shared between all readers.
type bufferItem struct {
	// Events is the set of events published at one raft index. This may be nil as
	// a sentinel value to allow watching for the first event in a buffer. Callers
	// should check and skip nil Events at any point in the buffer. It will also
	// be nil if the producer appends an Error event because they can't complete
	// the request to populate the buffer. Err will be non-nil in this case.
	Events *structs.Events

	// Err is non-nil if the producer can't complete their task and terminates the
	// buffer. Subscribers should return the error to clients and cease attempting
	// to read from the buffer.
	Err error

	// link holds the next pointer and channel. This extra bit of indirection
	// allows us to splice buffers together at arbitrary points without including
	// events in one buffer just for the side-effect of watching for the next set.
	// The link may not be mutated once the event is appended to a buffer.
	link *bufferLink

	createdAt time.Time
}

type bufferLink struct {
	// next is an atomically updated pointer to the next event in the buffer. It
	// is written exactly once by the single published and will always be set if
	// ch is closed.
	next atomic.Value

	// nextCh is closed when the next event is published. It should never be mutated
	// (e.g. set to nil) as that is racey, but is closed once when the next event
	// is published. the next pointer will have been set by the time this is
	// closed.
	nextCh chan struct{}

	// droppedCh is closed when the event is dropped from the buffer due to
	// sizing constraints.
	droppedCh chan struct{}
}

// newBufferItem returns a blank buffer item with a link and chan ready to have
// the fields set and be appended to a buffer.
func newBufferItem(events *structs.Events) *bufferItem {
	return &bufferItem{
		link: &bufferLink{
			nextCh:    make(chan struct{}),
			droppedCh: make(chan struct{}),
		},
		Events:    events,
		createdAt: time.Now(),
	}
}

// Next return the next buffer item in the buffer. It may block until ctx is
// cancelled or until the next item is published.
func (i *bufferItem) Next(ctx context.Context, forceClose <-chan struct{}) (*bufferItem, error) {
	// See if there is already a next value, block if so. Note we don't rely on
	// state change (chan nil) as that's not threadsafe but detecting close is.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-forceClose:
		return nil, fmt.Errorf("subscription closed")
	case <-i.link.nextCh:
	}

	// Check if the reader is too slow and the event buffer as discarded the event
	// This must happen after the above select to prevent a random selection
	// between linkCh and droppedCh
	select {
	case <-i.link.droppedCh:
		return nil, fmt.Errorf("event dropped from buffer")
	default:
	}

	// If channel closed, there must be a next item to read
	nextRaw := i.link.next.Load()
	if nextRaw == nil {
		// shouldn't be possible
		return nil, errors.New("invalid next item")
	}
	next := nextRaw.(*bufferItem)
	if next.Err != nil {
		return nil, next.Err
	}
	return next, nil
}

// NextNoBlock returns the next item in the buffer without blocking. If it
// reaches the most recent item it will return nil.
func (i *bufferItem) NextNoBlock() *bufferItem {
	nextRaw := i.link.next.Load()
	if nextRaw == nil {
		return nil
	}
	return nextRaw.(*bufferItem)
}
