package nomad

import (
	"container/heap"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/nomad/structs"
)

// EvalBroker is used to manage brokering of evaluations. When an evaluation is
// created, due to a change in a job specification or a node, we put it into the
// broker. The broker sorts by evaluations by priority and scheduler type. This
// allows us to dequeue the highest priority work first, while also allowing sub-schedulers
// to only dequeue work they know how to handle. The broker is designed to be entirely
// in-memory and is managed by the leader node.
//
// The broker must provide at-least-once delivery semantics. It relies on explicit
// Ack/Nack messages to handle this. If a delivery is not Ack'd in a sufficient time
// span, it will be assumed Nack'd.
type EvalBroker struct {
	nackTimeout time.Duration

	enabled bool
	stats   *BrokerStats

	ready   map[string]PendingEvaluations
	unack   map[string]*unackEval
	waiting map[string]chan struct{}

	l sync.RWMutex
}

// unackEval tracks an unacknowledged evaluation along with the Nack timer
type unackEval struct {
	Eval      *structs.Evaluation
	NackTimer *time.Timer
}

// PendingEvaluations is a list of waiting evaluations.
// We implement the container/heap interface so that this is a
// priority queue
type PendingEvaluations []*structs.Evaluation

// NewEvalBroker creates a new evaluation broker. This is parameterized
// with the timeout used for messages that are not acknowledged before we
// assume a Nack and attempt to redeliver.
func NewEvalBroker(timeout time.Duration) (*EvalBroker, error) {
	if timeout < 0 {
		return nil, fmt.Errorf("timeout cannot be negative")
	}
	b := &EvalBroker{
		nackTimeout: timeout,
		enabled:     false,
		stats:       new(BrokerStats),
		ready:       make(map[string]PendingEvaluations),
		unack:       make(map[string]*unackEval),
		waiting:     make(map[string]chan struct{}),
	}
	b.stats.ByScheduler = make(map[string]*SchedulerStats)
	return b, nil
}

// Enabled is used to check if the broker is enabled.
func (b *EvalBroker) Enabled() bool {
	b.l.RLock()
	defer b.l.RUnlock()
	return b.enabled
}

// SetEnabled is used to control if the broker is enabled. The broker
// should only be enabled on the active leader.
func (b *EvalBroker) SetEnabled(enabled bool) {
	b.l.Lock()
	b.enabled = enabled
	b.l.Unlock()
	if !enabled {
		b.Flush()
	}
}

// Enqueue is used to enqueue an evaluation
func (b *EvalBroker) Enqueue(eval *structs.Evaluation) error {
	b.l.Lock()
	defer b.l.Unlock()
	return b.enqueueLocked(eval)
}

// enqueueLocked is used to enqueue with the lock held
func (b *EvalBroker) enqueueLocked(eval *structs.Evaluation) error {
	// Do nothing if not enabled
	if !b.enabled {
		return nil
	}

	// Find the pending by scheduler class
	pending, ok := b.ready[eval.Type]
	if !ok {
		pending = make([]*structs.Evaluation, 0, 16)
		if _, ok := b.waiting[eval.Type]; !ok {
			b.waiting[eval.Type] = make(chan struct{}, 1)
		}
	}

	// Push onto the heap
	heap.Push(&pending, eval)
	b.ready[eval.Type] = pending

	// Update the stats
	b.stats.TotalReady += 1
	bySched, ok := b.stats.ByScheduler[eval.Type]
	if !ok {
		bySched = &SchedulerStats{}
		b.stats.ByScheduler[eval.Type] = bySched
	}
	bySched.Ready += 1

	// Unblock any blocked dequeues
	select {
	case b.waiting[eval.Type] <- struct{}{}:
	default:
	}
	return nil
}

// Dequeue is used to perform a blocking dequeue
func (b *EvalBroker) Dequeue(schedulers []string, timeout time.Duration) (*structs.Evaluation, error) {
	var timeoutTimer *time.Timer
SCAN:
	// Scan for work
	eval, err := b.scanForSchedulers(schedulers)
	if err != nil {
		if timeoutTimer != nil {
			timeoutTimer.Stop()
		}
		return nil, err
	}

	// Check if we have something
	if eval != nil {
		if timeoutTimer != nil {
			timeoutTimer.Stop()
		}
		return eval, nil
	}

	// Setup the timeout channel the first time around
	if timeoutTimer == nil && timeout != 0 {
		timeoutTimer = time.NewTimer(timeout)
	}

	// Block until we get work
	scan := b.waitForSchedulers(schedulers, timeoutTimer.C)
	if scan {
		goto SCAN
	}
	return nil, nil
}

