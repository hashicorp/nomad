package scheduler

import (
	"fmt"
	"log"
	"math"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// maxScheduleAttempts is used to limit the number of times
	// we will attempt to schedule if we continue to hit conflicts.
	maxScheduleAttempts = 5
)

// ServiceScheduler is used for 'service' type jobs. This scheduler is
// designed for long-lived services, and as such spends more time attemping
// to make a high quality placement. This is the primary scheduler for
// most workloads.
type ServiceScheduler struct {
	logger  *log.Logger
	state   State
	planner Planner

	attempts int
	eval     *structs.Evaluation
	job      *structs.Job
	plan     *structs.Plan
}

// NewServiceScheduler is a factory function to instantiate a new service scheduler
func NewServiceScheduler(logger *log.Logger, state State, planner Planner) Scheduler {
	s := &ServiceScheduler{
		logger:  logger,
		state:   state,
		planner: planner,
	}
	return s
}

// Process is used to handle a single evaluation
func (s *ServiceScheduler) Process(eval *structs.Evaluation) error {
	// Store the evaluation
	s.eval = eval

	// Use the evaluation trigger reason to determine what we need to do
	switch eval.TriggeredBy {
	case structs.EvalTriggerJobRegister, structs.EvalTriggerNodeUpdate:
		return s.process(s.computeJobAllocs)
	case structs.EvalTriggerJobDeregister:
		return s.process(s.evictJobAllocs)
	default:
		return fmt.Errorf("service scheduler cannot handle '%s' evaluation reason",
			eval.TriggeredBy)
	}
}

// process is used to iteratively run the handler until we have no
// further work or we've made the maximum number of attempts.
func (s *ServiceScheduler) process(handler func() error) error {
START:
	// Check the attempt count
	if s.attempts == maxScheduleAttempts {
		return fmt.Errorf("maximum schedule attempts reached (%d)", s.attempts)
	}
	s.attempts += 1

	// Lookup the Job by ID
	job, err := s.state.GetJobByID(s.eval.JobID)
	if err != nil {
		return fmt.Errorf("failed to get job '%s': %v",
			s.eval.JobID, err)
	}
	s.job = job

	// Create a plan
	s.plan = s.eval.MakePlan(job)

	// Invoke the handler to setup the plan
	if err := handler(); err != nil {
		s.logger.Printf("[ERR] sched: %#v: %v", s.eval, err)
		return err
	}

	// Submit the plan
	result, newState, err := s.planner.SubmitPlan(s.plan)
	if err != nil {
		return err
	}

	// If we got a state refresh, try again since we have stale data
	if newState != nil {
		s.logger.Printf("[DEBUG] sched: %#v: refresh forced", s.eval)
		s.state = newState
		goto START
	}

	// Try again if the plan was not fully committed, potential conflict
	fullCommit, expected, actual := result.FullCommit(s.plan)
	if !fullCommit {
		s.logger.Printf("[DEBUG] sched: %#v: attempted %d placements, %d placed",
			s.eval, expected, actual)
		goto START
	}
	return nil
}

// computeJobAllocs is used to reconcile differences between the job,
// existing allocations and node status to update the allocations.
func (s *ServiceScheduler) computeJobAllocs() error {
	// If the job is missing, maybe a concurrent deregister
	if s.job == nil {
		s.logger.Printf("[DEBUG] sched: %#v: job not found, skipping", s.eval)
		return nil
	}

	// Materialize all the task groups
	groups := materializeTaskGroups(s.job)

	// If there is nothing required for this job, treat like a deregister
	if len(groups) == 0 {
		return s.evictJobAllocs()
	}

	// Lookup the allocations by JobID
	allocs, err := s.state.AllocsByJob(s.eval.JobID)
	if err != nil {
		return fmt.Errorf("failed to get allocs for job '%s': %v",
			s.eval.JobID, err)
	}

	// Determine the tainted nodes containing job allocs
	tainted, err := s.taintedNodes(allocs)
	if err != nil {
		return fmt.Errorf("failed to get tainted nodes for job '%s': %v",
			s.eval.JobID, err)
	}

	// Index the existing allocations
	indexed := indexAllocs(allocs)

	// Diff the required and existing allocations
	place, update, migrate, evict, ignore := diffAllocs(s.job, tainted, groups, indexed)
	s.logger.Printf("[DEBUG] sched: %#v: need %d placements, %d updates, %d migrations, %d evictions, %d ignored allocs",
		s.eval, len(place), len(update), len(migrate), len(evict), len(ignore))

	// Fast-pass if nothing to do
	if len(place) == 0 && len(update) == 0 && len(evict) == 0 && len(migrate) == 0 {
		return nil
	}

	// Add all the evicts
	addEvictsToPlan(s.plan, evict, indexed)

	// For simplicity, we treat all migrates as an evict + place.
	// XXX: This could probably be done more intelligently?
	addEvictsToPlan(s.plan, migrate, indexed)
	place = append(place, migrate...)

	// For simplicity, we treat all updates as an evict + place.
	// XXX: This should be done with rolling in-place updates instead.
	addEvictsToPlan(s.plan, update, indexed)
	place = append(place, update...)

	// Get the iteration stack
	stack, err := s.iterStack()
	if err != nil {
		return fmt.Errorf("failed to create iter stack: %v", err)
	}

	// Attempt to place all the allocations
	if err := s.planAllocations(stack, place, groups); err != nil {
		return fmt.Errorf("failed to plan allocations: %v", err)
	}
	return nil
}

