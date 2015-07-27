package nomad

import "github.com/hashicorp/nomad/nomad/structs"

// planApply is a long lived goroutine that reads plan allocations from
// the plan queue, determines if they can be applied safely and applies
// them via Raft.
func (s *Server) planApply() {
	for {
		// Pull the next pending plan, exit if we are no longer leader
		pending, err := s.planQueue.Dequeue(0)
		if err != nil {
			return
		}

		// TODO: Evaluate the plan

		// TODO: Apply the plan

		// TODO: Prepare the response
		result := &structs.PlanResult{
			AllocIndex: 1000,
		}

		// Respond to the plan
		pending.respond(result, nil)
	}
}
