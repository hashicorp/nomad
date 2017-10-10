// +build !ent

package scheduler

func NewQuotaIterator(ctx Context, source FeasibleIterator) FeasibleIterator {
	return source
}
