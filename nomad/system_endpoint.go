package nomad

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

// System endpoint is used to call invoke system tasks.
type System struct {
	srv *Server
}

// GarbageCollect is used to trigger the system to immediately garbage collect nodes, evals
// and jobs.
func (s *System) GarbageCollect(args *structs.GenericRequest, reply *structs.GenericResponse) error {
	if done, err := s.srv.forward("System.GarbageCollect", args, args, reply); done {
		return err
	}

	s.srv.evalBroker.Enqueue(s.srv.coreJobEval(structs.CoreJobForceGC))
	return nil
}
