package scheduler

import (
	"fmt"
	"math"

	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// binPackingMaxFitScore is the maximum possible bin packing fitness score.
	// This is used to normalize bin packing score to a value between 0 and 1
	binPackingMaxFitScore = 18.0
)

// Rank is used to provide a score and various ranking metadata
// along with a node when iterating. This state can be modified as
// various rank methods are applied.
type RankedNode struct {
	Node          *structs.Node
	FinalScore    float64
	Scores        []float64
	TaskResources map[string]*structs.AllocatedTaskResources

	// Allocs is used to cache the proposed allocations on the
	// node. This can be shared between iterators that require it.
	Proposed []*structs.Allocation

	// PreemptedAllocs is used by the BinpackIterator to identify allocs
	// that should be preempted in order to make the placement
	PreemptedAllocs []*structs.Allocation
}

func (r *RankedNode) GoString() string {
	return fmt.Sprintf("<Node: %s Score: %0.3f>", r.Node.ID, r.FinalScore)
}

func (r *RankedNode) ProposedAllocs(ctx Context) ([]*structs.Allocation, error) {
	if r.Proposed != nil {
		return r.Proposed, nil
	}

	p, err := ctx.ProposedAllocs(r.Node.ID)
	if err != nil {
		return nil, err
	}
	r.Proposed = p
	return p, nil
}

func (r *RankedNode) SetTaskResources(task *structs.Task,
	resource *structs.AllocatedTaskResources) {
	if r.TaskResources == nil {
		r.TaskResources = make(map[string]*structs.AllocatedTaskResources)
	}
	r.TaskResources[task.Name] = resource
}

// RankFeasibleIterator is used to iteratively yield nodes along
// with ranking metadata. The iterators may manage some state for
// performance optimizations.
type RankIterator interface {
	// Next yields a ranked option or nil if exhausted
	Next() *RankedNode

	// Reset is invoked when an allocation has been placed
	// to reset any stale state.
	Reset()
}

// FeasibleRankIterator is used to consume from a FeasibleIterator
// and return an unranked node with base ranking.
type FeasibleRankIterator struct {
	ctx    Context
	source FeasibleIterator
}

// NewFeasibleRankIterator is used to return a new FeasibleRankIterator
// from a FeasibleIterator source.
func NewFeasibleRankIterator(ctx Context, source FeasibleIterator) *FeasibleRankIterator {
	iter := &FeasibleRankIterator{
		ctx:    ctx,
		source: source,
	}
	return iter
}

func (iter *FeasibleRankIterator) Next() *RankedNode {
	option := iter.source.Next()
	if option == nil {
		return nil
	}
	ranked := &RankedNode{
		Node: option,
	}
	return ranked
}

func (iter *FeasibleRankIterator) Reset() {
	iter.source.Reset()
}

// StaticRankIterator is a RankIterator that returns a static set of results.
// This is largely only useful for testing.
type StaticRankIterator struct {
	ctx    Context
	nodes  []*RankedNode
	offset int
	seen   int
}

// NewStaticRankIterator returns a new static rank iterator over the given nodes
func NewStaticRankIterator(ctx Context, nodes []*RankedNode) *StaticRankIterator {
	iter := &StaticRankIterator{
		ctx:   ctx,
		nodes: nodes,
	}
	return iter
}

func (iter *StaticRankIterator) Next() *RankedNode {
	// Check if exhausted
	n := len(iter.nodes)
	if iter.offset == n || iter.seen == n {
		if iter.seen != n {
			iter.offset = 0
		} else {
			return nil
		}
	}

	// Return the next offset
	offset := iter.offset
	iter.offset += 1
	iter.seen += 1
	return iter.nodes[offset]
}

func (iter *StaticRankIterator) Reset() {
	iter.seen = 0
}

// BinPackIterator is a RankIterator that scores potential options
// based on a bin-packing algorithm.
type BinPackIterator struct {
	ctx       Context
	source    RankIterator
	evict     bool
	priority  int
	taskGroup *structs.TaskGroup
}

