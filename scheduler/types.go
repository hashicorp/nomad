package scheduler

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

type allocStateCounts struct {
	pending  int
	running  int
	complete int
	failed   int
	lost     int
	unknown  int
}

type Plan struct {
	previous *CandidateHeap
	current  *CandidateHeap
	discard  *CandidateHeap

	evals []*structs.Evaluation
}

type NodeStatus int

const (
	NodeStatusUnplaced NodeStatus = iota
	NodeStatusReady
	NodeStatusDown
	NodeStatusDraining
	NodeStatusDisconnected
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
	Alloc      *structs.Allocation
	Status     CandidateStatus
	Index      bitfield
	AllocIndex uint
	JobVersion uint64
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

func NewCandidate(alloc *structs.Allocation, job *structs.Job, tg *structs.TaskGroup, indexFn func(Candidate) bitfield) Candidate {
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
			Name: alloc.Name,
		}
	}

	candidate := Candidate{
		Alloc:      alloc,
		Status:     CandidateStatusFromAlloc(alloc, tg, job),
		AllocIndex: allocNameToIndex(alloc.Name),
		JobVersion: job.Version,
	}
	candidate.Index = indexFn(candidate)
	return candidate
}

var (
	// AllocationIndexRegex is a regular expression to find the allocation index.
	allocationIndexRegex = regexp.MustCompile(".+\\[(\\d+)\\]$")
)

// allocNameToIndex returns the index of the allocation.
func allocNameToIndex(name string) uint {
	matches := allocationIndexRegex.FindStringSubmatch(name)
	if len(matches) != 2 {
		return 0
	}

	index, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}

	return uint(index)
}

func NewCandidateReplacement(alloc *structs.Allocation, eval *structs.Evaluation, jobVersion uint64, nodeID string, index bitfield) Candidate {
	newAlloc := &structs.Allocation{
		ID:            uuid.Generate(),
		JobID:         eval.JobID,
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
		NodeID:        nodeID,
		RescheduleTracker: &structs.RescheduleTracker{
			Events: []*structs.RescheduleEvent{},
		},
		Name: alloc.Name,
	}

	candidate := Candidate{
		Alloc:      newAlloc,
		Status:     CandidateStatusPlace,
		Index:      index,
		JobVersion: jobVersion,
		AllocIndex: allocNameToIndex(alloc.Name),
	}

	return candidate
}

func CandidateStatusFromAlloc(alloc *structs.Allocation, tg *structs.TaskGroup, job *structs.Job) CandidateStatus {
	if alloc == nil {
		return CandidateStatusPlace // TODO: always?
	}
	if alloc.Job == nil {
		return CandidateStatusPlace // TODO: always?
	}

	if alloc.DesiredStatus == structs.AllocDesiredStatusStop {
		if alloc.ClientTerminalStatus() {
			return CandidateStatusClientTerminal
		}
		return CandidateStatusStop
	}
	if alloc.ClientStatus == structs.AllocClientStatusComplete {
		return CandidateStatusStop
	}

	switch alloc.ClientStatus {
	case structs.AllocClientStatusRunning, structs.AllocClientStatusPending:

	case structs.AllocClientStatusFailed:
		if alloc.Job.Version == job.Version &&
			len(alloc.RescheduleTracker.Events) > tg.ReschedulePolicy.Attempts {
			// TODO(tgross): very prototype-y
			return CandidateStatusNeedsRescheduleNow // TODO: now vs later
		} else {
			return CandidateStatusClientTerminal
		}
	case structs.AllocClientStatusLost:
	case structs.AllocClientStatusUnknown:
	}

	if alloc.Job.Version != job.Version {
		// TODO: fix this
		//		if eval.WasDestructive {
		return CandidateStatusDestructiveUpdate
		// } else {
		// 	return CandidateStatusInplaceUpdate
		// }
	}

	// nodeStatus := NodeStatusUnplaced
	// if alloc.Node != nil {
	// 	nodeStatus = alloc.Node.Status
	// }
	// x := ex{nodeStatus, alloc.ClientStatus, alloc.DesiredStatus}

	// switch x {
	// case ex{NodeStatusDown, AllocClientStatusComplete, AllocDesiredStatusStop}:
	// 	return CandidateStatusClientTerminal
	// case ex{NodeStatusDown, AllocClientStatusComplete, AllocDesiredStatusRun}:
	// 	return CandidateStatusStop
	// case ex{NodeStatusDown, AllocClientStatusComplete, AllocDesiredStatusStop}:

	// }

	return CandidateStatusIgnore
}
