package scheduler

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

// Rank is used to provide a score and various ranking metadata
// along with a node when iterating. This state can be modified as
// various rank methods are applied.
type RankedNode struct {
	Node  *structs.Node
	Score float64

	// Allocs is used to cache the proposed allocations on the
	// node. This can be shared between iterators that require it.
	Proposed []*structs.Allocation
}

func (r *RankedNode) GoString() string {
	return fmt.Sprintf("<Node: %s Score: %0.3f>", r.Node.ID, r.Score)
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
	resources *structs.Resources
	evict     bool
	priority  int
}

// NewBinPackIterator returns a BinPackIterator which tries to fit the given
// resources, potentially evicting other tasks based on a given priority.
func NewBinPackIterator(ctx Context, source RankIterator, resources *structs.Resources, evict bool, priority int) *BinPackIterator {
	iter := &BinPackIterator{
		ctx:       ctx,
		source:    source,
		resources: resources,
		evict:     evict,
		priority:  priority,
	}
	return iter
}

func (iter *BinPackIterator) SetResources(r *structs.Resources) {
	iter.resources = r
}

func (iter *BinPackIterator) SetPriority(p int) {
	iter.priority = p
}

func (iter *BinPackIterator) Next() *RankedNode {
	for {
		// Get the next potential option
		option := iter.source.Next()
		if option == nil {
			return nil
		}
		nodeID := option.Node.ID

		// Get the proposed allocations
		var proposed []*structs.Allocation
		if option.Proposed != nil {
			proposed = option.Proposed
		} else {
			p, err := iter.ctx.ProposedAllocs(nodeID)
			if err != nil {
				iter.ctx.Logger().Printf("[ERR] sched.binpack: failed to get proposed allocations for '%s': %v",
					nodeID, err)
				continue
			}
			proposed = p
			option.Proposed = p
		}

		// Add the resources we are trying to fit
		proposed = append(proposed, &structs.Allocation{Resources: iter.resources})

		// Check if these allocations fit, if they do not, simply skip this node
		fit, util, _ := structs.AllocsFit(option.Node, proposed)
		if !fit {
			iter.ctx.Metrics().ExhaustedNode(option.Node)
			continue
		}

		// XXX: For now we completely ignore evictions. We should use that flag
		// to determine if its possible to evict other lower priority allocations
		// to make room. This explodes the search space, so it must be done
		// carefully.

		// Score the fit normally otherwise
		fitness := structs.ScoreFit(option.Node, util)
		option.Score += fitness
		iter.ctx.Metrics().ScoreNode(option.Node, "binpack", fitness)
		return option
	}
}

func (iter *BinPackIterator) Reset() {
	iter.source.Reset()
}