// NewBinPackIterator returns a BinPackIterator which tries to fit tasks
// potentially evicting other tasks based on a given priority.
func NewBinPackIterator(ctx Context, source RankIterator, evict bool, priority int) *BinPackIterator {
	iter := &BinPackIterator{
		ctx:      ctx,
		source:   source,
		evict:    evict,
		priority: priority,
	}
	return iter
}

func (iter *BinPackIterator) SetPriority(p int) {
	iter.priority = p
}

func (iter *BinPackIterator) SetTaskGroup(taskGroup *structs.TaskGroup) {
	iter.taskGroup = taskGroup
}

func (iter *BinPackIterator) Next() *RankedNode {
OUTER:
	for {
		// Get the next potential option
		option := iter.source.Next()
		if option == nil {
			return nil
		}

		// Get the proposed allocations
		proposed, err := option.ProposedAllocs(iter.ctx)
		if err != nil {
			iter.ctx.Logger().Named("binpack").Error("failed retrieving proposed allocations", "error", err)
			continue
		}

		// Index the existing network usage
		netIdx := structs.NewNetworkIndex()
		netIdx.SetNode(option.Node)
		netIdx.AddAllocs(proposed)

		// Create a device allocator
		devAllocator := newDeviceAllocator(iter.ctx, option.Node)
		devAllocator.AddAllocs(proposed)

		// Track the affinities of the devices
		totalDeviceAffinityWeight := 0.0
		sumMatchingAffinities := 0.0

		// Assign the resources for each task
		total := &structs.AllocatedResources{
			Tasks: make(map[string]*structs.AllocatedTaskResources,
				len(iter.taskGroup.Tasks)),
			Shared: structs.AllocatedSharedResources{
				DiskMB: int64(iter.taskGroup.EphemeralDisk.SizeMB),
			},
		}

		var allocsToPreempt []*structs.Allocation

		// Initialize preemptor with node
		preemptor := NewPreemptor(iter.priority, iter.ctx)
		preemptor.SetNode(option.Node)

		// Count the number of existing preemptions
		allPreemptions := iter.ctx.Plan().NodePreemptions
		var currentPreemptions []*structs.Allocation
		for _, allocs := range allPreemptions {
			currentPreemptions = append(currentPreemptions, allocs...)
		}
		preemptor.SetPreemptions(currentPreemptions)

		for _, task := range iter.taskGroup.Tasks {
			// Allocate the resources
			taskResources := &structs.AllocatedTaskResources{
				Cpu: structs.AllocatedCpuResources{
					CpuShares: int64(task.Resources.CPU),
				},
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: int64(task.Resources.MemoryMB),
				},
			}

			// Check if we need a network resource
			if len(task.Resources.Networks) > 0 {
				ask := task.Resources.Networks[0].Copy()
				offer, err := netIdx.AssignNetwork(ask)
				if offer == nil {
					// If eviction is not enabled, mark this node as exhausted and continue
					if !iter.evict {
						iter.ctx.Metrics().ExhaustedNode(option.Node,
							fmt.Sprintf("network: %s", err))
						netIdx.Release()
						continue OUTER
					}

					// Look for preemptible allocations to satisfy the network resource for this task
					preemptor.SetCandidates(proposed)

					netPreemptions := preemptor.PreemptForNetwork(ask, netIdx)
					if netPreemptions == nil {
						iter.ctx.Logger().Named("binpack").Error("preemption not possible ", "network_resource", ask)
						netIdx.Release()
						continue OUTER
					}
					allocsToPreempt = append(allocsToPreempt, netPreemptions...)

					// First subtract out preempted allocations
					proposed = structs.RemoveAllocs(proposed, netPreemptions)

					// Reset the network index and try the offer again
					netIdx.Release()
					netIdx = structs.NewNetworkIndex()
					netIdx.SetNode(option.Node)
					netIdx.AddAllocs(proposed)

					offer, err = netIdx.AssignNetwork(ask)
					if offer == nil {
						iter.ctx.Logger().Named("binpack").Error("unexpected error, unable to create network offer after considering preemption", "error", err)
						netIdx.Release()
						continue OUTER
					}
				}

				// Reserve this to prevent another task from colliding
				netIdx.AddReserved(offer)

				// Update the network ask to the offer
				taskResources.Networks = []*structs.NetworkResource{offer}
			}

			// Check if we need to assign devices
			for _, req := range task.Resources.Devices {
				offer, sumAffinities, err := devAllocator.AssignDevice(req)
				if offer == nil {
					// If eviction is not enabled, mark this node as exhausted and continue
					if !iter.evict {
						iter.ctx.Metrics().ExhaustedNode(option.Node, fmt.Sprintf("devices: %s", err))
						continue OUTER
					}

					// Attempt preemption
					preemptor.SetCandidates(proposed)
					devicePreemptions := preemptor.PreemptForDevice(req, devAllocator)

					if devicePreemptions == nil {
						iter.ctx.Logger().Named("binpack").Error("preemption not possible", "requested_device", req)
						netIdx.Release()
						continue OUTER
					}
					allocsToPreempt = append(allocsToPreempt, devicePreemptions...)

					// First subtract out preempted allocations
					proposed = structs.RemoveAllocs(proposed, allocsToPreempt)

					// Reset the device allocator with new set of proposed allocs
					devAllocator := newDeviceAllocator(iter.ctx, option.Node)
					devAllocator.AddAllocs(proposed)

					// Try offer again
					offer, sumAffinities, err = devAllocator.AssignDevice(req)
					if offer == nil {
						iter.ctx.Logger().Named("binpack").Error("unexpected error, unable to create device offer after considering preemption", "error", err)
						continue OUTER
					}
				}

				// Store the resource
				devAllocator.AddReserved(offer)
				taskResources.Devices = append(taskResources.Devices, offer)

				// Add the scores
				if len(req.Affinities) != 0 {
					for _, a := range req.Affinities {
						totalDeviceAffinityWeight += math.Abs(float64(a.Weight))
					}
					sumMatchingAffinities += sumAffinities
				}
			}

			// Store the task resource
			option.SetTaskResources(task, taskResources)

			// Accumulate the total resource requirement
			total.Tasks[task.Name] = taskResources
		}

		// Store current set of running allocs before adding resources for the task group
		current := proposed

		// Add the resources we are trying to fit
		proposed = append(proposed, &structs.Allocation{AllocatedResources: total})

		// Check if these allocations fit, if they do not, simply skip this node
		fit, dim, util, _ := structs.AllocsFit(option.Node, proposed, netIdx, false)
		netIdx.Release()
		if !fit {
			// Skip the node if evictions are not enabled
			if !iter.evict {
				iter.ctx.Metrics().ExhaustedNode(option.Node, dim)
				continue
			}

			// If eviction is enabled and the node doesn't fit the alloc, check if
			// any allocs can be preempted

			// Initialize preemptor with candidate set
			preemptor.SetCandidates(current)

			preemptedAllocs := preemptor.PreemptForTaskGroup(total)
			allocsToPreempt = append(allocsToPreempt, preemptedAllocs...)

			// If we were unable to find preempted allocs to meet these requirements
			// mark as exhausted and continue
			if len(preemptedAllocs) == 0 {
				iter.ctx.Metrics().ExhaustedNode(option.Node, dim)
				continue
			}
		}
		if len(allocsToPreempt) > 0 {
			option.PreemptedAllocs = allocsToPreempt
		}

		// Score the fit normally otherwise
		fitness := structs.ScoreFit(option.Node, util)
		normalizedFit := fitness / binPackingMaxFitScore
		option.Scores = append(option.Scores, normalizedFit)
		iter.ctx.Metrics().ScoreNode(option.Node, "binpack", normalizedFit)

		// Score the device affinity
		if totalDeviceAffinityWeight != 0 {
			sumMatchingAffinities /= totalDeviceAffinityWeight
			option.Scores = append(option.Scores, sumMatchingAffinities)
			iter.ctx.Metrics().ScoreNode(option.Node, "devices", sumMatchingAffinities)
		}

		return option
	}
}

