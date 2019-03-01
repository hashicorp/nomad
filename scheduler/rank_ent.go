// +build pro ent

package scheduler

// PreemptionScoringIterator is used to score nodes according to the
// combination of preemptible allocations in them
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

		// preemption score is inversly proportional to netPriority
		preemptionScore := 1.0 / float64(netPriority)
		option.Scores = append(option.Scores, preemptionScore)
		iter.ctx.Metrics().ScoreNode(option.Node, "preemption-score", preemptionScore)
	}
	return option
}
