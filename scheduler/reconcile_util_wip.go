package scheduler

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (a allocSet) XXXfilterByRescheduleable(isBatch, isDisconnecting bool, now time.Time, evalID string, deployment *structs.Deployment) (allocSet, allocSet, []*delayedRescheduleInfo) {
	untainted := make(map[string]*structs.Allocation)
	rescheduleNow := make(map[string]*structs.Allocation)
	rescheduleLater := []*delayedRescheduleInfo{}

	fmt.Printf("[*] EvalID: %q\n", evalID)

	for _, alloc := range a {
		fmt.Printf("[*] checking alloc %q (%s/%s)\n\tPrev: %q Next: %q\n\tFollowupEvalID: %q\n\tRT: %v\n",
			alloc.ID, alloc.DesiredStatus, alloc.ClientStatus,
			alloc.PreviousAllocation, alloc.NextAllocation,
			alloc.FollowupEvalID,
			alloc.RescheduleTracker,
		)

		// Ignore disconnecting allocs that are already unknown. This can happen
		// in the case of canaries that are interrupted by a disconnect.
		if isDisconnecting && alloc.ClientStatus == structs.AllocClientStatusUnknown {
			continue
		}

		var eligibleNow, eligibleLater bool
		var rescheduleTime time.Time

		// Ignore failing allocs that have already been rescheduled.
		// Only failed or disconnecting allocs should be rescheduled.
		// Protects against a bug allowing rescheduling running allocs.
		if alloc.NextAllocation != "" && alloc.TerminalStatus() {
			// DEBUG
			fmt.Printf("[*] ignoring terminal alloc %q\n", alloc.ID)
			continue
		}

		isUntainted, ignore := shouldFilterX(alloc, isBatch)
		if isUntainted && !isDisconnecting {
			fmt.Printf("[*] isUntainted alloc %q\n", alloc.ID)
			untainted[alloc.ID] = alloc
		}

		if ignore {
			fmt.Printf("[*] ignoring alloc %q\n", alloc.ID)
			continue
		}

		eligibleNow, eligibleLater, rescheduleTime = updateByReschedulableX(alloc, now, evalID, deployment, isDisconnecting)
		if eligibleNow {
			rescheduleNow[alloc.ID] = alloc
			fmt.Printf("[*] rescheduleNow alloc %q\n", alloc.ID)
			continue
		}

		// If the failed alloc is not eligible for rescheduling now we
		// add it to the untainted set.
		untainted[alloc.ID] = alloc

		if eligibleLater {
			rescheduleLater = append(rescheduleLater, &delayedRescheduleInfo{alloc.ID, alloc, rescheduleTime})
			fmt.Printf("[*] rescheduleLater alloc %q\n", alloc.ID)
		}

	}
	return untainted, rescheduleNow, rescheduleLater
}

func shouldFilterX(alloc *structs.Allocation, isBatch bool) (untainted, ignore bool) {

	// Allocs from batch jobs should be filtered when the desired status
	// is terminal and the client did not finish or when the client
	// status is failed so that they will be replaced. If they are
	// complete but not failed, they shouldn't be replaced.
	if isBatch {
		switch alloc.DesiredStatus {
		case structs.AllocDesiredStatusStop:
			if alloc.RanSuccessfully() {
				return true, false
			}
			return false, true
		case structs.AllocDesiredStatusEvict:
			return false, true
		}

		switch alloc.ClientStatus {
		case structs.AllocClientStatusFailed:
			return false, false
		}

		return true, false
	}

	// Handle service jobs
	switch alloc.DesiredStatus {
	case structs.AllocDesiredStatusStop, structs.AllocDesiredStatusEvict:

		if alloc.NextAllocation == "" {
			return false, false
		}

		return false, true
	}

	switch alloc.ClientStatus {
	case structs.AllocClientStatusComplete, structs.AllocClientStatusLost:
		return false, true
	}
	return false, false
}

func updateByReschedulableX(alloc *structs.Allocation, now time.Time, evalID string, d *structs.Deployment, isDisconnecting bool) (rescheduleNow, rescheduleLater bool, rescheduleTime time.Time) {
	// If the allocation is part of an ongoing active deployment, we only allow it to reschedule
	// if it has been marked eligible
	if d != nil && alloc.DeploymentID == d.ID && d.Active() && !alloc.DesiredTransition.ShouldReschedule() {
		return
	}

	// Check if the allocation is marked as it should be force rescheduled
	if alloc.DesiredTransition.ShouldForceReschedule() {
		rescheduleNow = true
	}

	// Reschedule if the eval ID matches the alloc's followup evalID or if its close to its reschedule time
	var eligible bool
	switch {
	case isDisconnecting:
		rescheduleTime, eligible = alloc.RescheduleTimeOnDisconnect(now)

	case alloc.ClientStatus == structs.AllocClientStatusUnknown && alloc.FollowupEvalID == evalID:
		lastDisconnectTime := alloc.LastUnknown()
		rescheduleTime, eligible = alloc.NextRescheduleTimeByTime(lastDisconnectTime)

	default:
		rescheduleTime, eligible = alloc.NextRescheduleTime()
		fmt.Printf("eligible: %v\n", eligible)
	}

	if eligible && (alloc.FollowupEvalID == evalID || rescheduleTime.Sub(now) <= rescheduleWindowSize) {
		rescheduleNow = true
		return
	}

	if eligible && (alloc.FollowupEvalID == "" || isDisconnecting) {
		rescheduleLater = true
	}

	return
}
