// +build !ent

package scheduler

func NewPreemptionScoringIterator(ctx Context, source RankIterator) RankIterator {
	return source
}
