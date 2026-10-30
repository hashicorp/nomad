// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"time"
)

const (
	// DefaultBackfillBuffer is the default DrainSpec BackfillBuffer.
	//
	// Must be long enough to cover time from scheduling to starting on the
	// Client.
	DefaultBackfillBuffer = 30 * time.Second

	minBackfillBuffer = 1 * time.Second
)

func (d *DrainSpec) Validate() error {
	if d.DurationAware && d.Deadline <= 0 {
		return fmt.Errorf("duration-aware drains require a positive deadline")
	}
	if d.BackfillBuffer != 0 && d.BackfillBuffer < minBackfillBuffer {
		return fmt.Errorf("backfill buffer must be at least %s", minBackfillBuffer)
	}
	if !d.DurationAware && d.BackfillBuffer != 0 {
		return fmt.Errorf("backfill buffer requires a duration-aware drain")
	}
	return nil
}

func (d *DrainSpec) GetBackfillBuffer() time.Duration {
	if d.BackfillBuffer == 0 {
		return DefaultBackfillBuffer
	}
	return d.BackfillBuffer
}

// BackfillOpen returns true if this node can receive duration-aware
// placements.
func (n *Node) BackfillOpen(when time.Time) bool {
	if n == nil || n.Status != NodeStatusReady {
		return false
	}

	d := n.DrainStrategy
	if d == nil {
		return false
	}
	return d.DurationAware && d.ForceDeadline.Sub(when) > d.GetBackfillBuffer()
}

// MaxShutdown is the maximum amount of time an alloc can take shutting down
// based on the shutdown delay and kill timeouts of all tasks.
func (tg *TaskGroup) MaxShutdown() time.Duration {
	var total time.Duration
	var maxTask time.Duration

	for _, t := range tg.Tasks {
		if t.Leader {
			// Leader is shutdown first, so add its time directly
			total += t.ShutdownDelay
			total += t.KillTimeout
		} else {
			// All tasks are shutdown concurrently so use their max
			maxTask = max(maxTask, t.ShutdownDelay+t.KillTimeout)
		}
	}
	total += maxTask

	if tg.ShutdownDelay != nil {
		total += *tg.ShutdownDelay
	}

	return total
}

func backfillBounded(job *Job, tg *TaskGroup) bool {
	return job != nil && tg != nil &&
		(job.Type == JobTypeBatch || job.Type == JobTypeSysBatch) &&
		tg.MaxRunDuration != nil && *tg.MaxRunDuration > 0
}

// CanBackfill returns true if the node permits backfilling the group because
// the group will shutdown before the drain's deadline.
func (n *Node) CanBackfill(job *Job, tg *TaskGroup, when time.Time) bool {
	if !n.BackfillOpen(when) || !backfillBounded(job, tg) {
		return false
	}
	remaining := n.DrainStrategy.ForceDeadline.Sub(when) - n.DrainStrategy.GetBackfillBuffer()
	shutdown := tg.MaxShutdown()
	return shutdown < remaining && *tg.MaxRunDuration < remaining-shutdown
}

// BackfillUpdateAllowed preserves ordinary in-place updates and decreases in
// runtime, but prevents updates from extending work beyond the drain window.
// The original creation time, not the update time, anchors the runtime limit.
func (n *Node) BackfillUpdateAllowed(existing, updated *Allocation, now time.Time) bool {
	if updated.TerminalStatus() || existing.TerminalStatus() {
		return true
	}
	oldMax, oldBounded := existing.MaxRunDuration()
	if !oldBounded {
		return true // existing unbounded work already follows ordinary draining
	}
	newMax, newBounded := updated.MaxRunDuration()
	if !newBounded {
		return false
	}
	oldTG := existing.Job.LookupTaskGroup(existing.TaskGroup)
	newTG := updated.Job.LookupTaskGroup(updated.TaskGroup)
	if newMax <= oldMax && newTG.MaxShutdown() <= oldTG.MaxShutdown() {
		return true
	}
	if !n.BackfillOpen(now) || existing.CreateTime == 0 || !backfillBounded(updated.Job, newTG) {
		return false
	}
	expires := time.Unix(0, existing.CreateTime).Add(newMax)
	if expires.Before(now) {
		expires = now
	}
	remaining := n.DrainStrategy.ForceDeadline.Sub(expires)
	buffer := n.DrainStrategy.GetBackfillBuffer()
	return remaining > buffer && remaining-buffer > newTG.MaxShutdown()
}
