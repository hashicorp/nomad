package nomad

import (
	"context"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

type drainingJob struct {
	job    *structs.Job
	allocs []*structs.Allocation
}

type drainingAlloc struct {
	// LastModified+MigrateStrategy.HealthyDeadline
	deadline time.Time

	// Task Group key
	tgKey string
}

func newDrainingAlloc(a *structs.Allocation, deadline time.Time) drainingAlloc {
	return drainingAlloc{
		deadline: deadline,
		key:      makeTaskGroupKey(a),
	}
}

// makeTaskGroupKey returns a unique key for an allocation's task group
func makeTaskGroupKey(a *structs.Allocation) string {
	return strings.Join([]string{a.Namespace, a.JobID, a.TaskGroup}, "-")
}

type stopAllocs map[string][]string

func (s stopAllocs) add(a *structs.Allocation) {
	key := makeTaskGroupKey(a)
	s[key] = append(s[key], a.ID)
}

// startNodeDrainer should be called in establishLeadership by the leader.
func (s *Server) startNodeDrainer(stopCh chan struct{}) {
	state := s.fsm.State()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	nodes, nodesIndex, drainingAllocs, allocsIndex := initDrainer(s.logger, state)

	// Wait for a node's drain deadline to expire
	nextDeadline := time.Unix(math.MaxInt64, math.MaxInt64)
	for _, deadline := range nodes {
		if deadline.Before(nextDeadline) {
			nextDeadline = deadline
		}

	}
	deadlineTimer := time.NewTimer(time.Until(nextDeadline))

	// Watch for nodes to start or stop draining
	nodeWatcher := newNodeWatcher(s.logger, nodes, nodesIndex, state)
	go nodeWatcher.run(ctx)

	// Watch for drained allocations to be replaced
	prevAllocs := newPrevAllocWatcher(s.logger, drainingAllocs, allocsIndex, state)
	go prevAllocs.run(ctx)

	for {
		//TODO this method of async node updates means we could make
		//migration decisions on out of date information. the worst
		//possible outcome of this is that an allocation could be
		//stopped on a node that recently had its drain cancelled which
		//doesn't seem like that bad of a pathological case
		select {
		case nodes = <-nodeWatcher.nodesCh:
			// update draining nodes
		case drainedID := <-prevAllocs.allocsCh:
			// drained alloc has been replaced
			delete(drainingAllocs, drainedID)
		case <-deadlineTimer.C:
			// deadline for a node was reached
		case <-ctx.Done():
			// exit
			return
		}

		//TODO work from a state snapshot? perhaps from a last update
		//index? I can't think of why this would be beneficial as this
		//entire process runs asynchronously with the fsm/scheduler/etc
		snapshot := state.Snapshot()
		now := time.Now() // for determing deadlines in a consistent way

		// namespace -> job id -> {job, allocs}
		// Collect all allocs for all jobs with at least one
		// alloc on a draining node.
		// Invariants:
		//  - No system jobs
		//  - No batch jobs unless their node's deadline is reached
		//  - No entries with 0 allocs
		drainable := map[string]map[string]*drainingJob{}

		// Collect all drainable jobs
		for nodeID, deadline := range nodes {
			allocs, err := snapshot.AllocsByNode(nil, node.ID)
			if err != nil {
				//FIXME
				panic(err)
			}

			for _, alloc := range allocs {
				if _, ok := drainable[alloc.Namespace]; !ok {
					// namespace does not exist
					drainable[alloc.Namespace] = make(map[string]*drainingJob)
				}

				if _, ok := drainable[alloc.Namespace][alloc.JobID]; ok {
					// already found
					continue
				}

				// job does not found yet
				job, err := snapshot.JobByID(nil, alloc.Namespace, alloc.JobID)
				if err != nil {
					//FIXME
					panic(err)
				}
				//TODO check for job == nil?

				// Don't bother collecting system jobs
				if job.Type == structs.JobTypeSystem {
					continue
				}

				// Don't bother collecting batch jobs for nodes that haven't hit their deadline
				if job.Type == structs.JobTypeBatch && deadline.After(now) {
					continue
				}

				jobAllocs, err := snapshot.AllocsByJob(nil, alloc.Namespace, alloc.JobID, true)
				if err != nil {
					//FIXME
					panic(err)
				}

				drainable[alloc.Namespace][alloc.JobID] = &drainingJob{
					job:    job,
					allocs: jobAllocs,
				}
			}
		}

		//TODO convert drainingAllocs to this outside the mainloop?!
		// Map of allocs which have already been stopped and are
		// awaiting their replacement; ns+jobid+tg -> count of in progress drains
		alreadyStopped := make(map[string]int, len(drainingAllocs))
		for id, a := range drainingAllocs {
			alreadyStopped[a.tgKey]++
		}

		// Map of allocations to stop (drain); ns+jobid+tg -> alloc.ID
		toStop := make(stopAllocs, len(drainingAllocs))

		//TODO build drain list considering deadline & max_parallel
		for ns, drainingJobs := range drainable {
			for jobID, drainingJob := range drainingJobs {
				for _, alloc := range drainingJob.allocs {
					// Already draining/dead allocs don't need to be drained
					if alloc.TerminalStatus() {
						continue
					}

					deadline, ok := nodes[alloc.NodeID]
					if !ok {
						// Alloc's node is not draining so not elligible for draining!
						continue
					}

					if deadline.Before(now) {
						// Alloc's Node has reached its deadline
						stoplist.add(alloc)
						continue
					}

					// Batch jobs are only stopped when the node
					// deadline is reached which has already been
					// done.
					if job.Type == structs.JobTypeBatch {
						continue
					}

					// Stop allocs with count=1, max_parallel==0, or draining<max_parallel
					tg := drainingJob.Job.LookupTaskGroup(alloc.TaskGroup)
					//FIXME tg==nil here?

					// Only 1, drain
					if tg.Count == 1 {
						stoplist.add(alloc)
						continue
					}

					// No migrate strategy or a max parallel of 0 mean force draining
					if tg.MigrateStrategy == nil || tg.MigrateStrategy.MaxParallel == 0 {
						stoplist.add(alloc)
						continue
					}

					// If MaxParallel > how many allocs are
					// already draining for this task
					// group, drain this alloc as well
					tgKey := makeTaskGroupKey(alloc)
					if tg.MigrateStrategy.MaxParallel > alreadyStopped[tgKey] {
						// More migrations are allowed
						stoplist.add(alloc)
						alreadyStopped[tgKey]++
					}
				}
			}
		}

		//TODO stop allocs in stoplist and add them to drainingAllocs + prevAllocWatcher
		//TODO emit node update evaluation for all nodes who reached their deadline
		//TODO unset drain for nodes with no allocs
	}
}

type nodeWatcher struct {
	index   uint64
	nodes   map[string]time.Time
	nodesCh chan map[string]time.Time
	state   *state.StateStore
	logger  *log.Logger
}

func newNodeWatcher(logger *log.Logger, nodes map[string]time.Time, index uint64, state *state.StateStore) *nodeWatcher {
	return &nodeWatcher{
		nodes:  nodes,
		index:  index,
		state:  state,
		logger: logger,
	}
}

func (n *nodeWatcher) run(ctx context.Context) {
	for {
		//FIXME it seems possible for this to return a nil error and a 0 index, what to do in that case?
		resp, index, err := n.state.BlockingQuery(n.queryNodeDrain, n.index, ctx)
		if err != nil {
			n.logger.Printf("[ERR] nomad.drain: error blocking on node updates at index %d: %v", n.index, err)
			return
		}

		// update index for next run
		n.index = index

		nodes := resp.([]*structs.Node)
		for _, node := range nodes {
			if _, ok := n.nodes[node.ID]; ok {
				// Node was draining
				if !node.Drain {
					// Node stopped draining
					delete(n.nodes, node.ID)
				} else {
					// Update deadline
					n.nodes[node.ID] = node.DrainStrategy.DeadlineTime()
				}
			} else {
				// Node was not draining
				if node.Drain {
					// Node started draining
					n.nodes[node.ID] = node.DrainStrategy.DeadlineTime()
				}
			}
		}

		// Send a copy of the draining nodes
		nodesCopy := make(map[string]time.Time, len(n.nodes))
		for k, v := range n.nodes {
			nodesCopy[k] = v
		}

		select {
		case n.nodesCh <- nodesCopy:
		case <-ctx.Done():
			return
		}
	}
}

func (n *nodeWatcher) queryNodeDrain(ws memdb.WatchSet, state *state.StateStore) (interface{}, uint64, error) {
	iter, err := state.Nodes(ws)
	if err != nil {
		return nil, 0, err
	}

	index, err := state.Index("nodes")
	if err != nil {
		return nil, 0, err
	}

	//FIXME initial cap?
	resp := make([]*structs.Node, 0, 1)

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		node := raw.(*structs.Node)
		resp = append(resp, node)
	}

	return resp, index, nil
}

