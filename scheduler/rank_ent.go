// +build ent

package scheduler

// PreemptionScoringIterator is used to add a
type PreemptionScoringIterator struct {
	ctx    Context
	source RankIterator
}

// PreemptionScoringIterator is used to create a score based on net aggregate priority
// of preempted allocations
func NewPreemptionScoringIterator(ctx Context, source RankIterator) RankIterator {
	return &PreemptionScoringIterator{
		ctx:    ctx,
		source: source}
}

func (iter *PreemptionScoringIterator) Reset() {
	iter.source.Reset()
}

func (iter *PreemptionScoringIterator) Next() *RankedNode {
	option := iter.source.Next()
	if option == nil {
		return option
	}
	if option.PreemptedAllocs != nil {
		netPriority := netAggregatePriority(option.PreemptedAllocs)

		// The max score of 1 is when the net priority is equal to the min across all options
		minNetPriority := iter.ctx.Metrics().PreemptedMinNetPriority
		preemptionScore := float64(minNetPriority) / float64(netPriority)
		option.Scores = append(option.Scores, preemptionScore)
		iter.ctx.Metrics().ScoreNode(option.Node, "preemption-score", preemptionScore)
	}
	return option
}
