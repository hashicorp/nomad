// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

// LimitIterator is a RankIterator used to limit the number of options
// that are returned before we artificially end the stream.
type LimitIterator struct {
	ctx              Context
	source           RankIterator
	limit            int
	maxSkip          int
	scoreThreshold   float64
	seen             int
	skippedNodes     []*RankedNode
	skippedNodeIndex int
}

// NewLimitIterator returns a LimitIterator with a fixed limit of returned options.
// Up to maxSkip options whose score is below scoreThreshold are skipped
// if there are additional options available in the source iterator
func NewLimitIterator(ctx Context, source RankIterator, limit int, scoreThreshold float64, maxSkip int) *LimitIterator {
	iter := &LimitIterator{
		ctx:            ctx,
		source:         source,
		limit:          limit,
		maxSkip:        maxSkip,
		scoreThreshold: scoreThreshold,
		skippedNodes:   make([]*RankedNode, 0, maxSkip),
	}
	return iter
}

func (iter *LimitIterator) SetLimit(limit int) {
	iter.limit = limit
}

func (iter *LimitIterator) Next() *RankedNode {
	if iter.seen == iter.limit {
		return nil
	}
	option := iter.nextOption()
	if option == nil {
		return nil
	}

	if len(iter.skippedNodes) < iter.maxSkip {
		// Try skipping ahead up to maxSkip to find an option with score lesser than the threshold
		for option != nil && option.FinalScore <= iter.scoreThreshold && len(iter.skippedNodes) < iter.maxSkip {
			iter.skippedNodes = append(iter.skippedNodes, option)
			option = iter.source.Next()
		}
	}
	iter.seen += 1
	if option == nil { // Didn't find anything, so use the skipped nodes instead
		return iter.nextOption()
	}
	return option
}

// nextOption uses the iterator's list of skipped nodes if the source iterator is exhausted
func (iter *LimitIterator) nextOption() *RankedNode {
	sourceOption := iter.source.Next()
	if sourceOption == nil && iter.skippedNodeIndex < len(iter.skippedNodes) {
		skippedOption := iter.skippedNodes[iter.skippedNodeIndex]
		iter.skippedNodeIndex += 1
		return skippedOption
	}
	return sourceOption
}

func (iter *LimitIterator) Reset() {
	iter.source.Reset()
	iter.seen = 0
	iter.skippedNodes = make([]*RankedNode, 0, iter.maxSkip)
	iter.skippedNodeIndex = 0
}

// MaxScoreIterator is a RankIterator used to return only a single result
// of the item with the highest score. This iterator will consume all of the
// possible inputs and only returns the highest ranking result.
type MaxScoreIterator struct {
	ctx    Context
	source RankIterator
	max    *RankedNode
}

// NewMaxScoreIterator returns a MaxScoreIterator over the given source
func NewMaxScoreIterator(ctx Context, source RankIterator) *MaxScoreIterator {
	iter := &MaxScoreIterator{
		ctx:    ctx,
		source: source,
	}
	return iter
}

func (iter *MaxScoreIterator) Next() *RankedNode {
	// Check if we've found the max, return nil
	if iter.max != nil {
		return nil
	}

	// Consume and determine the max
	for {
		option := iter.source.Next()
		if option == nil {
			return iter.max
		}

		if iter.max == nil || option.FinalScore > iter.max.FinalScore {
			iter.max = option
		}
	}
}

func (iter *MaxScoreIterator) Reset() {
	iter.source.Reset()
	iter.max = nil
}