func (iter *BinPackIterator) Reset() {
	iter.source.Reset()
}

// JobAntiAffinityIterator is used to apply an anti-affinity to allocating
// along side other allocations from this job. This is used to help distribute
// load across the cluster.
type JobAntiAffinityIterator struct {
	ctx          Context
	source       RankIterator
	jobID        string
	taskGroup    string
	desiredCount int
}

// NewJobAntiAffinityIterator is used to create a JobAntiAffinityIterator that
// applies the given penalty for co-placement with allocs from this job.
func NewJobAntiAffinityIterator(ctx Context, source RankIterator, jobID string) *JobAntiAffinityIterator {
	iter := &JobAntiAffinityIterator{
		ctx:    ctx,
		source: source,
		jobID:  jobID,
	}
	return iter
}

func (iter *JobAntiAffinityIterator) SetJob(job *structs.Job) {
	iter.jobID = job.ID
}

func (iter *JobAntiAffinityIterator) SetTaskGroup(tg *structs.TaskGroup) {
	iter.taskGroup = tg.Name
	iter.desiredCount = tg.Count
}

func (iter *JobAntiAffinityIterator) Next() *RankedNode {
	for {
		option := iter.source.Next()
		if option == nil {
			return nil
		}

		// Get the proposed allocations
		proposed, err := option.ProposedAllocs(iter.ctx)
		if err != nil {
			iter.ctx.Logger().Named("job_anti_affinity").Error("failed retrieving proposed allocations", "error", err)
			continue
		}

		// Determine the number of collisions
		collisions := 0
		for _, alloc := range proposed {
			if alloc.JobID == iter.jobID && alloc.TaskGroup == iter.taskGroup {
				collisions += 1
			}
		}

		// Calculate the penalty based on number of collisions
		// TODO(preetha): Figure out if batch jobs need a different scoring penalty where collisions matter less
		if collisions > 0 {
			scorePenalty := -1 * float64(collisions+1) / float64(iter.desiredCount)
			option.Scores = append(option.Scores, scorePenalty)
			iter.ctx.Metrics().ScoreNode(option.Node, "job-anti-affinity", scorePenalty)
		} else {
			iter.ctx.Metrics().ScoreNode(option.Node, "job-anti-affinity", 0)
		}
		return option
	}
}

