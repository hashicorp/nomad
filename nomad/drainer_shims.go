package nomad

import "github.com/hashicorp/nomad/nomad/structs"

// drainerShim implements the drainer.RaftApplier interface required by the
// NodeDrainer.
type drainerShim struct {
	s *Server
}

func (d drainerShim) NodeDrainComplete(nodeID string) (uint64, error) {
	args := &structs.NodeUpdateDrainRequest{
		NodeID:       nodeID,
		Drain:        false,
		WriteRequest: structs.WriteRequest{Region: d.s.config.Region},
	}

	resp, index, err := d.s.raftApply(structs.NodeUpdateDrainRequestType, args)
	return d.convertApplyErrors(resp, index, err)
}

func (d drainerShim) AllocUpdateDesiredTransition(allocs map[string]*structs.DesiredTransition, evals []*structs.Evaluation) (uint64, error) {
	args := &structs.AllocUpdateDesiredTransitionRequest{
		Allocs:       allocs,
		Evals:        evals,
		WriteRequest: structs.WriteRequest{Region: d.s.config.Region},
	}
	resp, index, err := d.s.raftApply(structs.AllocUpdateDesiredTransitionRequestType, args)
	return d.convertApplyErrors(resp, index, err)
}

// convertApplyErrors parses the results of a raftApply and returns the index at
// which it was applied and any error that occurred. Raft Apply returns two
// separate errors, Raft library errors and user returned errors from the FSM.
// This helper, joins the errors by inspecting the applyResponse for an error.
func (d drainerShim) convertApplyErrors(applyResp interface{}, index uint64, err error) (uint64, error) {
	if applyResp != nil {
		if fsmErr, ok := applyResp.(error); ok && fsmErr != nil {
			return index, fsmErr
		}
	}
	return index, err
}
