package nomad

import (
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler"
)

// SystemScheduler is a special "scheduler" that is registered
// as "system". It is used to run various administrative work
// across the cluster.
type SystemScheduler struct {
	srv  *Server
	snap *state.StateSnapshot
}

// NewSystemScheduler is used to return a new system scheduler instance
func NewSystemScheduler(srv *Server, snap *state.StateSnapshot) scheduler.Scheduler {
	s := &SystemScheduler{
		srv:  srv,
		snap: snap,
	}
	return s
}

// Process is used to implement the scheduler.Scheduler interface
func (s *SystemScheduler) Process(*structs.Evaluation) error {
	return nil
}
