package reconciler

import (
	"strings"
	"time"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler/fixedheap"
	sstructs "github.com/hashicorp/nomad/scheduler/structs"
)

type CandidateHeap struct {
	fixedheap.FixedHeap[Candidate]
}

func NewCandidateHeap(max int) *CandidateHeap {
	if max < 1 {
		return nil
	}
	heap := CandidateHeap{fixedheap.NewFixedHeap[Candidate](max,
		func(a, b Candidate) bool {
			if a.Index == b.Index {
				return a.AllocIndex < b.AllocIndex
			}
			return a.Index < b.Index
		})}
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

type indexFunc func(*Candidate) (CandidateStatus, bitfield)

func genericIndexFuncNoDeploy(tg *structs.TaskGroup, targetVersion uint64, now time.Time) indexFunc {
	return func(c *Candidate) (CandidateStatus, bitfield) {
		b := bitfield(0)
		status := CandidateStatusIgnore
		if c.Alloc == nil {
			c.Status = status
			c.Index = b
			return status, b
		}
		if c.JobVersion == targetVersion {
			b = b | BitJobVersionsMatch
		} else if c.Status != CandidateStatusDestructiveUpdate {
			status = CandidateStatusInplaceUpdate
			c.NewDescription = sstructs.StatusAllocInPlace
			b = b | BitJobNondestructive
		} else {
			status = CandidateStatusDestructiveUpdate
			c.NewDescription = sstructs.StatusAllocUpdating
		}

		switch c.Alloc.ClientStatus {
		case structs.AllocClientStatusComplete:
			b = b | BitAllocComplete
		case structs.AllocClientStatusRunning:
			b = b | BitAllocRunning
		case structs.AllocClientStatusPending:
			b = b | BitAllocPending

		case structs.AllocClientStatusFailed:
			status = CandidateStatusStop
			b = b | BitAllocFailed

			// TODO: how do we annotate the reschedule time?
			now, later, _ := updateByReschedulable(c.Alloc, now, c.Alloc.EvalID, nil, false)
			if now {
				status = CandidateStatusNeedsRescheduleNow
				c.NewDescription = sstructs.StatusAllocRescheduled
				b = b | BitRescheduleNow
			} else if later {
				status = CandidateStatusNeedsRescheduleLater
				b = b | BitRescheduleLater
			}

		case structs.AllocClientStatusUnknown:
			b = b | BitAllocUnknown

		case structs.AllocClientStatusLost:
			status = CandidateStatusOnDownNode
			b = b | BitAllocLost
		}

		if c.Node != nil {
			if c.Node.DrainStrategy != nil {
				status = CandidateStatusOnDrainingNode
				c.NewDescription = sstructs.StatusAllocMigrating
				b = b | BitNodeDraining
			}
			switch c.Node.Status {
			case structs.NodeStatusDown:
				status = CandidateStatusOnDownNode
				c.NewDescription = sstructs.StatusAllocLost
				b = b | BitNodeDown
			case structs.NodeStatusDisconnected:
				status = CandidateStatusOnDisconnectingNode
				c.NewDescription = sstructs.StatusAllocUnknown
				b = b | BitNodeDisconnected
			default:
				b = b | BitNodeIsUntainted
			}
		} else {
			b = b | BitNodeIsUntainted
		}

		c.Status = status
		c.Index = b

		return status, b
	}
}
