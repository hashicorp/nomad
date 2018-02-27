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

	_, _, err := d.s.raftApply(structs.NodeUpdateDrainRequestType, args)
	return err
}

func (d drainerShim) AllocUpdateDesiredTransition(allocs map[string]*structs.DesiredTransition, evals []*structs.Evaluation) error {
	args := &structs.AllocUpdateDesiredTransitionRequest{
		Allocs:       allocs,
		Evals:        evals,
		WriteRequest: structs.WriteRequest{Region: d.s.config.Region},
	}
	_, _, err := d.s.raftApply(structs.AllocUpdateDesiredTransitionRequestType, args)
	return err
}