func (iter *JobAntiAffinityIterator) Reset() {
	iter.source.Reset()
}

// NodeReschedulingPenaltyIterator is used to apply a penalty to
// a node that had a previous failed allocation for the same job.
// This is used when attempting to reschedule a failed alloc
type NodeReschedulingPenaltyIterator struct {
	ctx          Context
	source       RankIterator
	penaltyNodes map[string]struct{}
}

// NewNodeReschedulingPenaltyIterator is used to create a NodeReschedulingPenaltyIterator that
// applies the given scoring penalty for placement onto nodes in penaltyNodes
func NewNodeReschedulingPenaltyIterator(ctx Context, source RankIterator) *NodeReschedulingPenaltyIterator {
	iter := &NodeReschedulingPenaltyIterator{
		ctx:    ctx,
		source: source,
	}
	return iter
}

func (iter *NodeReschedulingPenaltyIterator) SetPenaltyNodes(penaltyNodes map[string]struct{}) {
	iter.penaltyNodes = penaltyNodes
}

func (iter *NodeReschedulingPenaltyIterator) Next() *RankedNode {
	for {
		option := iter.source.Next()
		if option == nil {
			return nil
		}

		_, ok := iter.penaltyNodes[option.Node.ID]
		if ok {
			option.Scores = append(option.Scores, -1)
			iter.ctx.Metrics().ScoreNode(option.Node, "node-reschedule-penalty", -1)
		} else {
			iter.ctx.Metrics().ScoreNode(option.Node, "node-reschedule-penalty", 0)
		}
		return option
	}
}

func (iter *NodeReschedulingPenaltyIterator) Reset() {
	iter.penaltyNodes = make(map[string]struct{})
	iter.source.Reset()
}

// NodeAffinityIterator is used to resolve any affinity rules in the job or task group,
// and apply a weighted score to nodes if they match.
type NodeAffinityIterator struct {
	ctx           Context
	source        RankIterator
	jobAffinities []*structs.Affinity
	affinities    []*structs.Affinity
}

