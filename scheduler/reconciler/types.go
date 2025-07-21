package reconciler

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

type CandidateStatus int

const (
	CandidateStatusIgnore CandidateStatus = iota
	CandidateStatusInplaceUpdate
	CandidateStatusPlace
	CandidateStatusStop
	CandidateStatusClientTerminal
	CandidateStatusNeedsRescheduleNow
	CandidateStatusNeedsRescheduleLater
	CandidateStatusDestructiveUpdate
	CandidateStatusPlaceCanary
	CandidateStatusOnDownNode
	CandidateStatusOnDrainingNode
	CandidateStatusOnDisconnectingNode
	CandidateStatusAwaitingReconnect
)

func (c CandidateStatus) String() string {
	switch c {
	case CandidateStatusIgnore:
		return "ignore"
	case CandidateStatusInplaceUpdate:
		return "inplace-update"
	case CandidateStatusPlace:
		return "place"
	case CandidateStatusStop:
		return "stop"
	case CandidateStatusClientTerminal:
		return "client-terminal"
	case CandidateStatusNeedsRescheduleNow:
		return "reschedule-now"
	case CandidateStatusNeedsRescheduleLater:
		return "reschedule-later"
	case CandidateStatusDestructiveUpdate:
		return "destructive-update"
	case CandidateStatusPlaceCanary:
		return "place-canary"
	case CandidateStatusOnDownNode:
		return "on-down-node"
	case CandidateStatusOnDrainingNode:
		return "on-draining-node"
	case CandidateStatusOnDisconnectingNode:
		return "on-disconnecting-node"
	case CandidateStatusAwaitingReconnect:
		return "on-reconnecting-node"
	default:
		return fmt.Sprint(int(c))
	}
}

type Candidate struct {
	Alloc          *structs.Allocation
	Node           *structs.Node
	Status         CandidateStatus
	NewDescription string
	Index          bitfield
	AllocIndex     uint
	JobVersion     uint64
}

func (c *Candidate) String() string {
	if c == nil {
		return "<nil>"
	}
	if c.Alloc == nil {
		return fmt.Sprintf("alloc=<nil> [status=%s]", c.Status)
	}
	return fmt.Sprintf("alloc=%s [status=%s]", c.Alloc.ID, c.Status)
}

func NewCandidate(alloc *structs.Allocation, node *structs.Node, job *structs.Job, tg *structs.TaskGroup, indexFn indexFunc) *Candidate {
	if alloc == nil {
		alloc = &structs.Allocation{
			ID:            uuid.Generate(),
			JobID:         job.ID,
			Job:           job,
			TaskGroup:     tg.Name,
			DesiredStatus: structs.AllocDesiredStatusRun,
			ClientStatus:  structs.AllocClientStatusPending,
			RescheduleTracker: &structs.RescheduleTracker{
				Events: []*structs.RescheduleEvent{},
			},
		}
	}

	candidate := &Candidate{
		Alloc: alloc,
		Node:  node,
		//		Status:     status,
		AllocIndex: convertAllocNameToIndex(alloc.Name),
		JobVersion: job.Version,
	}
	indexFn(candidate)

	return candidate
}

var (
	// AllocationIndexRegex is a regular expression to find the allocation index.
	findAllocationIndexRegex = regexp.MustCompile(".+\\[(\\d+)\\]$")
)

// convertAllocNameToIndex returns the index of the allocation.
func convertAllocNameToIndex(name string) uint {
	matches := findAllocationIndexRegex.FindStringSubmatch(name)
	if len(matches) != 2 {
		return 0
	}

	index, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}

	return uint(index)
}

func NewReplacementCandidate(alloc *structs.Allocation, evalID string, jobVersion uint64) Candidate {
	newAlloc := &structs.Allocation{
		ID:                uuid.Generate(),
		EvalID:            evalID,
		JobID:             alloc.Job.ID,
		DesiredStatus:     structs.AllocDesiredStatusRun,
		ClientStatus:      structs.AllocClientStatusPending,
		RescheduleTracker: alloc.RescheduleTracker.Copy(),
		Name:              alloc.Name,
	}

	candidate := Candidate{
		Alloc:      newAlloc,
		Status:     CandidateStatusPlace,
		JobVersion: jobVersion,
		AllocIndex: convertAllocNameToIndex(alloc.Name),
	}

	return candidate
}
