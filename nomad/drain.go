package nomad

import (
	"log"
	"time"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// startNodeDrainer should be called in establishLeadership by the leader.
func (s *Server) startNodeDrainer(stopCh chan struct{}) {
	state := s.fsm.State()

	// Determine first deadline
	deadlineTimer, drainingNodes := newDeadlineTimer(s.logger, state)

	// Determine if there are any drained allocs pending replacement
	drainingAllocs := gatherDrainingAllocs(s.logger, state, drainingNodes)

	//TODO need a chan to watch for alloc & job updates on if len(drainingallocs)>0
	_ = drainingAllocs

	for {
		select {
		// case <-NodeDrainStrategyChanging:
		// case <-Alloc running with Previous alloc in list
		case <-deadlineTimer.C:
		}

	}
}

// newDeadlineTimer returns a Timer that will tick when the next Node Drain
// Deadline is reached.
//TODO kinda ugly to return both a timer and a map from this but it saves an extra iteration over nodes
func newDeadlineTimer(logger *log.Logger, state *state.StateStore) (*time.Timer, map[string]struct{}) {
	// Create a stopped timer to return if no deadline is found
	deadlineTimer := time.NewTimer(0)
	if !deadlineTimer.Stop() {
		<-deadlineTimer.C
	}

	drainingNodes := make(map[string]struct{})

	iter, err := state.Nodes(nil)
	if err != nil {
		logger.Printf("[ERR] nomad.drain: error iterating nodes: %v", err)
		return deadlineTimer, drainingNodes
	}

	var nextDeadline time.Time
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		// Filter on datacenter and status
		node := raw.(*structs.Node)
		if !node.Drain {
			continue
		}

		drainingNodes[node.ID] = struct{}{}

		deadline := node.DrainStrategy.DeadlineTime()
		if deadline.IsZero() {
			continue
		}

		if nextDeadline.IsZero() || deadline.Before(nextDeadline) {
			nextDeadline = deadline
		}
	}

	// Enable the timer if a deadline was found
	if !nextDeadline.IsZero() {
		deadlineTimer.Reset(nextDeadline.Sub(time.Now()))
	}
	return deadlineTimer, drainingNodes
}

func gatherDrainingAllocs(logger *log.Logger, state *state.StateStore, drainingNodes map[string]struct{}) map[string]struct{} {
	if len(drainingNodes) == 0 {
		// There can't be any draining allocs if there are no draining nodes!
		return nil
	}

	drainingAllocs := map[string]struct{}{}

	// Collect allocs on draining nodes whose jobs are not stopped
	for nodeID := range drainingNodes {
		allocs, err := state.AllocsByNode(nil, nodeID)
		if err != nil {
			logger.Printf("[ERR] nomad.drain: error iterating allocs for node %q: %v", nodeID, err)
			return nil
		}

		for _, alloc := range allocs {
			if alloc.DesiredStatus == structs.AllocDesiredStatusStop {
				drainingAllocs[alloc.ID] = struct{}{}
			}
		}
	}

	// Remove allocs from draining list if they have a replacement alloc running
	iter, err := state.Allocs(nil)
	if err != nil {
		logger.Printf("[ERR] nomad.drain: error iterating allocs: %v", err)
		//TODO is it safe to return a non-nil map here?!
		return drainingAllocs
	}

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		alloc := raw.(*structs.Allocation)

		job, err := state.JobByID(nil, alloc.Namespace, alloc.JobID)
		if err != nil {
			logger.Printf("[ERR] nomad.drain: error getting job %q for alloc %q: %v", alloc.JobID, alloc.ID, err)
			// Errors here mean it's unlikely any further lookups will work
			return drainingAllocs
		}

		if job.Stopped() {
			// If this is a replacement allocation remove any
			// previous allocation from the draining allocations
			// list as if the draining allocation is terminal it
			// may not have an updated Job.
			delete(drainingAllocs, alloc.PreviousAllocation)

			// Nothing else to do for an alloc for a stopped Job
			continue
		}

		if alloc.ClientStatus == structs.AllocClientStatusRunning {
			// Remove the alloc this one replaced from the drained list
			delete(drainingAllocs, alloc.PreviousAllocation)
		}
	}

	return drainingAllocs
}
