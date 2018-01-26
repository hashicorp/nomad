package nomad

import (
	"log"
	"time"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

type drainingNode struct {
	deadline        bool
	remainingAllocs int
}

// startNodeDrainer should be called in establishLeadership by the leader.
func (s *Server) startNodeDrainer(stopCh chan struct{}) {
	state := s.fsm.State()

	// Determine first deadline
	deadlineTimer, drainingNodes := newDeadlineTimer(s.logger, state)

	// Determine if there are any drained allocs pending replacement
	drainingAllocs := gatherDrainingAllocs(s.logger, state, drainingNodes)

	//TODO need a chan to watch for alloc & job updates on if len(drainingallocs)>0
	_ = drainingAllocs
	var nodeUpdateCh chan struct{}
	var allocUpdateCh chan struct{}

	for {
		select {
		case <-nodeUpdateCh:
			// update draining nodes
		case <-allocUpdateCh:
			// update draining allocs
		case <-deadlineTimer.C:
			// deadline for a node was reached
		}

		now := time.Now()

		// collect all draining nodes and whether or not their deadline is reached
		drainingNodes = map[string]drainingNode{}
		nodes, err := state.Nodes(nil)
		if err != nil {
			//FIXME
			panic(err)
		}

		for {
			raw := nodes.Next()
			if raw == nil {
				break
			}

			node := raw.(*structs.Node)
			if !node.Drain {
				continue
			}

			drainingNodes[node.ID] = drainingNode{
				deadline: now.After(node.DrainStrategy.DeadlineTime()),
			}

		}

		// iterate over all allocs to clean drainingAllocs and stop
		// allocs with count==1 or whose node has reached its deadline
		allocs, err := state.Allocs(nil)
		if err != nil {
			//FIXME
			panic(err)
		}

		for {
			raw := allocs.Next()
			if raw == nil {
				break
			}

			alloc := raw.(*structs.Allocation)

			// If running remove the alloc this one replaced from the drained list
			if alloc.ClientStatus == structs.AllocClientStatusRunning {
				delete(drainingAllocs, alloc.PreviousAllocation)
				continue
			}

			job, err := state.JobByID(nil, alloc.Namespace, alloc.JobID)
			if err != nil {
				//FIXME
				panic(err)
			}

			// Do nothing for System jobs
			if job.Type == structs.JobTypeSystem {
				continue
			}

			// No action needed for allocs for stopped jobs. The
			// scheduler will take care of them.
			if job.Stopped() {
				// Clean out of drainingAllocs list since all
				// allocs for the job will be stopped by the
				// scheduler.
				delete(drainingAllocs, alloc.ID)
				continue
			}

			// See if this alloc is on a node which has met its drain deadline
			if drainingNodes[alloc.NodeID] {
				// No need to track draining allocs for nodes
				// who have reached their deadline as all
				// allocs on that node will be stopped
				delete(drainingAllocs, alloc.ID)

				//TODO is it safe to mutate this alloc or do I need to copy it first
				//alloc.DesiredStatus = structs.
			}
		}

		//TODO: emit node update evaluation for all nodes who reached their deadline
		//TODO: unset drain for nodes with no allocs
	}
}

// newDeadlineTimer returns a Timer that will tick when the next Node Drain
// Deadline is reached.
//TODO kinda ugly to return both a timer and a map from this but it saves an extra iteration over nodes
func newDeadlineTimer(logger *log.Logger, state *state.StateStore) (*nodeDeadline, map[string]struct{}) {
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

	if nextDeadline.IsZero() {
		return nil, drainingNodes
	}
	return timer.After(nextDeadline.Sub(timer.Now())), drainingNodes
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
