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

func (as *allocSlot) placeResults() []allocPlaceResult {
	return nil
}

func (as *allocSlot) stopResults() []allocStopResult {
	return nil
}

func (as *allocSlot) deploymentStatusUpdates() []*structs.DeploymentStatusUpdate {
	return nil
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
