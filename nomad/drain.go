package nomad

import (
	"context"
	"log"
	"sync"
	"time"

	memdb "github.com/hashicorp/go-memdb"
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

	//TODO build deadlineTimer and drainingAllocs together to initialize state efficiently
	// Determine first deadline
	deadlineTimer, drainingNodes := newDeadlineTimer(s.logger, state)

	// Determine if there are any drained allocs pending replacement
	drainingAllocs := gatherDrainingAllocs(s.logger, state, drainingNodes)

	//TODO need a chan to watch for alloc & job updates on if len(drainingallocs)>0
	var nodeUpdateCh chan struct{}

	prevAllocs := newPrevAllocWatcher(s.logger, stopCh, drainingAllocs, state)
	go prevAllocs.run()

	for {
		select {
		case <-nodeUpdateCh:
			// update draining nodes
		case drainedID := <-prevAllocs.allocsCh:
			// drained alloc has been replaced
			//TODO update draining allocs
		case <-deadlineTimer.C:
			// deadline for a node was reached
		}

		now := time.Now()

		// collect all draining nodes and whether or not their deadline is reached
		//TODO don't shadow previous drainingNodes
		drainingNodes := map[string]drainingNode{}
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
			if _, ok := drainingNodes[alloc.NodeID]; ok {
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

type prevAllocWatcher struct {
	// watchList is a map of alloc ids to look for in PreviousAllocation
	// fields of new allocs
	watchList   map[string]struct{}
	watchListMu sync.Mutex

	// stopCh signals shutdown
	stopCh <-chan struct{}

	state *state.StateStore

	// allocsCh is sent Allocation.IDs as they're removed from the watchList
	allocsCh chan string

	logger *log.Logger
}

func newPrevAllocWatcher(logger *log.Logger, stopCh <-chan struct{}, drainingAllocs map[string]struct{},
	state *state.StateStore) *prevAllocWatcher {

	return &prevAllocWatcher{
		watchList: drainingAllocs,
		stopCh:    stopCh,
		allocsCh:  make(chan string, 8), //FIXME 8? really? what should this be
		logger:    logger,
	}
}

func (p *prevAllocWatcher) run() {
	// convert stopCh to a Context for BlockingQuery
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-p.stopCh:
			cancel()
		case <-ctx.Done():
			// already cancelled
		}
	}()

	// index to watch from
	var resp interface{}
	var index uint64 = 1
	var err error

	for {
		resp, index, err = p.state.BlockingQuery(p.queryPrevAlloc, index, ctx)
		if err != nil {
			p.logger.Printf("[ERR] nomad.drain: error blocking on alloc updates: %v", err)
			return
		}

		allocIDs := resp.([]string)
		for _, id := range allocIDs {
			select {
			case p.allocsCh <- id:
			case <-p.stopCh:
				return
			}
		}
	}
}

func (p *prevAllocWatcher) queryPrevAlloc(ws memdb.WatchSet, state *state.StateStore) (interface{}, uint64, error) {
	allocs, err := state.Allocs(ws)
	if err != nil {
		return nil, 0, err
	}

	index, err := state.Index("allocs")
	if err != nil {
		return nil, 0, err
	}

	now := time.Now()

	p.watchListMu.Lock()
	defer p.watchListMu.Unlock()

	resp := make([]string, 0, len(p.watchList))

	//FIXME needs to use result iterator
	for _, alloc := range allocs.([]*structs.Allocation) {
		deadline, ok := p.watchList[alloc.PreviousAllocation]
		if !ok {
			// PreviousAllocation not in watchList, skip it
			continue
		}

		// If the migration health is set on the replacement alloc we can stop watching the drained alloc
		if alloc.DeploymentStatus.IsHealthy() || alloc.DeploymentStatus.IsUnhealthy() {
			delete(p.watchList, alloc.PreviousAllocation)
			resp = append(resp, alloc.PreviousAllocation)
			continue
		}

		//TODO should I implement this?
		// As a fail-safe from blocking drains indefinitely stop
		// watching a drained alloc if a replacement has existed for
		// longer than the drained alloc's deadline
	}

	return resp, index, nil
}

// newDeadlineTimer returns a Timer that will tick when the next Node Drain
// Deadline is reached.
//TODO kinda ugly to return both a timer and a map from this but it saves an extra iteration over nodes
func newDeadlineTimer(logger *log.Logger, state *state.StateStore) (time.Time, map[string]struct{}) {
	drainingNodes := make(map[string]drainingNode)

	iter, err := state.Nodes(nil)
	if err != nil {
		logger.Printf("[ERR] nomad.drain: error iterating nodes: %v", err)
		return time.NewTimer(0), drainingNodes
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

		deadline := node.DrainStrategy.DeadlineTime()

		drainingNodes[node.ID] = struct{}{}

		if deadline.IsZero() {
			continue
		}

		if nextDeadline.IsZero() || deadline.Before(nextDeadline) {
			nextDeadline = deadline
		}
	}

	if nextDeadline.IsZero() {
		//FIXME returning nil is going to cause a panic, return a time.Time
		return nil, drainingNodes
	}
	return time.After(nextDeadline.Sub(time.Now())), drainingNodes
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
