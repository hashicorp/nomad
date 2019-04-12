// +build !pro,!ent

package scheduler

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

	// Apply node rescheduling penalty. This tries to avoid placing on a
	// node where the allocation failed previously
	s.nodeReschedulingPenalty = NewNodeReschedulingPenaltyIterator(ctx, s.jobAntiAff)

	// Apply scores based on affinity stanza
	s.nodeAffinity = NewNodeAffinityIterator(ctx, s.nodeReschedulingPenalty)

	// Apply scores based on spread stanza
	s.spread = NewSpreadIterator(ctx, s.nodeAffinity)

	// Normalizes scores by averaging them across various scorers
	s.scoreNorm = NewScoreNormalizationIterator(ctx, s.spread)

	// Apply a limit function. This is to avoid scanning *every* possible node.
	s.limit = NewLimitIterator(ctx, s.scoreNorm, 2, skipScoreThreshold, maxSkip)

	// Select the node with the maximum score for placement
	s.maxScore = NewMaxScoreIterator(ctx, s.limit)
	return s
}
