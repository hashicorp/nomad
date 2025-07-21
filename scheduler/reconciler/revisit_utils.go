package reconciler

import "github.com/hashicorp/nomad/nomad/structs"

type allocStateCounts struct {
	pending  int
	running  int
	complete int
	failed   int
	lost     int
	unknown  int
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
	stepSize = min(stepSize, tg.Count)

	maxPrevious := tg.Count - stepSize
	maxPrevious = max(maxPrevious, minPrevious)

	actual := previousVersion.running + previousVersion.pending + previousVersion.unknown
	if actual < maxPrevious {
		return actual, stepSize
	}
	return maxPrevious, stepSize
}
