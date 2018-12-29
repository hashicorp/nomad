package scheduler

import (
	"math"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// skipScoreThreshold is a threshold used in the limit iterator to skip nodes
	// that have a score lower than this. -1 is the lowest possible score for a
	// node with penalties (based on job anti affinity and node rescheduling penalties
	skipScoreThreshold = 0.0

	// maxSkip limits the number of nodes that can be skipped in the limit iterator
	maxSkip = 3
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
	Select(tg *structs.TaskGroup, options *SelectOptions) *RankedNode
}

type SelectOptions struct {
	PenaltyNodeIDs map[string]struct{}
	PreferredNodes []*structs.Node
}

// GenericStack is the Stack used for the Generic scheduler. It is
// designed to make better placement decisions at the cost of performance.
type GenericStack struct {
	batch  bool
	ctx    Context
	source *StaticIterator

	wrappedChecks       *FeasibilityWrapper
	quota               FeasibleIterator
	jobConstraint       *ConstraintChecker
	taskGroupDrivers    *DriverChecker
	taskGroupConstraint *ConstraintChecker
	taskGroupDevices    *DeviceChecker

	distinctHostsConstraint    *DistinctHostsIterator
	distinctPropertyConstraint *DistinctPropertyIterator
	binPack                    *BinPackIterator
	jobAntiAff                 *JobAntiAffinityIterator
	nodeReschedulingPenalty    *NodeReschedulingPenaltyIterator
	limit                      *LimitIterator
	maxScore                   *MaxScoreIterator
	nodeAffinity               *NodeAffinityIterator
	spread                     *SpreadIterator
	scoreNorm                  *ScoreNormalizationIterator
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

	// Create the quota iterator to determine if placements would result in the
	// quota attached to the namespace of the job to go over.
	s.quota = NewQuotaIterator(ctx, s.source)

	// Attach the job constraints. The job is filled in later.
	s.jobConstraint = NewConstraintChecker(ctx, nil)

	// Filter on task group drivers first as they are faster
	s.taskGroupDrivers = NewDriverChecker(ctx, nil)

	// Filter on task group constraints second
	s.taskGroupConstraint = NewConstraintChecker(ctx, nil)

	// Filter on task group devices
	s.taskGroupDevices = NewDeviceChecker(ctx)

	// Create the feasibility wrapper which wraps all feasibility checks in
	// which feasibility checking can be skipped if the computed node class has
	// previously been marked as eligible or ineligible. Generally this will be
	// checks that only needs to examine the single node to determine feasibility.
	jobs := []FeasibilityChecker{s.jobConstraint}
	tgs := []FeasibilityChecker{s.taskGroupDrivers, s.taskGroupConstraint, s.taskGroupDevices}
	s.wrappedChecks = NewFeasibilityWrapper(ctx, s.quota, jobs, tgs)

	// Filter on distinct host constraints.
	s.distinctHostsConstraint = NewDistinctHostsIterator(ctx, s.wrappedChecks)

	// Filter on distinct property constraints.
	s.distinctPropertyConstraint = NewDistinctPropertyIterator(ctx, s.distinctHostsConstraint)

	// Upgrade from feasible to rank iterator
	rankSource := NewFeasibleRankIterator(ctx, s.distinctPropertyConstraint)

	// Apply the bin packing, this depends on the resources needed
	// by a particular task group.

	s.binPack = NewBinPackIterator(ctx, rankSource, false, 0)

	// Apply the job anti-affinity iterator. This is to avoid placing
	// multiple allocations on the same node for this job.
	s.jobAntiAff = NewJobAntiAffinityIterator(ctx, s.binPack, "")

	s.nodeReschedulingPenalty = NewNodeReschedulingPenaltyIterator(ctx, s.jobAntiAff)

	s.nodeAffinity = NewNodeAffinityIterator(ctx, s.nodeReschedulingPenalty)

	s.spread = NewSpreadIterator(ctx, s.nodeAffinity)

	s.scoreNorm = NewScoreNormalizationIterator(ctx, s.spread)

	// Apply a limit function. This is to avoid scanning *every* possible node.
	s.limit = NewLimitIterator(ctx, s.scoreNorm, 2, skipScoreThreshold, maxSkip)

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
	s.jobAntiAff.SetJob(job)
	s.nodeAffinity.SetJob(job)
	s.spread.SetJob(job)
	s.ctx.Eligibility().SetJob(job)

	if contextual, ok := s.quota.(ContextualIterator); ok {
		contextual.SetJob(job)
	}
}