// NewNodeAffinityIterator is used to create a NodeAffinityIterator that
// applies a weighted score according to whether nodes match any
// affinities in the job or task group.
func NewNodeAffinityIterator(ctx Context, source RankIterator) *NodeAffinityIterator {
	return &NodeAffinityIterator{
		ctx:    ctx,
		source: source,
	}
}

func (iter *NodeAffinityIterator) SetJob(job *structs.Job) {
	iter.jobAffinities = job.Affinities
}

func (iter *NodeAffinityIterator) SetTaskGroup(tg *structs.TaskGroup) {
	// Merge job affinities
	if iter.jobAffinities != nil {
		iter.affinities = append(iter.affinities, iter.jobAffinities...)
	}

	// Merge task group affinities and task affinities
	if tg.Affinities != nil {
		iter.affinities = append(iter.affinities, tg.Affinities...)
	}
	for _, task := range tg.Tasks {
		if task.Affinities != nil {
			iter.affinities = append(iter.affinities, task.Affinities...)
		}
	}
}

func (iter *NodeAffinityIterator) Reset() {
	iter.source.Reset()
	// This method is called between each task group, so only reset the merged list
	iter.affinities = nil
}

func (iter *NodeAffinityIterator) hasAffinities() bool {
	return len(iter.affinities) > 0
}

func (iter *NodeAffinityIterator) Next() *RankedNode {
	option := iter.source.Next()
	if option == nil {
		return nil
	}
	if !iter.hasAffinities() {
		iter.ctx.Metrics().ScoreNode(option.Node, "node-affinity", 0)
		return option
	}
	// TODO(preetha): we should calculate normalized weights once and reuse it here
	sumWeight := 0.0
	for _, affinity := range iter.affinities {
		sumWeight += math.Abs(float64(affinity.Weight))
	}

	totalAffinityScore := 0.0
	for _, affinity := range iter.affinities {
		if matchesAffinity(iter.ctx, affinity, option.Node) {
			totalAffinityScore += float64(affinity.Weight)
		}
	}
	normScore := totalAffinityScore / sumWeight
	if totalAffinityScore != 0.0 {
		option.Scores = append(option.Scores, normScore)
		iter.ctx.Metrics().ScoreNode(option.Node, "node-affinity", normScore)
	}
	return option
}

func matchesAffinity(ctx Context, affinity *structs.Affinity, option *structs.Node) bool {
	//TODO(preetha): Add a step here that filters based on computed node class for potential speedup
	// Resolve the targets
	lVal, lOk := resolveTarget(affinity.LTarget, option)
	rVal, rOk := resolveTarget(affinity.RTarget, option)

	// Check if satisfied
	return checkAffinity(ctx, affinity.Operand, lVal, rVal, lOk, rOk)
}

// ScoreNormalizationIterator is used to combine scores from various prior
// iterators and combine them into one final score. The current implementation
// averages the scores together.
type ScoreNormalizationIterator struct {
	ctx    Context
	source RankIterator
}

// NewScoreNormalizationIterator is used to create a ScoreNormalizationIterator that
// averages scores from various iterators into a final score.
func NewScoreNormalizationIterator(ctx Context, source RankIterator) *ScoreNormalizationIterator {
	return &ScoreNormalizationIterator{
		ctx:    ctx,
		source: source}
}

func (iter *ScoreNormalizationIterator) Reset() {
	iter.source.Reset()
}

func (iter *ScoreNormalizationIterator) Next() *RankedNode {
	option := iter.source.Next()
	if option == nil || len(option.Scores) == 0 {
		return option
	}
	numScorers := len(option.Scores)
	sum := 0.0
	for _, score := range option.Scores {
		sum += score
	}
	option.FinalScore = sum / float64(numScorers)
	//TODO(preetha): Turn map in allocmetrics into a heap of topK scores
	iter.ctx.Metrics().ScoreNode(option.Node, "normalized-score", option.FinalScore)
	return option
}
