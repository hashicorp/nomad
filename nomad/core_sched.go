package nomad

import (
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler"
)

// CoreScheduler is a special "scheduler" that is registered
// as "_core". It is used to run various administrative work
// across the cluster.
type CoreScheduler struct {
	srv  *Server
	snap *state.StateSnapshot
}

// NewCoreScheduler is used to return a new system scheduler instance
func NewCoreScheduler(srv *Server, snap *state.StateSnapshot) scheduler.Scheduler {
	s := &CoreScheduler{
		srv:  srv,
		snap: snap,
	}
	return s
}

// Process is used to implement the scheduler.Scheduler interface
func (s *CoreScheduler) Process(*structs.Evaluation) error {
	return nil
}
