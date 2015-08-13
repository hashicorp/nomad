package scheduler

import (
	"log"
	"os"
	"sync"
	"testing"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Harness is a lightweight testing harness for schedulers.
// It manages a state store copy and provides the planner
// interface. It can be extended for various testing uses.
type Harness struct {
	State *state.StateStore

	Planner  Planner
	planLock sync.Mutex

	Plans []*structs.Plan

	nextIndex     uint64
	nextIndexLock sync.Mutex
}

// NewHarness is used to make a new testing harness
func NewHarness(t *testing.T) *Harness {
	state, err := state.NewStateStore(os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	h := &Harness{
		State:     state,
		nextIndex: 1,
	}
	return h
}

// SubmitPlan is used to handle plan submission
func (h *Harness) SubmitPlan(plan *structs.Plan) (*structs.PlanResult, State, error) {
	// Ensure sequential plan application
	h.planLock.Lock()
	defer h.planLock.Unlock()

	// Store the plan
	h.Plans = append(h.Plans, plan)

	// Check for custom planner
	if h.Planner != nil {
		return h.Planner.SubmitPlan(plan)
	}

	// Get the index
	index := h.NextIndex()

	// Prepare the result
	result := new(structs.PlanResult)
	result.NodeEvict = plan.NodeEvict
	result.NodeAllocation = plan.NodeAllocation
	result.AllocIndex = index

	// Flatten evicts and allocs
	var evicts []string
	var allocs []*structs.Allocation
	for _, evictList := range plan.NodeEvict {
		evicts = append(evicts, evictList...)
	}
	for _, allocList := range plan.NodeAllocation {
		allocs = append(allocs, allocList...)
	}

	// Apply the full plan
	err := h.State.UpdateAllocations(index, evicts, allocs)
	return result, nil, err
}

// NextIndex returns the next index
func (h *Harness) NextIndex() uint64 {
	h.nextIndexLock.Lock()
	defer h.nextIndexLock.Unlock()
	idx := h.nextIndex
	h.nextIndex += 1
	return idx
}

// Snapshot is used to snapshot the current state
func (h *Harness) Snapshot() State {
	snap, _ := h.State.Snapshot()
	return snap
}

// Scheduler is used to return a new scheduler from
// a snapshot of current state using the harness for planning.
func (h *Harness) Scheduler(factory Factory) Scheduler {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	return factory(logger, h.Snapshot(), h)
}

// Process is used to process an evaluation given a factory
// function to create the scheduler
func (h *Harness) Process(factory Factory, eval *structs.Evaluation) error {
	sched := h.Scheduler(factory)
	return sched.Process(eval)
}

// noErr is used to assert there are no errors
func noErr(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}
