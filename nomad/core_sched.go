package nomad

import (
	"fmt"
	"time"

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
func (s *CoreScheduler) Process(eval *structs.Evaluation) error {
	switch eval.JobID {
	case structs.CoreJobEvalGC:
		return s.evalGC(eval)
	default:
		return fmt.Errorf("core scheduler cannot handle job '%s'", eval.JobID)
	}
}

// evalGC is used to garbage collect old evaluations
func (c *CoreScheduler) evalGC(eval *structs.Evaluation) error {
	// Iterate over the evaluations
	iter, err := c.snap.Evals()
	if err != nil {
		return err
	}

	// Compute the old threshold limit for GC using the FSM
	// time table.  This is a rough mapping of a time to the
	// Raft index it belongs to.
	tt := c.srv.fsm.TimeTable()
	cutoff := time.Now().UTC().Add(-1 * c.srv.config.EvalGCThreshold)
	oldThreshold := tt.NearestIndex(cutoff)
	c.srv.logger.Printf("[DEBUG] sched.core: eval GC: scanning before index %d (%v)",
		oldThreshold, c.srv.config.EvalGCThreshold)

	// Collect the allocations and evaluations to GC
	var gcAlloc, gcEval []string

OUTER:
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		eval := raw.(*structs.Evaluation)

		// Ignore non-terminal and new evaluations
		if !eval.TerminalStatus() || eval.ModifyIndex > oldThreshold {
			continue
		}

		// Get the allocations by eval
		allocs, err := c.snap.AllocsByEval(eval.ID)
		if err != nil {
			c.srv.logger.Printf("[ERR] sched.core: failed to get allocs for eval %s: %v",
				eval.ID, err)
			continue
		}

		// Scan the allocations to ensure they are terminal and old
		for _, alloc := range allocs {
			if !alloc.TerminalStatus() || alloc.ModifyIndex > oldThreshold {
				continue OUTER
			}
		}

		// Evaluation is eligible for garbage collection
		gcEval = append(gcEval, eval.ID)
		for _, alloc := range allocs {
			gcAlloc = append(gcAlloc, alloc.ID)
		}
	}

	// Fast-path the nothing case
	if len(gcEval) == 0 && len(gcAlloc) == 0 {
		return nil
	}
	c.srv.logger.Printf("[DEBUG] sched.core: eval GC: %d evaluations, %d allocs",
		len(gcEval), len(gcAlloc))

	// Call to the leader to issue the reap
	req := structs.EvalDeleteRequest{
		Evals:  gcEval,
		Allocs: gcAlloc,
		WriteRequest: structs.WriteRequest{
			Region: c.srv.config.Region,
		},
	}
	var resp structs.GenericResponse
	if err := c.srv.RPC("Eval.Reap", &req, &resp); err != nil {
		c.srv.logger.Printf("[ERR] sched.core: eval reap failed: %v", err)
		return err
	}
	return nil
}
