package scheduler

import (
	"math"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// serviceJobAntiAffinityPenalty is the penalty applied
	// to the score for placing an alloc on a node that
	// already has an alloc for this job.
	serviceJobAntiAffinityPenalty = 20.0

	// batchJobAntiAffinityPenalty is the same as the
	// serviceJobAntiAffinityPenalty but for batch type jobs.
	batchJobAntiAffinityPenalty = 10.0
)

// Stack is a chained collection of iterators. The stack is used to
// make placement decisions. Different schedulers may customize the
// stack they use to vary the way placements are made.
type Stack interface {
	// SetNodes is used to set the base set of potential nodes
	SetNodes([]*structs.Node)

	// SetTaskGroup is used to set the job for selection
	SetJob(job *structs.Job)

	// Select is used to select a node for the task group
	Select(tg *structs.TaskGroup) (*RankedNode, *structs.Resources)
}

// GenericStack is the Stack used for the Generic scheduler. It is
// designed to make better placement decisions at the cost of performance.
type GenericStack struct {
	batch  bool
	ctx    Context
	source *StaticIterator

	wrappedChecks       *FeasibilityWrapper
	jobConstraint       *ConstraintChecker
	taskGroupDrivers    *DriverChecker
	taskGroupConstraint *ConstraintChecker

	distinctHostsConstraint    *DistinctHostsIterator
	distinctPropertyConstraint *DistinctPropertyIterator
	binPack                    *BinPackIterator
	jobAntiAff                 *JobAntiAffinityIterator
	limit                      *LimitIterator
	maxScore                   *MaxScoreIterator
}

// NewGenericStack constructs a stack used for selecting service placements
func NewGenericStack(batch bool, ctx Context) *GenericStack {
	// Create a new stack
	s := &GenericStack{
		batch: batch,
		ctx:   ctx,
	}

	// Create the source iterator. We randomize the order we visit nodes
	// to reduce collisions between schedulers and to do a basic load
	// balancing across eligible nodes.
	s.source = NewRandomIterator(ctx, nil)

	// Attach the job constraints. The job is filled in later.
	s.jobConstraint = NewConstraintChecker(ctx, nil)

	// Filter on task group drivers first as they are faster
	s.taskGroupDrivers = NewDriverChecker(ctx, nil)

	// Filter on task group constraints second
	s.taskGroupConstraint = NewConstraintChecker(ctx, nil)

	// Create the feasibility wrapper which wraps all feasibility checks in
	// which feasibility checking can be skipped if the computed node class has
	// previously been marked as eligible or ineligible. Generally this will be
	// checks that only needs to examine the single node to determine feasibility.
	jobs := []FeasibilityChecker{s.jobConstraint}
	tgs := []FeasibilityChecker{s.taskGroupDrivers, s.taskGroupConstraint}
	s.wrappedChecks = NewFeasibilityWrapper(ctx, s.source, jobs, tgs)

	// Filter on distinct host constraints.
	s.distinctHostsConstraint = NewDistinctHostsIterator(ctx, s.wrappedChecks)

	// Filter on distinct property constraints.
	s.distinctPropertyConstraint = NewDistinctPropertyIterator(ctx, s.distinctHostsConstraint)

	// Upgrade from feasible to rank iterator
	rankSource := NewFeasibleRankIterator(ctx, s.distinctPropertyConstraint)

	// Apply the bin packing, this depends on the resources needed
	// by a particular task group. Only enable eviction for the service
	// scheduler as that logic is expensive.
	evict := !batch
	s.binPack = NewBinPackIterator(ctx, rankSource, evict, 0)

	// Apply the job anti-affinity iterator. This is to avoid placing
	// multiple allocations on the same node for this job. The penalty
	// is less for batch jobs as it matters less.
	penalty := serviceJobAntiAffinityPenalty
	if batch {
		penalty = batchJobAntiAffinityPenalty
	}
	s.jobAntiAff = NewJobAntiAffinityIterator(ctx, s.binPack, penalty, "")

	// Apply a limit function. This is to avoid scanning *every* possible node.
	s.limit = NewLimitIterator(ctx, s.jobAntiAff, 2)

	// Select the node with the maximum score for placement
	s.maxScore = NewMaxScoreIterator(ctx, s.limit)
	return s
}

func (s *GenericStack) SetNodes(baseNodes []*structs.Node) {
	// Shuffle base nodes
	shuffleNodes(baseNodes)

	// Update the set of base nodes
	s.source.SetNodes(baseNodes)

	// Apply a limit function. This is to avoid scanning *every* possible node.
	// For batch jobs we only need to evaluate 2 options and depend on the
	// power of two choices. For services jobs we need to visit "enough".
	// Using a log of the total number of nodes is a good restriction, with
	// at least 2 as the floor
	limit := 2
	if n := len(baseNodes); !s.batch && n > 0 {
		logLimit := int(math.Ceil(math.Log2(float64(n))))
		if logLimit > limit {
			limit = logLimit
		}
	}
	s.limit.SetLimit(limit)
}

func (s *GenericStack) SetJob(job *structs.Job) {
	s.jobConstraint.SetConstraints(job.Constraints)
	s.distinctHostsConstraint.SetJob(job)
	s.distinctPropertyConstraint.SetJob(job)
	s.binPack.SetPriority(job.Priority)
	s.jobAntiAff.SetJob(job.ID)
	s.ctx.Eligibility().SetJob(job)
}

