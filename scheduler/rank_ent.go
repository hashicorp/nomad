// +build pro ent

package scheduler

import "math"

const rate = 0.0048
const origin = 2048.0

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
		preemptionScore := preemptionScore(netPriority)
		option.Scores = append(option.Scores, preemptionScore)
		iter.ctx.Metrics().ScoreNode(option.Node, "preemption-score", preemptionScore)
	}
	return option
}

// preemptionScore is calculated using a logistic function
// see https://www.desmos.com/calculator/alaeiuaiey for a visual representation of the curve.
// Lower values of netPriority get a score closer to 1 and the inflection point is around 1500
func preemptionScore(netPriority int) float64 {
	// This function manifests as an s curve that asympotically moves towards zero for large values of netPriority
	return 1.0 / (1 + math.Exp(rate*(float64(netPriority)-origin)))
}