// scanForSchedulers scans for work on any of the schedulers. The highest priority work
// is dequeued first. This may return nothing if there is no work waiting.
func (b *EvalBroker) scanForSchedulers(schedulers []string) (*structs.Evaluation, error) {
	b.l.Lock()
	defer b.l.Unlock()

	// Do nothing if not enabled
	if !b.enabled {
		return nil, fmt.Errorf("eval broker disabled")
	}

	// Scan for eligible work
	var eligibleSched []string
	var eligiblePriority int
	for _, sched := range schedulers {
		// Get the pending queue
		pending, ok := b.ready[sched]
		if !ok {
			continue
		}

		// Peek at the next item
		ready := pending.Peek()
		if ready == nil {
			continue
		}

		// Add to eligible if equal or greater priority
		if len(eligibleSched) == 0 || ready.Priority > eligiblePriority {
			eligibleSched = []string{sched}
			eligiblePriority = ready.Priority

		} else if eligiblePriority > ready.Priority {
			continue

		} else if eligiblePriority == ready.Priority {
			eligibleSched = append(eligibleSched, sched)
		}
	}

	// Determine behavior based on eligible work
	switch n := len(eligibleSched); n {
	case 0:
		// No work to do!
		return nil, nil

	case 1:
		// Only a single task, dequeue
		return b.dequeueForSched(eligibleSched[0])

	default:
		// Multiple tasks. We pick a random task so that we fairly
		// distribute work.
		offset := rand.Int63() % int64(n)
		return b.dequeueForSched(eligibleSched[offset])
	}
}

// dequeueForSched is used to dequeue the next work item for a given scheduler.
// This assumes locks are held and that this scheduler has work
func (b *EvalBroker) dequeueForSched(sched string) (*structs.Evaluation, error) {
	// Get the pending queue
	pending := b.ready[sched]
	raw := heap.Pop(&pending)
	b.ready[sched] = pending
	eval := raw.(*structs.Evaluation)

	// Setup Nack timer
	nackTimer := time.AfterFunc(b.nackTimeout, func() {
		b.Nack(eval.ID)
	})

	// Add to the unack queue
	b.unack[eval.ID] = &unackEval{
		Eval:      eval,
		NackTimer: nackTimer,
	}

	// Update the stats
	b.stats.TotalReady -= 1
	b.stats.TotalUnacked += 1
	bySched := b.stats.ByScheduler[eval.Type]
	bySched.Ready -= 1
	bySched.Unacked += 1

	return eval, nil
}

// waitForSchedulers is used to wait for work on any of the scheduler or until a timeout.
// Returns if there is work waiting potentially.
func (b *EvalBroker) waitForSchedulers(schedulers []string, timeoutCh <-chan time.Time) bool {
	doneCh := make(chan struct{})
	readyCh := make(chan struct{}, 1)
	defer close(doneCh)

	// Start all the watchers
	b.l.Lock()
	for _, sched := range schedulers {
		waitCh, ok := b.waiting[sched]
		if !ok {
			waitCh = make(chan struct{}, 1)
			b.waiting[sched] = waitCh
		}

		// Start a goroutine that either waits for the waitCh on this scheduler
		// to unblock or for this waitForSchedulers call to return
		go func() {
			select {
			case <-waitCh:
				select {
				case readyCh <- struct{}{}:
				default:
				}
			case <-doneCh:
			}
		}()
	}
	b.l.Unlock()

	// Block until we have ready work and should scan, or until we timeout
	// and should not make an attempt to scan for work
	select {
	case <-readyCh:
		return true
	case <-timeoutCh:
		return false
	}
}

// Outstanding checks if an EvalID has been delivered but not acknowledged
func (b *EvalBroker) Outstanding(evalID string) bool {
	b.l.RLock()
	defer b.l.RUnlock()
	_, ok := b.unack[evalID]
	return ok
}

// Ack is used to positively acknowledge handling an evaluation
func (b *EvalBroker) Ack(evalID string) error {
	b.l.Lock()
	defer b.l.Unlock()

	// Lookup the unack'd eval
	unack, ok := b.unack[evalID]
	if !ok {
		return fmt.Errorf("Evaluation ID not found")
	}

	// Ensure we were able to stop the timer
	if !unack.NackTimer.Stop() {
		return fmt.Errorf("Evaluation ID Ack'd after Nack timer expiration")
	}

	// Cleanup
	delete(b.unack, evalID)

	// Update the stats
	b.stats.TotalUnacked -= 1
	bySched := b.stats.ByScheduler[unack.Eval.Type]
	bySched.Unacked -= 1
	return nil
}