func (s *GenericStack) Select(tg *structs.TaskGroup) (*RankedNode, *structs.Resources) {
	// Reset the max selector and context
	s.maxScore.Reset()
	s.ctx.Reset()
	start := time.Now()

	// Get the task groups constraints.
	tgConstr := taskGroupConstraints(tg)

	// Update the parameters of iterators
	s.taskGroupDrivers.SetDrivers(tgConstr.drivers)
	s.taskGroupConstraint.SetConstraints(tgConstr.constraints)
	s.distinctHostsConstraint.SetTaskGroup(tg)
	s.distinctPropertyConstraint.SetTaskGroup(tg)
	s.wrappedChecks.SetTaskGroup(tg.Name)
	s.binPack.SetTaskGroup(tg)

	// Find the node with the max score
	option := s.maxScore.Next()

	// Ensure that the task resources were specified
	if option != nil && len(option.TaskResources) != len(tg.Tasks) {
		for _, task := range tg.Tasks {
			option.SetTaskResources(task, task.Resources)
		}
	}

	// Store the compute time
	s.ctx.Metrics().AllocationTime = time.Since(start)
	return option, tgConstr.size
}

// SelectPreferredNode returns a node where an allocation of the task group can
// be placed, the node passed to it is preferred over the other available nodes
func (s *GenericStack) SelectPreferringNodes(tg *structs.TaskGroup, nodes []*structs.Node) (*RankedNode, *structs.Resources) {
	originalNodes := s.source.nodes
	s.source.SetNodes(nodes)
	if option, resources := s.Select(tg); option != nil {
		s.source.SetNodes(originalNodes)
		return option, resources
	}
	s.source.SetNodes(originalNodes)
	return s.Select(tg)
}

// SystemStack is the Stack used for the System scheduler. It is designed to
// attempt to make placements on all nodes.
type SystemStack struct {
	ctx                        Context
	source                     *StaticIterator
	wrappedChecks              *FeasibilityWrapper
	jobConstraint              *ConstraintChecker
	taskGroupDrivers           *DriverChecker
	taskGroupConstraint        *ConstraintChecker
	distinctPropertyConstraint *DistinctPropertyIterator
	binPack                    *BinPackIterator
}

// NewSystemStack constructs a stack used for selecting service placements
func NewSystemStack(ctx Context) *SystemStack {
	// Create a new stack
	s := &SystemStack{ctx: ctx}

	// Create the source iterator. We visit nodes in a linear order because we
	// have to evaluate on all nodes.
	s.source = NewStaticIterator(ctx, nil)

	// Attach the job constraints. The job is filled in later.
	s.jobConstraint = NewConstraintChecker(ctx, nil)

	// Filter on task group drivers first as they are faster
	s.taskGroupDrivers = NewDriverChecker(ctx, nil)

	// Filter on task group constraints second
	s.taskGroupConstraint = NewConstraintChecker(ctx, nil)

	// Create the feasibility wrapper which wraps all feasibility checks in
	// which feasibility checking can be skipped if the computed node class has
	// previously been marked as eligible or ineligible. Generally this will be
	// checks that only needs to examine the single node to determine feasibility.
	jobs := []FeasibilityChecker{s.jobConstraint}
	tgs := []FeasibilityChecker{s.taskGroupDrivers, s.taskGroupConstraint}
	s.wrappedChecks = NewFeasibilityWrapper(ctx, s.source, jobs, tgs)

	// Filter on distinct property constraints.
	s.distinctPropertyConstraint = NewDistinctPropertyIterator(ctx, s.wrappedChecks)

	// Upgrade from feasible to rank iterator
	rankSource := NewFeasibleRankIterator(ctx, s.distinctPropertyConstraint)

	// Apply the bin packing, this depends on the resources needed
	// by a particular task group. Enable eviction as system jobs are high
	// priority.
	s.binPack = NewBinPackIterator(ctx, rankSource, true, 0)
	return s
}

func (s *SystemStack) SetNodes(baseNodes []*structs.Node) {
	// Update the set of base nodes
	s.source.SetNodes(baseNodes)
}

func (s *SystemStack) SetJob(job *structs.Job) {
	s.jobConstraint.SetConstraints(job.Constraints)
	s.distinctPropertyConstraint.SetJob(job)
	s.binPack.SetPriority(job.Priority)
	s.ctx.Eligibility().SetJob(job)
}

func (s *SystemStack) Select(tg *structs.TaskGroup) (*RankedNode, *structs.Resources) {
	// Reset the binpack selector and context
	s.binPack.Reset()
	s.ctx.Reset()
	start := time.Now()

	// Get the task groups constraints.
	tgConstr := taskGroupConstraints(tg)

	// Update the parameters of iterators
	s.taskGroupDrivers.SetDrivers(tgConstr.drivers)
	s.taskGroupConstraint.SetConstraints(tgConstr.constraints)
	s.wrappedChecks.SetTaskGroup(tg.Name)
	s.distinctPropertyConstraint.SetTaskGroup(tg)
	s.binPack.SetTaskGroup(tg)

	// Get the next option that satisfies the constraints.
	option := s.binPack.Next()

	// Ensure that the task resources were specified
	if option != nil && len(option.TaskResources) != len(tg.Tasks) {
		for _, task := range tg.Tasks {
			option.SetTaskResources(task, task.Resources)
		}
	}

	// Store the compute time
	s.ctx.Metrics().AllocationTime = time.Since(start)
	return option, tgConstr.size
}
