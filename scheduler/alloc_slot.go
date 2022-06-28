package scheduler

import "github.com/hashicorp/nomad/nomad/structs"

type allocSlot struct {
	name           string
	index          int
	taskGroup      *structs.TaskGroup
	candidates     []*structs.Allocation
	desiredUpdates *structs.DesiredUpdates
	followupEvals  []*structs.Evaluation
}

func (as *allocSlot) PlaceResults() []allocPlaceResult {
	return nil
}

func (as *allocSlot) StopResults() []allocStopResult {
	return nil
}

func (as *allocSlot) DeploymentStatusUpdates() []*structs.DeploymentStatusUpdate {
	return nil
}

func (as *allocSlot) DestructiveResults() []allocDestructiveResult {
	return nil
}

func (as *allocSlot) InplaceUpdates() []*structs.Allocation {
	return nil
}

func (as *allocSlot) AttributeUpdates() map[string]*structs.Allocation {
	return nil
}

func (as *allocSlot) DisconnectUpdates() map[string]*structs.Allocation {
	return nil
}

func (as *allocSlot) ReconnectUpdates() map[string]*structs.Allocation {
	return nil
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