func (s *GenericStack) Select(tg *structs.TaskGroup, options *SelectOptions) *RankedNode {

	// This block handles trying to select from preferred nodes if options specify them
	// It also sets back the set of nodes to the original nodes
	if options != nil && len(options.PreferredNodes) > 0 {
		originalNodes := s.source.nodes
		s.source.SetNodes(options.PreferredNodes)
		optionsNew := *options
		optionsNew.PreferredNodes = nil
		if option := s.Select(tg, &optionsNew); option != nil {
			s.source.SetNodes(originalNodes)
			return option
		}
		s.source.SetNodes(originalNodes)
		return s.Select(tg, &optionsNew)
	}

	// Reset the max selector and context
	s.maxScore.Reset()
	s.ctx.Reset()
	start := time.Now()

	// Get the task groups constraints.
	tgConstr := taskGroupConstraints(tg)

	// Update the parameters of iterators
	s.taskGroupDrivers.SetDrivers(tgConstr.drivers)
	s.taskGroupConstraint.SetConstraints(tgConstr.constraints)
	s.taskGroupDevices.SetTaskGroup(tg)
	s.distinctHostsConstraint.SetTaskGroup(tg)
	s.distinctPropertyConstraint.SetTaskGroup(tg)
	s.wrappedChecks.SetTaskGroup(tg.Name)
	s.binPack.SetTaskGroup(tg)
	s.jobAntiAff.SetTaskGroup(tg)
	if options != nil {
		s.nodeReschedulingPenalty.SetPenaltyNodes(options.PenaltyNodeIDs)
	}
	s.nodeAffinity.SetTaskGroup(tg)
	s.spread.SetTaskGroup(tg)

	if s.nodeAffinity.hasAffinities() || s.spread.hasSpreads() {
		s.limit.SetLimit(math.MaxInt32)
	}

	if contextual, ok := s.quota.(ContextualIterator); ok {
		contextual.SetTaskGroup(tg)
	}

	// Find the node with the max score
	option := s.maxScore.Next()

	// Store the compute time
	s.ctx.Metrics().AllocationTime = time.Since(start)
	return option
}

// SystemStack is the Stack used for the System scheduler. It is designed to
// attempt to make placements on all nodes.
type SystemStack struct {
	ctx    Context
	source *StaticIterator

	wrappedChecks       *FeasibilityWrapper
	quota               FeasibleIterator
	jobConstraint       *ConstraintChecker
	taskGroupDrivers    *DriverChecker
	taskGroupConstraint *ConstraintChecker
	taskGroupDevices    *DeviceChecker

	distinctPropertyConstraint *DistinctPropertyIterator
	binPack                    *BinPackIterator
	scoreNorm                  *ScoreNormalizationIterator
}

// NewSystemStack constructs a stack used for selecting service placements
func NewSystemStack(ctx Context) *SystemStack {
	// Create a new stack
	s := &SystemStack{ctx: ctx}

	// Create the source iterator. We visit nodes in a linear order because we
	// have to evaluate on all nodes.
	s.source = NewStaticIterator(ctx, nil)

	// Create the quota iterator to determine if placements would result in the
	// quota attached to the namespace of the job to go over.
	s.quota = NewQuotaIterator(ctx, s.source)

	// Attach the job constraints. The job is filled in later.
	s.jobConstraint = NewConstraintChecker(ctx, nil)

	// Filter on task group drivers first as they are faster
	s.taskGroupDrivers = NewDriverChecker(ctx, nil)

	// Filter on task group constraints second
	s.taskGroupConstraint = NewConstraintChecker(ctx, nil)

	// Filter on task group devices
	s.taskGroupDevices = NewDeviceChecker(ctx)

	// Create the feasibility wrapper which wraps all feasibility checks in
	// which feasibility checking can be skipped if the computed node class has
	// previously been marked as eligible or ineligible. Generally this will be
	// checks that only needs to examine the single node to determine feasibility.
	jobs := []FeasibilityChecker{s.jobConstraint}
	tgs := []FeasibilityChecker{s.taskGroupDrivers, s.taskGroupConstraint, s.taskGroupDevices}
	s.wrappedChecks = NewFeasibilityWrapper(ctx, s.quota, jobs, tgs)

	// Filter on distinct property constraints.
	s.distinctPropertyConstraint = NewDistinctPropertyIterator(ctx, s.wrappedChecks)

	// Upgrade from feasible to rank iterator
	rankSource := NewFeasibleRankIterator(ctx, s.distinctPropertyConstraint)

	// Apply the bin packing, this depends on the resources needed
	// by a particular task group. Enable eviction as system jobs are high
	// priority.
	_, schedConfig, _ := s.ctx.State().SchedulerConfig()
	enablePreemption := true
	if schedConfig != nil {
		enablePreemption = schedConfig.PreemptionConfig.SystemSchedulerEnabled
	}
	s.binPack = NewBinPackIterator(ctx, rankSource, enablePreemption, 0)

	// Apply score normalization
	s.scoreNorm = NewScoreNormalizationIterator(ctx, s.binPack)
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

	if contextual, ok := s.quota.(ContextualIterator); ok {
		contextual.SetJob(job)
	}
}

func (s *SystemStack) Select(tg *structs.TaskGroup, options *SelectOptions) *RankedNode {
	// Reset the binpack selector and context
	s.scoreNorm.Reset()
	s.ctx.Reset()
	start := time.Now()

	// Get the task groups constraints.
	tgConstr := taskGroupConstraints(tg)

	// Update the parameters of iterators
	s.taskGroupDrivers.SetDrivers(tgConstr.drivers)
	s.taskGroupConstraint.SetConstraints(tgConstr.constraints)
	s.taskGroupDevices.SetTaskGroup(tg)
	s.wrappedChecks.SetTaskGroup(tg.Name)
	s.distinctPropertyConstraint.SetTaskGroup(tg)
	s.binPack.SetTaskGroup(tg)

	if contextual, ok := s.quota.(ContextualIterator); ok {
		contextual.SetTaskGroup(tg)
	}

	// Get the next option that satisfies the constraints.
	option := s.scoreNorm.Next()

	// Store the compute time
	s.ctx.Metrics().AllocationTime = time.Since(start)
	return option
}