type prevAllocWatcher struct {
	// watchList is a map of alloc ids to look for in PreviousAllocation
	// fields of new allocs
	watchList   map[string]drainingAlloc
	watchListMu sync.Mutex

	state *state.StateStore

	// allocIndex to start watching from
	allocIndex uint64

	// allocsCh is sent Allocation.IDs as they're removed from the watchList
	allocsCh chan string

	logger *log.Logger
}

func newPrevAllocWatcher(logger *log.Logger, drainingAllocs map[string]drainingAlloc, allocIndex uint64,
	state *state.StateStore) *prevAllocWatcher {

	return &prevAllocWatcher{
		watchList:  drainingAllocs,
		state:      state,
		allocIndex: allocIndex,
		allocsCh:   make(chan string, 8), //FIXME 8? really? what should this be
		logger:     logger,
	}
}

func (p *prevAllocWatcher) run(ctx context.Context) {
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
			case <-ctx.Done():
				return
			}
		}
	}
}

func (p *prevAllocWatcher) queryPrevAlloc(ws memdb.WatchSet, state *state.StateStore) (interface{}, uint64, error) {
	iter, err := state.Allocs(ws)
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

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		alloc := raw.(*structs.Allocation)
		drainingAlloc, ok := p.watchList[alloc.PreviousAllocation]
		if !ok {
			// PreviousAllocation not in watchList, skip it
			continue
		}

		// If the migration health is set on the replacement alloc we can stop watching the drained alloc
		if alloc.DeploymentStatus.IsHealthy() || alloc.DeploymentStatus.IsUnhealthy() || drainingAlloc.deadline.After(now) {
			delete(p.watchList, alloc.PreviousAllocation)
			resp = append(resp, alloc.PreviousAllocation)
			continue
		}
	}

	return resp, index, nil
}

