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

	nextDeadline, nodesIndex, drainingAllocs, allocsIndex := initDrainer(s.logger, state)

	//TODO create node deadline timer
	_ = nextDeadline
	var deadlineTimer time.Timer

	//TODO create node watcher
	_ = nodesIndex
	var nodeUpdateCh chan struct{}

	prevAllocs := newPrevAllocWatcher(s.logger, stopCh, drainingAllocs, allocsIndex, state)
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

			//FIXME
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
	watchList   map[string]time.Time
	watchListMu sync.Mutex

	// stopCh signals shutdown
	stopCh <-chan struct{}

	state *state.StateStore

	// allocIndex to start watching from
	allocIndex uint64

	// allocsCh is sent Allocation.IDs as they're removed from the watchList
	allocsCh chan string

	logger *log.Logger
}

func newPrevAllocWatcher(logger *log.Logger, stopCh <-chan struct{}, drainingAllocs map[string]time.Time, allocIndex uint64,
	state *state.StateStore) *prevAllocWatcher {

	return &prevAllocWatcher{
		watchList:  drainingAllocs,
		state:      state,
		stopCh:     stopCh,
		allocIndex: allocIndex,
		allocsCh:   make(chan string, 8), //FIXME 8? really? what should this be
		logger:     logger,
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
	var err error

	for {
		//FIXME it seems possible for this to return a nil error and a 0 index, what to do in that case?
		resp, p.allocIndex, err = p.state.BlockingQuery(p.queryPrevAlloc, p.allocIndex, ctx)
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

func initDrainer(logger *log.Logger, state *state.StateStore) (time.Time, uint64, map[string]time.Time, uint64) {
	// StateStore.Snapshot never returns an error so don't bother checking it
	snapshot, _ := state.Snapshot()
	now := time.Now()

	iter, err := snapshot.Nodes(nil)
	if err != nil {
		logger.Printf("[ERR] nomad.drain: error iterating nodes: %v", err)
		panic(err) //FIXME
	}

	// node.ID -> drain deadline
	nodeDeadlines := map[string]time.Time{}

	// List of draining allocs by namespace and job: namespace -> job.ID -> alloc.ID -> *Allocation
	allocsByNS := map[string]map[string]map[string]*structs.Allocation{}

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

		nodeDeadlines[node.ID] = deadline

		if deadline.Before(nextDeadline) {
			nextDeadline = deadline
		}

		if deadline.Before(now) {
			// No point in tracking draining allocs as the deadline has been reached
			continue
		}

		allocs, err := snapshot.AllocsByNode(nil, node.ID)
		if err != nil {
			logger.Printf("[ERR] nomad.drain: error iterating allocs for node %q: %v", nodeID, err)
			panic(err) //FIXME
		}

		for _, alloc := range allocs {
			if alloc.DesiredStatus == structs.AllocDesiredStatusStop {
				if allocsByJob, ok := allocsByNS[alloc.Namespace]; ok {
					if allocs, ok := allocsByJob[alloc.JobID]; ok {
						allocs[alloc.ID] = alloc
					} else {
						// First alloc for job
						allocsByJob[alloc.JobID] = map[string]struct{}{alloc.ID: alloc}
					}
				} else {
					// First alloc in namespace
					allocsByNS[alloc.Namespace] = map[string]map[string]struct{}{
						alloc.JobID: map[string]struct{}{alloc.ID: alloc},
					}
				}
			}
		}
	}

	// alloc.ID -> LastModified+MigrateStrategy.HealthyDeadline
	drainingAllocs := map[string]time.Time{}

	for ns, allocsByJobs := range allocsByNS {
		for jobID, allocs := range allocsByJobs {
			job, err := snapshot.JobByID(nil, alloc.Namespace, alloc.JobID)
			if err != nil {
				logger.Printf("[ERR] nomad.drain: error getting job %q for alloc %q: %v", alloc.JobID, alloc.ID, err)
				//FIXME
				panic(err)
			}

			// Don't track drains for stopped or gc'd jobs
			if job == nil || job.Status == structs.JobStatusDead {
				continue
			}

			jobAllocs, err := snapshot.AllocsByJob(nil, alloc.Namespace, alloc.JobID, true)
			if err != nil {
				//FIXME
				panic(err)
			}

			// Remove drained allocs for replacement allocs
			for _, alloc := range jobAllocs {
				if alloc.DeploymentStatus.IsHealthy() || alloc.DeploymentStatus.IsUnhealthy() {
					delete(allocs, alloc.PreviousAllocation)
				}
			}

			// Any remaining allocs need to be tracked
			for allocID, alloc := range allocs {
				tg := job.LookupTaskGroup(alloc.TaskGroup)
				if tg == nil {
					logger.Printf("[DEBUG] nomad.drain: unable to find task group %q for alloc %q", alloc.TaskGroup, allocID)
					continue
				}

				if tg.MigrateStrategy == nil {
					// No migrate strategy so don't track
					continue
				}

				// alloc.ModifyTime + HealthyDeadline is >= the
				// healthy deadline for the allocation, so we
				// can stop tracking it at that time.
				deadline := time.Unix(0, alloc.ModifyTime).Add(tg.MigrateStrategy.HealthyDeadline)

				if deadline.After(now) {
					// deadline already reached; don't bother tracking
					continue
				}

				// Draining allocation hasn't been replaced or
				// reached its deadline; track it!
				drainingAllocs[allocID] = deadline
			}
		}
	}

	nodesIndex, _ := snapshot.Index("nodes")
	if nodeIndex == 0 {
		//FIXME what to do here?
	}
	allocsIndex, _ := snapshot.Index("allocs")
	if allocIndex == 0 {
		//FIXME what to do here?
	}
	return nextDeadline, nodesIndex, drainingAllocs, allocsIndex
}
