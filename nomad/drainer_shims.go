package nomad

import "github.com/hashicorp/nomad/nomad/structs"

// drainerShim implements the drainer.RaftApplier interface required by the
// NodeDrainer.
type drainerShim struct {
	s *Server
}

func (d drainerShim) NodeDrainComplete(nodeID string) error {
	args := &structs.NodeUpdateDrainRequest{
		NodeID:       nodeID,
		Drain:        false,
		WriteRequest: structs.WriteRequest{Region: d.s.config.Region},
	}

	resp, _, err := d.s.raftApply(structs.NodeUpdateDrainRequestType, args)
	return d.convertApplyErrors(resp, err)
}

func (d drainerShim) AllocUpdateDesiredTransition(allocs map[string]*structs.DesiredTransition, evals []*structs.Evaluation) error {
	args := &structs.AllocUpdateDesiredTransitionRequest{
		Allocs:       allocs,
		Evals:        evals,
		WriteRequest: structs.WriteRequest{Region: d.s.config.Region},
	}
	resp, _, err := d.s.raftApply(structs.AllocUpdateDesiredTransitionRequestType, args)
	return d.convertApplyErrors(resp, err)
}

// convertApplyErrors parses the results of a raftApply and returns the index at
// which it was applied and any error that occurred. Raft Apply returns two
// separate errors, Raft library errors and user returned errors from the FSM.
// This helper, joins the errors by inspecting the applyResponse for an error.
//
// Similar to deployment watcher's convertApplyErrors
func (d drainerShim) convertApplyErrors(applyResp interface{}, err error) error {
	if applyResp != nil {
		if fsmErr, ok := applyResp.(error); ok && fsmErr != nil {
			return fsmErr
		}
	}
	return err
}
