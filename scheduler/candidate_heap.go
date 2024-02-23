package scheduler

import (
	"math"
	"strings"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler/fixedheap"
)

type CandidateHeap struct {
	fixedheap.FixedHeap[Candidate]
}

func NewCandidateHeap(max int) *CandidateHeap {
	if max < 1 {
		return nil
	}
	heap := CandidateHeap{fixedheap.NewFixedHeap[Candidate](max,
		func(a, b Candidate) bool { return a.Index < b.Index })}
	return &heap
}

func (h CandidateHeap) String() string {
	ids := helper.ConvertSlice(h.Slice(), func(a Candidate) string {
		if a.Alloc == nil {
			return "<nil>"
		}
		return a.Alloc.ID + " [" + a.Status.String() + "]"
	})
	return "[" + strings.Join(ids, ", ") + "]"
}

type indexFunc func(Candidate) bitfield

func genericIndexFuncNoDeploy(jobVersion uint64) indexFunc {
	return func(c Candidate) bitfield {
		b := bitfield(0)
		if c.Alloc == nil {
			return b
		}
		if c.JobVersion == jobVersion {
			b.set(63)
		}
		switch c.Alloc.ClientStatus {
		case structs.AllocClientStatusComplete:
			b.set(62)
		case structs.AllocClientStatusRunning:
			b.set(61)
		case structs.AllocClientStatusPending:
			b.set(60)
		case structs.AllocClientStatusUnknown:
			b.set(59)
		case structs.AllocClientStatusFailed:
			b.set(58)
		case structs.AllocClientStatusLost:
			b.set(57)
		}

		// note: we have up to 48 bits we can set above
		// TODO: this truncates the alloc index!
		b.mergeUint16(flipAllocIndex(uint16(c.AllocIndex)))
		return b
	}
}

// flipAllocIndex reverses an AllocIndex uint16 so that the highest # is lowest
// priority to keep
func flipAllocIndex(x uint16) uint16 {
	return x ^ math.MaxUint16
}