func initDrainer(logger *log.Logger, state *state.StateStore) (map[string]time.Time, uint64, map[string]time.Time, uint64) {
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

		if deadline.Before(now) {
			// No point in tracking draining allocs as the deadline has been reached
			continue
		}

		allocs, err := snapshot.AllocsByNode(nil, node.ID)
		if err != nil {
			logger.Printf("[ERR] nomad.drain: error iterating allocs for node %q: %v", node.ID, err)
			panic(err) //FIXME
		}

		for _, alloc := range allocs {
			if alloc.DesiredStatus == structs.AllocDesiredStatusStop {
				if allocsByJob, ok := allocsByNS[alloc.Namespace]; ok {
					if allocs, ok := allocsByJob[alloc.JobID]; ok {
						allocs[alloc.ID] = alloc
					} else {
						// First alloc for job
						allocsByJob[alloc.JobID] = map[string]*structs.Allocation{alloc.ID: alloc}
					}
				} else {
					// First alloc in namespace
					allocsByNS[alloc.Namespace] = map[string]map[string]*structs.Allocation{
						alloc.JobID: map[string]*structs.Allocation{alloc.ID: alloc},
					}
				}
			}
		}
	}

	// alloc.ID ->
	drainingAllocs := map[string]time.Time{}

	for ns, allocsByJobs := range allocsByNS {
		for jobID, allocs := range allocsByJobs {
			for allocID, alloc := range allocs {
				job, err := snapshot.JobByID(nil, ns, jobID)
				if err != nil {
					logger.Printf("[ERR] nomad.drain: error getting job %q for alloc %q: %v", alloc.JobID, allocID, err)
					//FIXME
					panic(err)
				}

				// Don't track drains for stopped or gc'd jobs
				if job == nil || job.Status == structs.JobStatusDead {
					continue
				}

				jobAllocs, err := snapshot.AllocsByJob(nil, ns, jobID, true)
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

					if tg.Migrate == nil {
						// No migrate strategy so don't track
						continue
					}

					// alloc.ModifyTime + HealthyDeadline is >= the
					// healthy deadline for the allocation, so we
					// can stop tracking it at that time.
					deadline := time.Unix(0, alloc.ModifyTime).Add(tg.Migrate.HealthyDeadline)

					if deadline.After(now) {
						// deadline already reached; don't bother tracking
						continue
					}

					// Draining allocation hasn't been replaced or
					// reached its deadline; track it!
					drainingAllocs[allocID] = newDrainingAlloc(alloc, deadline)
				}
			}
		}
	}

	nodesIndex, _ := snapshot.Index("nodes")
	if nodesIndex == 0 {
		//FIXME what to do here?
	}
	allocsIndex, _ := snapshot.Index("allocs")
	if allocsIndex == 0 {
		//FIXME what to do here?
	}
	return nodeDeadlines, nodesIndex, drainingAllocs, allocsIndex
}
