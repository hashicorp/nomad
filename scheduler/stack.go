// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	Preempt        bool
	AllocName      string
}

// GenericStack is the Stack used for the Generic scheduler. It is
// designed to make better placement decisions at the cost of performance.
type GenericStack struct {
	batch  bool
	ctx    Context
	source *StaticIterator

	wrappedChecks        *FeasibilityWrapper
	quota                FeasibleIterator
	jobVersion           *uint64
	jobConstraint        *ConstraintChecker
	taskGroupDrivers     *DriverChecker
	taskGroupConstraint  *ConstraintChecker
	taskGroupDevices     *DeviceChecker
	taskGroupHostVolumes *HostVolumeChecker
	taskGroupCSIVolumes  *CSIVolumeChecker
	taskGroupNetwork     *NetworkChecker

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

func (s *GenericStack) SetNodes(baseNodes []*structs.Node) {
	// Shuffle base nodes
	idx, _ := s.ctx.State().LatestIndex()
	shuffleNodes(s.ctx.Plan(), idx, baseNodes)

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
	if s.jobVersion != nil && *s.jobVersion == job.Version {
		return
	}

	jobVer := job.Version
	s.jobVersion = &jobVer

	s.jobConstraint.SetConstraints(job.Constraints)
	s.distinctHostsConstraint.SetJob(job)
	s.distinctPropertyConstraint.SetJob(job)
	s.binPack.SetJob(job)
	s.jobAntiAff.SetJob(job)
	s.nodeAffinity.SetJob(job)
	s.spread.SetJob(job)
	s.ctx.Eligibility().SetJob(job)
	s.taskGroupCSIVolumes.SetNamespace(job.Namespace)
	s.taskGroupCSIVolumes.SetJobID(job.ID)

	if contextual, ok := s.quota.(ContextualIterator); ok {
		contextual.SetJob(job)
	}
}

// SetSchedulerConfiguration applies the given scheduler configuration to
// process nodes. Scheduler configuration values may change per job depending
// on the node pool being used.
func (s *GenericStack) SetSchedulerConfiguration(schedConfig *structs.SchedulerConfiguration) {
	s.binPack.SetSchedulerConfiguration(schedConfig)
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
	s.taskGroupHostVolumes.SetVolumes(options.AllocName, tg.Volumes)
	s.taskGroupCSIVolumes.SetVolumes(options.AllocName, tg.Volumes)
	if len(tg.Networks) > 0 {
		s.taskGroupNetwork.SetNetwork(tg.Networks[0])
	}
	s.distinctHostsConstraint.SetTaskGroup(tg)
	s.distinctPropertyConstraint.SetTaskGroup(tg)
	s.wrappedChecks.SetTaskGroup(tg.Name)
	s.binPack.SetTaskGroup(tg)
	if options != nil {
		s.binPack.evict = options.Preempt
	}
	s.jobAntiAff.SetTaskGroup(tg)
	if options != nil {
		s.nodeReschedulingPenalty.SetPenaltyNodes(options.PenaltyNodeIDs)
	}
	s.nodeAffinity.SetTaskGroup(tg)
	s.spread.SetTaskGroup(tg)

	if s.nodeAffinity.hasAffinities() || s.spread.hasSpreads() {
		// scoring spread across all nodes has quadratic behavior, so
		// we need to consider a subset of nodes to keep evaluaton times
		// reasonable but enough to ensure spread is correct. this
		// value was empirically determined.
		s.limit.SetLimit(tg.Count)
		if tg.Count < 100 {
			s.limit.SetLimit(100)
		}
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

	wrappedChecks        *FeasibilityWrapper
	quota                FeasibleIterator
	jobConstraint        *ConstraintChecker
	taskGroupDrivers     *DriverChecker
	taskGroupConstraint  *ConstraintChecker
	taskGroupDevices     *DeviceChecker
	taskGroupHostVolumes *HostVolumeChecker
	taskGroupCSIVolumes  *CSIVolumeChecker
	taskGroupNetwork     *NetworkChecker

	distinctPropertyConstraint *DistinctPropertyIterator
	binPack                    *BinPackIterator
	scoreNorm                  *ScoreNormalizationIterator
}

// NewSystemStack constructs a stack used for selecting system and sysbatch
// job placements.
//
// sysbatch is used to determine which scheduler config option is used to
// control the use of preemption.
func NewSystemStack(sysbatch bool, ctx Context) *SystemStack {
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

	// Filter on task group host volumes
	s.taskGroupHostVolumes = NewHostVolumeChecker(ctx)

	// Filter on available, healthy CSI plugins
	s.taskGroupCSIVolumes = NewCSIVolumeChecker(ctx)

	// Filter on task group devices
	s.taskGroupDevices = NewDeviceChecker(ctx)

	// Filter on available client networks
	s.taskGroupNetwork = NewNetworkChecker(ctx)

	// Create the feasibility wrapper which wraps all feasibility checks in
	// which feasibility checking can be skipped if the computed node class has
	// previously been marked as eligible or ineligible. Generally this will be
	// checks that only needs to examine the single node to determine feasibility.
	jobs := []FeasibilityChecker{s.jobConstraint}
	tgs := []FeasibilityChecker{
		s.taskGroupDrivers,
		s.taskGroupConstraint,
		s.taskGroupHostVolumes,
		s.taskGroupDevices,
		s.taskGroupNetwork,
	}
	avail := []FeasibilityChecker{s.taskGroupCSIVolumes}
	s.wrappedChecks = NewFeasibilityWrapper(ctx, s.source, jobs, tgs, avail)

	// Filter on distinct property constraints.
	s.distinctPropertyConstraint = NewDistinctPropertyIterator(ctx, s.wrappedChecks)

	// Create the quota iterator to determine if placements would result in
	// the quota attached to the namespace of the job to go over.
	// Note: the quota iterator must be the last feasibility iterator before
	// we upgrade to ranking, or our quota usage will include ineligible
	// nodes!
	s.quota = NewQuotaIterator(ctx, s.distinctPropertyConstraint)

	// Upgrade from feasible to rank iterator
	rankSource := NewFeasibleRankIterator(ctx, s.quota)

	// Apply the bin packing, this depends on the resources needed
	// by a particular task group. Enable eviction as system jobs are high
	// priority.
	//
	// The scheduler configuration is read directly from state but only
	// values that can't be specified per node pool should be used. Other
	// values must be merged by calling schedConfig.WithNodePool() and set in
	// the stack by calling SetSchedulerConfiguration().
	_, schedConfig, _ := s.ctx.State().SchedulerConfig()
	enablePreemption := true
	if schedConfig != nil {
		if sysbatch {
			enablePreemption = schedConfig.PreemptionConfig.SysBatchSchedulerEnabled
		} else {
			enablePreemption = schedConfig.PreemptionConfig.SystemSchedulerEnabled
		}
	}

	// Create binpack iterator
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
	s.binPack.SetJob(job)
	s.ctx.Eligibility().SetJob(job)
	s.taskGroupCSIVolumes.SetNamespace(job.Namespace)
	s.taskGroupCSIVolumes.SetJobID(job.ID)

	if contextual, ok := s.quota.(ContextualIterator); ok {
		contextual.SetJob(job)
	}
}

// SetSchedulerConfiguration applies the given scheduler configuration to
// process nodes. Scheduler configuration values may change per job depending
// on the node pool being used.
func (s *SystemStack) SetSchedulerConfiguration(schedConfig *structs.SchedulerConfiguration) {
	s.binPack.SetSchedulerConfiguration(schedConfig)
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
	s.taskGroupHostVolumes.SetVolumes(options.AllocName, tg.Volumes)
	s.taskGroupCSIVolumes.SetVolumes(options.AllocName, tg.Volumes)
	if len(tg.Networks) > 0 {
		s.taskGroupNetwork.SetNetwork(tg.Networks[0])
	}
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

	// Filter on task group devices
	s.taskGroupDevices = NewDeviceChecker(ctx)

	// Filter on task group host volumes
	s.taskGroupHostVolumes = NewHostVolumeChecker(ctx)

	// Filter on available, healthy CSI plugins
	s.taskGroupCSIVolumes = NewCSIVolumeChecker(ctx)

	// Filter on available client networks
	s.taskGroupNetwork = NewNetworkChecker(ctx)

	// Create the feasibility wrapper which wraps all feasibility checks in
	// which feasibility checking can be skipped if the computed node class has
	// previously been marked as eligible or ineligible. Generally this will be
	// checks that only needs to examine the single node to determine feasibility.
	jobs := []FeasibilityChecker{s.jobConstraint}
	tgs := []FeasibilityChecker{
		s.taskGroupDrivers,
		s.taskGroupConstraint,
		s.taskGroupHostVolumes,
		s.taskGroupDevices,
		s.taskGroupNetwork,
	}
	avail := []FeasibilityChecker{s.taskGroupCSIVolumes}
	s.wrappedChecks = NewFeasibilityWrapper(ctx, s.source, jobs, tgs, avail)

	// Filter on distinct host constraints.
	s.distinctHostsConstraint = NewDistinctHostsIterator(ctx, s.wrappedChecks)

	// Filter on distinct property constraints.
	s.distinctPropertyConstraint = NewDistinctPropertyIterator(ctx, s.distinctHostsConstraint)

	// Create the quota iterator to determine if placements would result in
	// the quota attached to the namespace of the job to go over.
	// Note: the quota iterator must be the last feasibility iterator before
	// we upgrade to ranking, or our quota usage will include ineligible
	// nodes!
	s.quota = NewQuotaIterator(ctx, s.distinctPropertyConstraint)

	// Upgrade from feasible to rank iterator
	rankSource := NewFeasibleRankIterator(ctx, s.quota)

	// Apply the bin packing, this depends on the resources needed
	// by a particular task group.
	s.binPack = NewBinPackIterator(ctx, rankSource, false, 0)

	// Apply the job anti-affinity iterator. This is to avoid placing
	// multiple allocations on the same node for this job.
	s.jobAntiAff = NewJobAntiAffinityIterator(ctx, s.binPack, "")

	// Apply node rescheduling penalty. This tries to avoid placing on a
	// node where the allocation failed previously
	s.nodeReschedulingPenalty = NewNodeReschedulingPenaltyIterator(ctx, s.jobAntiAff)

	// Apply scores based on affinity block
	s.nodeAffinity = NewNodeAffinityIterator(ctx, s.nodeReschedulingPenalty)

	// Apply scores based on spread block
	s.spread = NewSpreadIterator(ctx, s.nodeAffinity)

	// Add the preemption options scoring iterator
	preemptionScorer := NewPreemptionScoringIterator(ctx, s.spread)

	// Normalizes scores by averaging them across various scorers
	s.scoreNorm = NewScoreNormalizationIterator(ctx, preemptionScorer)

	// Apply a limit function. This is to avoid scanning *every* possible node.
	s.limit = NewLimitIterator(ctx, s.scoreNorm, 2, skipScoreThreshold, maxSkip)

	// Select the node with the maximum score for placement
	s.maxScore = NewMaxScoreIterator(ctx, s.limit)
	return s
}

// taskGroupConstraints collects the constraints, drivers and resources required by each
// sub-task to aggregate the TaskGroup totals
func taskGroupConstraints(tg *structs.TaskGroup) tgConstrainTuple {
	c := tgConstrainTuple{
		constraints: make([]*structs.Constraint, 0, len(tg.Constraints)),
		drivers:     make(map[string]struct{}),
	}

	c.constraints = append(c.constraints, tg.Constraints...)
	for _, task := range tg.Tasks {
		c.drivers[task.Driver] = struct{}{}
		c.constraints = append(c.constraints, task.Constraints...)
	}

	return c
}

// tgConstrainTuple is used to store the total constraints of a task group.
type tgConstrainTuple struct {
	// Holds the combined constraints of the task group and all it's sub-tasks.
	constraints []*structs.Constraint

	// The set of required drivers within the task group.
	drivers map[string]struct{}
}
