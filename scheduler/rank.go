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
	TaskResources map[string]*structs.Resources

	// Allocs is used to cache the proposed allocations on the
	// node. This can be shared between iterators that require it.
	Proposed []*structs.Allocation
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
	resource *structs.Resources) {
	if r.TaskResources == nil {
		r.TaskResources = make(map[string]*structs.Resources)
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
			iter.ctx.Logger().Printf(
				"[ERR] sched.binpack: failed to get proposed allocations: %v",
				err)
			continue
		}

		// Index the existing network usage
		netIdx := structs.NewNetworkIndex()
		netIdx.SetNode(option.Node)
		netIdx.AddAllocs(proposed)

		// Assign the resources for each task
		total := &structs.Resources{
			DiskMB: iter.taskGroup.EphemeralDisk.SizeMB,
		}
		for _, task := range iter.taskGroup.Tasks {
			taskResources := task.Resources.Copy()

			// Check if we need a network resource
			if len(taskResources.Networks) > 0 {
				ask := taskResources.Networks[0]
				offer, err := netIdx.AssignNetwork(ask)
				if offer == nil {
					iter.ctx.Metrics().ExhaustedNode(option.Node,
						fmt.Sprintf("network: %s", err))
					netIdx.Release()
					continue OUTER
				}

				// Reserve this to prevent another task from colliding
				netIdx.AddReserved(offer)

				// Update the network ask to the offer
				taskResources.Networks = []*structs.NetworkResource{offer}
			}

			// Store the task resource
			option.SetTaskResources(task, taskResources)

			// Accumulate the total resource requirement
			total.Add(taskResources)
		}

		// Add the resources we are trying to fit
		proposed = append(proposed, &structs.Allocation{Resources: total})

		// Check if these allocations fit, if they do not, simply skip this node
		fit, dim, util, _ := structs.AllocsFit(option.Node, proposed, netIdx)
		netIdx.Release()
		if !fit {
			iter.ctx.Metrics().ExhaustedNode(option.Node, dim)
			continue
		}

		// XXX: For now we completely ignore evictions. We should use that flag
		// to determine if its possible to evict other lower priority allocations
		// to make room. This explodes the search space, so it must be done
		// carefully.

		// Score the fit normally otherwise
		fitness := structs.ScoreFit(option.Node, util)
		normalizedFit := float64(fitness) / float64(binPackingMaxFitScore)
		option.Scores = append(option.Scores, normalizedFit)
		iter.ctx.Metrics().ScoreNode(option.Node, "binpack", normalizedFit)
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
			iter.ctx.Logger().Printf(
				"[ERR] sched.job-anti-aff: failed to get proposed allocations: %v",
				err)
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
			scorePenalty := -1 * float64(collisions) / float64(iter.desiredCount)
			option.Scores = append(option.Scores, scorePenalty)
			iter.ctx.Metrics().ScoreNode(option.Node, "job-anti-affinity", scorePenalty)
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
		}
		return option
	}
}

func (iter *NodeReschedulingPenaltyIterator) Reset() {
	iter.penaltyNodes = make(map[string]struct{})
	iter.source.Reset()
}

type NodeAffinityIterator struct {
	ctx           Context
	source        RankIterator
	jobAffinities []*structs.Affinity
	affinities    []*structs.Affinity
}

func NewNodeAffinityIterator(ctx Context, source RankIterator) *NodeAffinityIterator {
	return &NodeAffinityIterator{
		ctx:    ctx,
		source: source,
	}
}

func (iter *NodeAffinityIterator) SetJob(job *structs.Job) {
	if job.Affinities != nil {
		iter.jobAffinities = job.Affinities
	}
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
	if len(iter.affinities) == 0 {
		return option
	}
	// TODO(preetha): we should calculate normalized weights once and reuse it here
	sumWeight := 0.0
	for _, affinity := range iter.affinities {
		sumWeight += math.Abs(affinity.Weight)
	}

	totalAffinityScore := 0.0
	for _, affinity := range iter.affinities {
		if matchesAffinity(iter.ctx, affinity, option.Node) {
			normScore := affinity.Weight / sumWeight
			totalAffinityScore += normScore
		}
	}
	if totalAffinityScore != 0.0 {
		option.Scores = append(option.Scores, totalAffinityScore)
		iter.ctx.Metrics().ScoreNode(option.Node, "node-affinity", totalAffinityScore)
	}
	return option
}

func matchesAffinity(ctx Context, affinity *structs.Affinity, option *structs.Node) bool {
	// Resolve the targets
	lVal, ok := resolveTarget(affinity.LTarget, option)
	if !ok {
		return false
	}
	rVal, ok := resolveTarget(affinity.RTarget, option)
	if !ok {
		return false
	}

	// Check if satisfied
	return checkAffinity(ctx, affinity.Operand, lVal, rVal)
}

type ScoreNormalizationIterator struct {
	ctx    Context
	source RankIterator
}

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
	if option == nil {
		return nil
	}
	numScorers := len(option.Scores)
	if numScorers > 0 {
		sum := 0.0
		for _, score := range option.Scores {
			sum += score
		}
		option.FinalScore = sum / float64(numScorers)
	}
	//TODO(preetha): Turn map in allocmetrics into a heap of topK scores
	iter.ctx.Metrics().ScoreNode(option.Node, "normalized-score", option.FinalScore)
	return option
}