// Nack is used to negatively acknowledge handling an evaluation
func (b *EvalBroker) Nack(evalID string) error {
	b.l.Lock()
	defer b.l.Unlock()

	// Lookup the unack'd eval
	unack, ok := b.unack[evalID]
	if !ok {
		return fmt.Errorf("Evaluation ID not found")
	}

	// Stop the timer, doesn't matter if we've missed it
	unack.NackTimer.Stop()

	// Cleanup
	delete(b.unack, evalID)

	// Update the stats
	b.stats.TotalUnacked -= 1
	bySched := b.stats.ByScheduler[unack.Eval.Type]
	bySched.Unacked -= 1

	// Re-enqueue the work
	// TODO: Re-enqueue at higher priority to avoid starvation.
	return b.enqueueLocked(unack.Eval)
}

// Flush is used to clear the state of the broker
func (b *EvalBroker) Flush() {
	b.l.Lock()
	defer b.l.Unlock()

	// Unblock any waiters
	for _, waitCh := range b.waiting {
		close(waitCh)
	}
	b.waiting = make(map[string]chan struct{})

	// Cancel any Nack timers
	for _, unack := range b.unack {
		unack.NackTimer.Stop()
	}

	// Reset the broker
	b.stats.TotalReady = 0
	b.stats.TotalUnacked = 0
	b.stats.ByScheduler = make(map[string]*SchedulerStats)
	b.ready = make(map[string]PendingEvaluations)
	b.unack = make(map[string]*unackEval)
}

// Stats is used to query the state of the broker
func (b *EvalBroker) Stats() *BrokerStats {
	// Allocate a new stats struct
	stats := new(BrokerStats)
	stats.ByScheduler = make(map[string]*SchedulerStats)

	b.l.RLock()
	defer b.l.RUnlock()

	// Copy all the stats
	stats.TotalReady = b.stats.TotalReady
	stats.TotalUnacked = b.stats.TotalUnacked
	for sched, subStat := range b.stats.ByScheduler {
		subStatCopy := new(SchedulerStats)
		*subStatCopy = *subStat
		stats.ByScheduler[sched] = subStatCopy
	}
	return stats
}

// EmitStats is used to export metrics about the broker while enabled
func (b *EvalBroker) EmitStats(period time.Duration) {
	for {
		<-time.After(period)

		stats := b.Stats()
		metrics.SetGauge([]string{"nomad", "broker", "total_ready"}, float32(stats.TotalReady))
		metrics.SetGauge([]string{"nomad", "broker", "total_unacked"}, float32(stats.TotalUnacked))
		for sched, schedStats := range stats.ByScheduler {
			metrics.SetGauge([]string{"nomad", "broker", sched, "ready"}, float32(schedStats.Ready))
			metrics.SetGauge([]string{"nomad", "broker", sched, "unacked"}, float32(schedStats.Unacked))
		}
	}
}

// BrokerStats returns all the stats about the broker
type BrokerStats struct {
	TotalReady   int
	TotalUnacked int
	ByScheduler  map[string]*SchedulerStats
}

// SchedulerStats returns the stats per scheduler
type SchedulerStats struct {
	Ready   int
	Unacked int
}

// Len is for the sorting interface
func (p PendingEvaluations) Len() int {
	return len(p)
}

// Less is for the sorting interface. We flip the check
// so that the "min" in the min-heap is the element with the
// highest priority
func (p PendingEvaluations) Less(i, j int) bool {
	if p[i].Priority != p[j].Priority {
		return !(p[i].Priority < p[j].Priority)
	}
	return p[i].CreateIndex < p[j].CreateIndex
}

// Swap is for the sorting interface
func (p PendingEvaluations) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

// Push is used to add a new evalution to the slice
func (p *PendingEvaluations) Push(e interface{}) {
	*p = append(*p, e.(*structs.Evaluation))
}

// Pop is used to remove an evaluation from the slice
func (p *PendingEvaluations) Pop() interface{} {
	n := len(*p)
	e := (*p)[n-1]
	(*p)[n-1] = nil
	*p = (*p)[:n-1]
	return e
}

// Peek is used to peek at the next element that would be popped
func (p PendingEvaluations) Peek() *structs.Evaluation {
	n := len(p)
	if n == 0 {
		return nil
	}
	return p[n-1]
}
