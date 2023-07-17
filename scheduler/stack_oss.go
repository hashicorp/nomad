//go:build !ent
// +build !ent

package scheduler

func NewQuotaIterator(_ Context, source FeasibleIterator) FeasibleIterator {
	return source
}
