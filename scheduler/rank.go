package scheduler

import (
	"math"

	"github.com/hashicorp/nomad/nomad/structs"
)

// Rank is used to provide a score and various ranking metadata
// along with a node when iterating. This state can be modified as
// various rank methods are applied.
type RankedNode struct {
	Node  *structs.Node
	Score float64
}

// RankFeasibleIterator is used to iteratively yield nodes along
// with ranking metadata. The iterators may manage some state for
// performance optimizations.
type RankIterator interface {
	Next() *RankedNode
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

// StaticRankIterator is a RankIterator that returns a static set of results.
// This is largely only useful for testing.
type StaticRankIterator struct {
	ctx    Context
	nodes  []*RankedNode
	offset int
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
	if iter.offset == len(iter.nodes) {
		return nil
	}

	// Return the next offset
	offset := iter.offset
	iter.offset += 1
	return iter.nodes[offset]
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

func (iter *BinPackIterator) Next() *RankedNode {
	for {
		option := iter.source.Next()
		if option == nil {
			return nil
		}

		// TODO: Evaluate the bin packing
		return option
	}
}

// scoreFit is used to score the fit based on the Google work published here:
// http://www.columbia.edu/~cs2035/courses/ieor4405.S13/datacenter_scheduling.ppt
// This is equivalent to their BestFit v3
func scoreFit(node *structs.Node, util *structs.Resources) float64 {
	// Determine the node availability
	nodeCpu := node.Resources.CPU
	if node.Reserved != nil {
		nodeCpu -= node.Reserved.CPU
	}
	nodeMem := float64(node.Resources.MemoryMB)
	if node.Reserved != nil {
		nodeMem -= float64(node.Reserved.MemoryMB)
	}

	// Compute the free percentage
	freePctCpu := 1 - (util.CPU / nodeCpu)
	freePctRam := 1 - (float64(util.MemoryMB) / nodeMem)

	// Total will be "maximized" the smaller the value is.
	// At 100% utilization, the total is 2, while at 0% util it is 20.
	total := math.Pow(10, freePctCpu) + math.Pow(10, freePctRam)

	// Invert so that the "maximized" total represents a high-value
	// score. Because the floor is 20, we simply use that as an anchor.
	// This means at a perfect fit, we return 18 as the score.
	score := 20.0 - total

	// Bound the score, just in case
	// If the score is over 18, that means we've overfit the node.
	if score > 18.0 {
		score = 18.0
	} else if score < 0 {
		score = 0
	}
	return score
}
