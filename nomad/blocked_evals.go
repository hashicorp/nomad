package nomad

import (
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/nomad/structs"
)

// BlockedEvals is used to track evaluations that shouldn't be queued until a
// certain class of nodes becomes available. An evaluation is put into the
// blocked state when it is run through the scheduler and produced failed
// allocations. It is unblocked when the capacity of a node that could run the
// failed allocation becomes available.
type BlockedEvals struct {
	evalBroker *EvalBroker
	enabled    bool
	stats      *BlockedStats

	// captured is the set of evaluations that are captured by computed node
	// classes.
	captured map[string]*structs.Evaluation

	// escaped is the set of evaluations that have escaped computed node
	// classes.
	escaped map[string]*structs.Evaluation

	l sync.RWMutex
}

// BlockedStats returns all the stats about the blocked eval tracker.
type BlockedStats struct {
	// The number of blocked evaluations that have escaped computed node
	// classses.
	TotalEscaped int

	// The number of blocked evaluations that are captured by computed node
	// classses.
	TotalCaptured int
}

// NewBlockedEvals creates a new blocked eval tracker that will enqueue
// unblocked evals into the passed broker.
func NewBlockedEvals(evalBroker *EvalBroker) *BlockedEvals {
	return &BlockedEvals{
		evalBroker: evalBroker,
		captured:   make(map[string]*structs.Evaluation),
		escaped:    make(map[string]*structs.Evaluation),
		stats:      new(BlockedStats),
	}
}

// Enabled is used to check if the broker is enabled.
func (b *BlockedEvals) Enabled() bool {
	b.l.RLock()
	defer b.l.RUnlock()
	return b.enabled
}

// SetEnabled is used to control if the broker is enabled. The broker
// should only be enabled on the active leader.
func (b *BlockedEvals) SetEnabled(enabled bool) {
	b.l.Lock()
	b.enabled = enabled
	b.l.Unlock()
	if !enabled {
		b.Flush()
	}
}

// Block tracks the passed evaluation and enqueues it into the eval broker when
// a suitable node calls unblock.
func (b *BlockedEvals) Block(eval *structs.Evaluation) {
	b.l.Lock()
	defer b.l.Unlock()

	// Do nothing if not enabled
	if !b.enabled {
		return
	}

	if eval.EscapedComputedClass {
		b.escaped[eval.ID] = eval
		b.stats.TotalEscaped++
		return
	}

	b.captured[eval.ID] = eval
	b.stats.TotalCaptured++
}

// Unblock causes any evaluation that could potentially make progress on a
// capacity change on the passed computed node class to be enqueued into the
// eval broker.
func (b *BlockedEvals) Unblock(computedClass uint64) {
	b.l.Lock()
	defer b.l.Unlock()

	// Do nothing if not enabled
	if !b.enabled {
		return
	}

	// Every eval that has escaped computed node class has to be unblocked
	// because any node could potentially be feasible.
	i := 0
	l := len(b.escaped)
	var unblocked []*structs.Evaluation
	if l != 0 {
		unblocked = make([]*structs.Evaluation, l)
		for id, eval := range b.escaped {
			unblocked[i] = eval
			delete(b.escaped, id)
			i++
		}
	}

	// Reset the escaped
	b.stats.TotalEscaped = 0

	// We unblock any eval that is explicitely eligible for the computed class
	// and also any eval that is not eligible or uneligible. This signifies that
	// when the evaluation was originally run through the scheduler, that it
	// never saw a node with the given computed class and thus needs to be
	// unblocked for correctness.
	var untrack []string
	for id, eval := range b.captured {
		if _, ok := eval.EligibleClasses[computedClass]; ok {
			goto UNBLOCK
		} else if _, ok := eval.IneligibleClasses[computedClass]; ok {
			// Can skip because the eval has explicitely marked the node class
			// as ineligible.
			continue
		}

	UNBLOCK:
		// The computed node class has never been seen by the eval so we unblock
		// it.
		unblocked = append(unblocked, eval)
		untrack = append(untrack, id)
	}

	// Untrack the unblocked evals.
	if l := len(untrack); l != 0 {
		for _, id := range untrack {
			delete(b.captured, id)
		}

		// Update the stats on captured evals.
		b.stats.TotalCaptured -= len(untrack)
	}

	if len(unblocked) != 0 {
		// Enqueue all the unblocked evals into the broker.
		b.evalBroker.EnqueueAll(unblocked)
	}
}

// Flush is used to clear the state of blocked evaluations.
func (b *BlockedEvals) Flush() {
	b.l.Lock()
	defer b.l.Unlock()

	// Reset the blocked eval tracker.
	b.stats.TotalEscaped = 0
	b.stats.TotalCaptured = 0
	b.captured = make(map[string]*structs.Evaluation)
	b.escaped = make(map[string]*structs.Evaluation)
}

// Stats is used to query the state of the blocked eval tracker.
func (b *BlockedEvals) Stats() *BlockedStats {
	// Allocate a new stats struct
	stats := new(BlockedStats)

	b.l.RLock()
	defer b.l.RUnlock()

	// Copy all the stats
	stats.TotalEscaped = b.stats.TotalEscaped
	stats.TotalCaptured = b.stats.TotalCaptured
	return stats
}

// EmitStats is used to export metrics about the blocked eval tracker while enabled
func (b *BlockedEvals) EmitStats(period time.Duration, stopCh chan struct{}) {
	for {
		select {
		case <-time.After(period):
			stats := b.Stats()
			metrics.SetGauge([]string{"nomad", "blocked_evals", "total_captured"}, float32(stats.TotalCaptured))
			metrics.SetGauge([]string{"nomad", "blocked_evals", "total_escaped"}, float32(stats.TotalEscaped))
		case <-stopCh:
			return
		}
	}
}
