// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package drainer

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

// deadlineHeap implements the DrainDeadlineNotifier and is backed by a min-heap
// to efficiently determine the next deadlining node. It also supports
// coalescing several deadlines into a single emission.
type deadlineHeap struct {
	ctx            context.Context
	coalesceWindow time.Duration
	batch          chan []string
	nodes          map[string]time.Time
	trigger        chan struct{}
	mu             sync.Mutex
}

// NewDeadlineHeap returns a new deadline heap that coalesces for the given
// duration and will stop watching when the passed context is cancelled.
func NewDeadlineHeap(ctx context.Context, coalesceWindow time.Duration) *deadlineHeap {
	d := &deadlineHeap{
		ctx:            ctx,
		coalesceWindow: coalesceWindow,
		batch:          make(chan []string),
		nodes:          make(map[string]time.Time, 64),
		trigger:        make(chan struct{}, 1),
	}

	go d.watch()
	return d
}

func (d *deadlineHeap) watch() {
	timer := time.NewTimer(0)
	timer.Stop()
	select {
	case <-timer.C:
	default:
	}
	defer timer.Stop()

	var nextDeadline time.Time
	for {
		select {
		case <-d.ctx.Done():
			return
		case <-timer.C:
			var batch []string

			d.mu.Lock()
			for nodeID, nodeDeadline := range d.nodes {
				if !nodeDeadline.After(nextDeadline) {
					batch = append(batch, nodeID)
					delete(d.nodes, nodeID)
				}
			}
			d.mu.Unlock()

			if len(batch) > 0 {
				// Send the batch
				select {
				case d.batch <- batch:
				case <-d.ctx.Done():
					return
				}
			}

		case <-d.trigger:
		}

		// Calculate the next deadline
		deadline, ok := d.calculateNextDeadline()
		if !ok {
			continue
		}

		// If the deadline is zero, it is a force drain. Otherwise if the
		// deadline is in the future, see if we already have a timer setup to
		// handle it. If we don't create the timer.
		if deadline.IsZero() || !deadline.Equal(nextDeadline) {
			timer.Reset(time.Until(deadline))
			nextDeadline = deadline
		}
	}
}

// calculateNextDeadline returns the next deadline in which to scan for
// deadlined nodes. It applies the coalesce window.
func (d *deadlineHeap) calculateNextDeadline() (time.Time, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

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
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.nodes, nodeID)

	select {
	case d.trigger <- struct{}{}:
	default:
	}
}

func (d *deadlineHeap) Watch(nodeID string, deadline time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.nodes[nodeID] = deadline

	select {
	case d.trigger <- struct{}{}:
	default:
	}
}