// taintedNodes is used to scan the allocations and then check if the
// underlying nodes are tainted, and should force a migration of the allocation.
func (s *ServiceScheduler) taintedNodes(allocs []*structs.Allocation) (map[string]bool, error) {
	out := make(map[string]bool)
	for _, alloc := range allocs {
		if _, ok := out[alloc.NodeID]; ok {
			continue
		}

		node, err := s.state.GetNodeByID(alloc.NodeID)
		if err != nil {
			return nil, err
		}

		out[alloc.NodeID] = structs.ShouldDrainNode(node.Status)
	}
	return out, nil
}

// IteratorStack is used to hold pointers to each of the
// iterators which are chained together to do selection.
// Half of the stack is used for feasibility checking, while
// the second half of the stack is used for ranking and selection.
type IteratorStack struct {
	Context             *EvalContext
	BaseNodes           []*structs.Node
	Source              *StaticIterator
	JobConstraint       *ConstraintIterator
	TaskGroupDrivers    *DriverIterator
	TaskGroupConstraint *ConstraintIterator
	RankSource          *FeasibleRankIterator
	BinPack             *BinPackIterator
	Limit               *LimitIterator
	MaxScore            *MaxScoreIterator
}

// iterStack is used to get a set of base nodes and to
// initialize the entire stack of iterators.
func (s *ServiceScheduler) iterStack() (*IteratorStack, error) {
	// Create a new stack
	stack := new(IteratorStack)

	// Create an evaluation context
	stack.Context = NewEvalContext(s.state, s.plan, s.logger)

	// Get the base nodes
	nodes, err := readyNodesInDCs(s.state, s.job.Datacenters)
	if err != nil {
		return nil, err
	}
	stack.BaseNodes = nodes

	// Create the source iterator. We randomize the order we visit nodes
	// to reduce collisions between schedulers and to do a basic load
	// balancing across eligible nodes.
	stack.Source = NewRandomIterator(stack.Context, stack.BaseNodes)

	// Attach the job constraints.
	stack.JobConstraint = NewConstraintIterator(stack.Context, stack.Source, s.job.Constraints)

	// Create the task group filters, this must be filled in later
	stack.TaskGroupDrivers = NewDriverIterator(stack.Context, stack.JobConstraint, nil)
	stack.TaskGroupConstraint = NewConstraintIterator(stack.Context, stack.TaskGroupDrivers, nil)

	// Upgrade from feasible to rank iterator
	stack.RankSource = NewFeasibleRankIterator(stack.Context, stack.TaskGroupConstraint)

	// Apply the bin packing, this depends on the resources needed by
	// a particular task group.
	stack.BinPack = NewBinPackIterator(stack.Context, stack.RankSource, nil, true, s.job.Priority)

	// Apply a limit function. This is to avoid scanning *every* possible node.
	// Instead we need to visit "enough". Using a log of the total number of
	// nodes is a good restriction, with at least 2 as the floor
	limit := 2
	if n := len(nodes); n > 0 {
		logLimit := int(math.Ceil(math.Log2(float64(n))))
		if logLimit > limit {
			limit = logLimit
		}
	}
	stack.Limit = NewLimitIterator(stack.Context, stack.BinPack, limit)

	// Select the node with the maximum score for placement
	stack.MaxScore = NewMaxScoreIterator(stack.Context, stack.Limit)

	return stack, nil
}

func (s *ServiceScheduler) planAllocations(stack *IteratorStack,
	place []allocNameID, groups map[string]*structs.TaskGroup) error {

	// Attempt to place each missing allocation
	for _, missing := range place {
		taskGroup := groups[missing.Name]

		// Collect the constraints, drivers and resources required by each
		// sub-task to aggregate the TaskGroup totals
		constr := make([]*structs.Constraint, 0, len(taskGroup.Constraints))
		drivers := make(map[string]struct{})
		size := new(structs.Resources)
		constr = append(constr, taskGroup.Constraints...)
		for _, task := range taskGroup.Tasks {
			drivers[task.Driver] = struct{}{}
			constr = append(constr, task.Constraints...)
			size.Add(task.Resources)
		}

		// Update the parameters of iterators
		stack.MaxScore.Reset()
		stack.TaskGroupDrivers.SetDrivers(drivers)
		stack.TaskGroupConstraint.SetConstraints(constr)
		stack.BinPack.SetResources(size)

		// Select the best fit
		option := stack.MaxScore.Next()
		if option == nil {
			s.logger.Printf("[DEBUG] sched: %#v: failed to place alloc %s",
				s.eval, missing)
			continue
		}

		// Create an allocation for this
		alloc := &structs.Allocation{
			ID:        mock.GenerateUUID(),
			Name:      missing.Name,
			NodeID:    option.Node.ID,
			JobID:     s.job.ID,
			Job:       s.job,
			Resources: size,
			Metrics:   nil,
			Status:    structs.AllocStatusPending,
		}
		s.plan.AppendAlloc(alloc)
	}
	return nil
}

// evictJobAllocs is used to evict all job allocations
func (s *ServiceScheduler) evictJobAllocs() error {
	// Lookup the allocations by JobID
	allocs, err := s.state.AllocsByJob(s.eval.JobID)
	if err != nil {
		return fmt.Errorf("failed to get allocs for job '%s': %v",
			s.eval.JobID, err)
	}
	s.logger.Printf("[DEBUG] sched: %#v: %d evictions needed",
		s.eval, len(allocs))

	// Add each alloc to be evicted
	for _, alloc := range allocs {
		s.plan.AppendEvict(alloc)
	}
	return nil
}
