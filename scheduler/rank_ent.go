// +build pro ent

package scheduler

import (
	"math"

	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// rate is the decay parameter of the logistic function used in scoring preemption options
	rate = 0.0048

	// origin controls the inflection point of the logistic function used in scoring preemption options
	origin = 2048.0
)

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
		source: source,
	}
}

func (iter *PreemptionScoringIterator) Reset() {
	iter.source.Reset()
}

func (iter *PreemptionScoringIterator) Next() *RankedNode {
	option := iter.source.Next()
	if option == nil || option.PreemptedAllocs == nil {
		return option
	}

	netPriority := netPriority(option.PreemptedAllocs)
	// preemption score is inversely proportional to netPriority
	preemptionScore := preemptionScore(netPriority)
	option.Scores = append(option.Scores, preemptionScore)
	iter.ctx.Metrics().ScoreNode(option.Node, "preemption", preemptionScore)

	return option
}

// netPriority is a scoring heuristic that represents a combination of two factors.
// First factor is the max priority in the set of allocations, with
// an additional factor that takes into account the individual priorities of allocations
func netPriority(allocs []*structs.Allocation) float64 {
	sumPriority := 0
	max := 0.0
	for _, alloc := range allocs {
		if float64(alloc.Job.Priority) > max {
			max = float64(alloc.Job.Priority)
		}
		sumPriority += alloc.Job.Priority
	}
	// We use the maximum priority across all allocations
	// with an additional penalty that increases proportional to the
	// ratio of the sum by max
	// This ensures that we penalize nodes that have a low max but a high
	// number of preemptible allocations
	ret := max + (float64(sumPriority) / max)
	return ret
}

// preemptionScore is calculated using a logistic function
// see https://www.desmos.com/calculator/alaeiuaiey for a visual representation of the curve.
// Lower values of netPriority get a score closer to 1 and the inflection point is around 1500
func preemptionScore(netPriority float64) float64 {
	// This function manifests as an s curve that asympotically moves towards zero for large values of netPriority
	return 1.0 / (1 + math.Exp(rate*(netPriority-origin)))
}
