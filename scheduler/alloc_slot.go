package scheduler

import "github.com/hashicorp/nomad/nomad/structs"

type allocSlot struct {
	name       string
	index      int
	taskGroup  *structs.TaskGroup
	candidates []*structs.Allocation
	mediator   resultMediator
}

func (as *allocSlot) removeCandidate(alloc *structs.Allocation) {
	if as.candidates == nil {
		return
	}

	for i, candidate := range as.candidates {
		if candidate.ID == alloc.ID {
			as.candidates = append(as.candidates[:i], as.candidates[i+1:]...)
			return
		}
	}
}

func (as *allocSlot) resolveCandidates() {
	// TODO: confirm this order of execution works.
	as.stopResults()
	as.placeResults()
	as.destructiveResults()
	as.inplaceUpdates()
	as.attributeUpdates()
	as.disconnectUpdates()
	as.reconnectUpdates()
}

func (as *allocSlot) placeResults() []allocPlaceResult {
	// TODO: Migrating allocs should be placed - see computeMigrations
	return nil
}

func (as *allocSlot) stopResults() []allocStopResult {
	var result []allocStopResult

	for _, candidate := range as.candidates {
		// Stop migrations
		if candidate.DesiredTransition.ShouldMigrate() {
			as.mediator.stopMigrating(candidate)
		}

		// Stop unneeded canaries - see cancelUnneededCanaries

		// Stop failed allocations - see computeReplacements

		// Stop rescheduled replacements - see computeReplacements

		// Stop replaced by canary - see computeStop

		// TODO: Do we need a duplicate name handler - see computeStop

		// Stop reconnecting - see computeStopByReconnecting - pay attention to failed reconnects
	}

	return result
}

func (as *allocSlot) destructiveResults() []allocDestructiveResult {
	return nil
}

func (as *allocSlot) inplaceUpdates() []*structs.Allocation {
	return nil
}

func (as *allocSlot) attributeUpdates() map[string]*structs.Allocation {
	return nil
}

func (as *allocSlot) disconnectUpdates() map[string]*structs.Allocation {
	return nil
}

func (as *allocSlot) reconnectUpdates() map[string]*structs.Allocation {
	return nil
}
