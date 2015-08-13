package scheduler

import "github.com/hashicorp/nomad/nomad/structs"

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
