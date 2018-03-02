package drainerv2

import (
	"context"
	"sync"
	"time"
)

// DrainDeadlineNotifier allows batch notification of nodes that have reached
// their drain deadline.
type DrainDeadlineNotifier interface {
	// NextBatch returns the next batch of nodes that have reached their
	// deadline.
	NextBatch() <-chan []string

	// Remove removes the given node from being tracked for a deadline.
	Remove(nodeID string)

	// Watch marks the given node for being watched for its deadline.
	Watch(nodeID string, deadline time.Time)
}

// TODO Make any of what I just wrote true :) Initially it is just a simple
// implementation.

// deadlineHeap implements the DrainDeadlineNotifier and is backed by a min-heap
// to efficiently determine the next deadlining node. It also supports
// coalescing several deadlines into a single emission.
type deadlineHeap struct {
	ctx            context.Context
	coalesceWindow time.Duration
	batch          chan []string
	nodes          map[string]time.Time
	trigger        chan string
	l              sync.RWMutex
}

// NewDeadlineHeap returns a new deadline heap that coalesces for the given
// duration and will stop watching when the passed context is cancelled.
func NewDeadlineHeap(ctx context.Context, coalesceWindow time.Duration) *deadlineHeap {
	d := &deadlineHeap{
		ctx:            ctx,
		coalesceWindow: coalesceWindow,
		batch:          make(chan []string, 4),
		nodes:          make(map[string]time.Time, 64),
		trigger:        make(chan string, 4),
	}

	go d.watch()
	return d
}

func (d *deadlineHeap) watch() {
	timer := time.NewTimer(0 * time.Millisecond)
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}

	var nextDeadline time.Time
	defer timer.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-timer.C:
			if nextDeadline.IsZero() {
				continue
			}

			d.l.Lock()
			var batch []string
			for nodeID, nodeDeadline := range d.nodes {
				if !nodeDeadline.After(nextDeadline) {
					batch = append(batch, nodeID)
				}
			}

			// If there is nothing exit early
			if len(batch) == 0 {
				d.l.Unlock()
				goto CALC
			}

			// Send the batch
			select {
			case d.batch <- batch:
			case <-d.ctx.Done():
				d.l.Unlock()
				return
			}

			// Clean up the nodes
			for _, nodeID := range batch {
				delete(d.nodes, nodeID)
			}
			d.l.Unlock()
		case <-d.trigger:
		}

	CALC:
		deadline, ok := d.calculateNextDeadline()
		if !ok {
			continue
		}

		if !deadline.Equal(nextDeadline) {
			timer.Reset(deadline.Sub(time.Now()))
			nextDeadline = deadline
		}
	}
}

// calculateNextDeadline returns the next deadline in which to scan for
// deadlined nodes. It applies the coalesce window.
func (d *deadlineHeap) calculateNextDeadline() (time.Time, bool) {
	d.l.Lock()
	defer d.l.Unlock()

	if len(d.nodes) == 0 {
		return time.Time{}, false
	}

	// Calculate the new timer value
	var deadline time.Time
	for _, v := range d.nodes {
		if deadline.IsZero() || v.Before(deadline) {
			deadline = v
		}
	}

	var maxWithinWindow time.Time
	coalescedDeadline := deadline.Add(d.coalesceWindow)
	for _, nodeDeadline := range d.nodes {
		if nodeDeadline.Before(coalescedDeadline) {
			if maxWithinWindow.IsZero() || nodeDeadline.After(maxWithinWindow) {
				maxWithinWindow = nodeDeadline
			}
		}
	}

	return maxWithinWindow, true
}

// NextBatch returns the next batch of nodes to be drained.
func (d *deadlineHeap) NextBatch() <-chan []string {
	return d.batch
}

func (d *deadlineHeap) Remove(nodeID string) {
	d.l.Lock()
	defer d.l.Unlock()
	delete(d.nodes, nodeID)

	select {
	case d.trigger <- nodeID:
	default:
	}
}

func (d *deadlineHeap) Watch(nodeID string, deadline time.Time) {
	d.l.Lock()
	defer d.l.Unlock()
	d.nodes[nodeID] = deadline

	select {
	case d.trigger <- nodeID:
	default:
	}
}
