package scheduler

import "github.com/hashicorp/nomad/nomad/structs"

func statesFromAllocs(eval *structs.Evaluation, currentJobVersion uint64, allocs []*structs.Allocation) (previous, current *allocStateCounts) {
	previous = &allocStateCounts{}
	current = &allocStateCounts{}

	incStatus := func(counts *allocStateCounts, alloc *structs.Allocation) {
		switch alloc.ClientStatus {
		case structs.AllocClientStatusPending:
			counts.pending++
		case structs.AllocClientStatusRunning:
			counts.running++
		case structs.AllocClientStatusComplete:
			counts.complete++
		case structs.AllocClientStatusFailed:
			counts.failed++
		case structs.AllocClientStatusLost:
			counts.lost++
		case structs.AllocClientStatusUnknown:
			counts.unknown++
		}
	}

	for _, alloc := range allocs {
		if alloc.Job != nil && alloc.Job.Version != currentJobVersion {
			incStatus(previous, alloc)
		} else {
			incStatus(current, alloc)
		}
	}
	return previous, current
}

func determineExpectedCounts(eval *structs.Evaluation, tg *structs.TaskGroup, d *structs.Deployment, previousVersion, currentVersion *allocStateCounts) (
	previous, current int,
) {
	if eval == nil {
		return 0, 0
	}

	minPrevious := previousVersion.running +
		previousVersion.pending +
		previousVersion.complete

	if d == nil {
		return minPrevious, tg.Count
	}

	base := tg.Count

	if d.RequiresPromotion() {
		// if we haven't promoted the deployment, we should have exactly count +
		// canaries, but we might have failed previous version allocs, and we
		// don't want to immediately replace them
		actual := previousVersion.running + previousVersion.pending + previousVersion.unknown
		if base < actual {
			return minPrevious, tg.Update.Canary
		}
		return actual, tg.Update.Canary
	}

	stepSize := tg.Update.MaxParallel +
		currentVersion.pending +
		currentVersion.running
	if stepSize > tg.Count {
		stepSize = tg.Count
	}

	maxPrevious := tg.Count - stepSize
	if maxPrevious < minPrevious {
		maxPrevious = minPrevious
	}
	actual := previousVersion.running + previousVersion.pending + previousVersion.unknown
	if actual < maxPrevious {
		return actual, stepSize
	}
	return maxPrevious, stepSize
}
