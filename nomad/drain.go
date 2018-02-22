package nomad

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// jobKey is a tuple of namespace+jobid for use as a map key by job
type jobKey struct {
	ns    string
	jobid string
}

// drainingJob contains the Job and allocations for that job meant to be used
// when collecting all allocations for a job with at least one allocation on a
// draining node.
//
// This allows the MaxParallel calculation to take the entire job's allocation
// state into account. FIXME is that even useful?
type drainingJob struct {
	job    *structs.Job
	allocs []*structs.Allocation
}

// drainingAlloc contains a conservative deadline an alloc has to be healthy by
// before it should stopped being watched and replaced.
type drainingAlloc struct {
	// LastModified+MigrateStrategy.HealthyDeadline
	deadline time.Time

	// Task Group key
	tgKey string
}

func newDrainingAlloc(a *structs.Allocation, deadline time.Time) drainingAlloc {
	return drainingAlloc{
		deadline: deadline,
		tgKey:    makeTaskGroupKey(a),
	}
}

// makeTaskGroupKey returns a unique key for an allocation's task group
func makeTaskGroupKey(a *structs.Allocation) string {
	return strings.Join([]string{a.Namespace, a.JobID, a.TaskGroup}, "-")
}

// stopAllocs tracks allocs to drain by a unique TG key
type stopAllocs struct {
	allocBatch []*structs.Allocation

	// namespace+jobid -> Job
	jobBatch map[jobKey]*structs.Job
}

//FIXME this method does an awful lot
func (s *stopAllocs) add(j *structs.Job, a *structs.Allocation) {
	// Update the allocation
	a.ModifyTime = time.Now().UnixNano()
	a.DesiredStatus = structs.AllocDesiredStatusStop

	// Add alloc to the allocation batch
	s.allocBatch = append(s.allocBatch, a)

	// Add job to the job batch
	s.jobBatch[jobKey{a.Namespace, a.JobID}] = j
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

	nodes, nodesIndex, drainingJobs, allocsIndex := initDrainer(s.logger, state)

	// Wait for a node's drain deadline to expire
	var nextDeadline time.Time
	for _, node := range nodes {
		if nextDeadline.IsZero() {
			nextDeadline = node.DrainStrategy.DeadlineTime()
			continue
		}
		if deadline := node.DrainStrategy.DeadlineTime(); deadline.Before(nextDeadline) {
			nextDeadline = deadline
		}

	}
	deadlineTimer := time.NewTimer(time.Until(nextDeadline))

	// Watch for nodes to start or stop draining
	nodeWatcher := newNodeWatcher(s.logger, nodes, nodesIndex, state)
	go nodeWatcher.run(ctx)

	// Watch for drained allocations to be replaced
	// Watch for changes in allocs for jobs with allocs on draining nodes
	jobWatcher := newJobWatcher(s.logger, drainingJobs, allocsIndex, state)
	go jobWatcher.run(ctx)

	for {
		//TODO this method of async node updates means we could make
		//migration decisions on out of date information. the worst
		//possible outcome of this is that an allocation could be
		//stopped on a node that recently had its drain cancelled which
		//doesn't seem like that bad of a pathological case
		s.logger.Printf("[TRACE] nomad.drain: LOOP next deadline: %s (%s)", nextDeadline, time.Until(nextDeadline))
		select {
		case nodes = <-nodeWatcher.nodesCh:
			// update draining nodes
			s.logger.Printf("[TRACE] nomad.drain: running due to node change (%d nodes draining)", len(nodes))

			// update deadline timer
			changed := false
			for _, n := range nodes {
				if nextDeadline.IsZero() {
					nextDeadline = n.DrainStrategy.DeadlineTime()
					changed = true
					continue
				}

				if deadline := n.DrainStrategy.DeadlineTime(); deadline.Before(nextDeadline) {
					nextDeadline = deadline
					changed = true
				}
			}

			// if changed reset the timer
			if changed {
				s.logger.Printf("[TRACE] nomad.drain: new node deadline: %s", nextDeadline)
				if !deadlineTimer.Stop() {
					// timer may have been recv'd in a
					// previous loop, so don't block
					select {
					case <-deadlineTimer.C:
					default:
					}
				}
				deadlineTimer.Reset(time.Until(nextDeadline))
			}

		case jobs := <-jobWatcher.WaitCh():
			s.logger.Printf("[TRACE] nomad.drain: running due to alloc change (%d jobs updated)", len(jobs))
		case when := <-deadlineTimer.C:
			// deadline for a node was reached
			s.logger.Printf("[TRACE] nomad.drain: running due to deadline reached (at %s)", when)
		case <-ctx.Done():
			// exit
			return
		}

		// Tracks nodes that are done draining
		doneNodes := map[string]*structs.Node{}

		//TODO work from a state snapshot? perhaps from a last update
		//index? I can't think of why this would be beneficial as this
		//entire process runs asynchronously with the fsm/scheduler/etc
		snapshot, err := state.Snapshot()
		if err != nil {
			//FIXME
			panic(err)
		}
		now := time.Now() // for determing deadlines in a consistent way

		// job key -> {job, allocs}
		// Collect all allocs for all jobs with at least one
		// alloc on a draining node.
		// Invariants:
		//  - No system jobs
		//  - No batch jobs unless their node's deadline is reached
		//  - No entries with 0 allocs
		//TODO could this be a helper method on prevAllocWatcher
		drainable := map[jobKey]*drainingJob{}

		// track jobs we've looked up before and know we shouldn't
		// consider for draining eg system jobs
		skipJob := map[jobKey]struct{}{}

		// track number of "up" allocs per task group (not terminal and
		// have a deployment status)
		upPerTG := map[string]int{}

		// Collect all drainable jobs
		for nodeID, node := range nodes {
			allocs, err := snapshot.AllocsByNode(nil, nodeID)
			if err != nil {
				//FIXME
				panic(err)
			}

			// track number of allocs left on this node to be drained
			allocsLeft := false
			for _, alloc := range allocs {
				jobkey := jobKey{alloc.Namespace, alloc.JobID}

				if _, ok := drainable[jobkey]; ok {
					// already found
					continue
				}

				if _, ok := skipJob[jobkey]; ok {
					// already looked up and skipped
					continue
				}

				// job does not found yet
				job, err := snapshot.JobByID(nil, alloc.Namespace, alloc.JobID)
				if err != nil {
					//FIXME
					panic(err)
				}

				// Don't bother collecting system jobs
				if job.Type == structs.JobTypeSystem {
					skipJob[jobkey] = struct{}{}
					s.logger.Printf("[TRACE] nomad.drain: skipping system job %s", job.Name)
					continue
				}

				// If alloc isn't yet terminal this node has
				// allocs left to be drained
				if !alloc.TerminalStatus() {
					if !allocsLeft {
						s.logger.Printf("[TRACE] nomad.drain: node %s has allocs left to drain", nodeID[:6])
						allocsLeft = true
					}
				}

				// Don't bother collecting batch jobs for nodes that haven't hit their deadline
				if job.Type == structs.JobTypeBatch && node.DrainStrategy.DeadlineTime().After(now) {
					s.logger.Printf("[TRACE] nomad.drain: not draining batch job %s because deadline isn't for %s", job.Name, node.DrainStrategy.DeadlineTime().Sub(now))
					skipJob[jobkey] = struct{}{}
					continue
				}

				jobAllocs, err := snapshot.AllocsByJob(nil, alloc.Namespace, alloc.JobID, true)
				if err != nil {
					//FIXME
					panic(err)
				}

				// Count the number of down (terminal or nil deployment status) per task group
				if job.Type == structs.JobTypeService {
					n := 0
					for _, a := range jobAllocs {
						if !a.TerminalStatus() && a.DeploymentStatus != nil {
							upPerTG[makeTaskGroupKey(a)]++
							n++
						}
					}
					s.logger.Printf("[TRACE] nomad.drain: job %s has %d task groups running", job.Name, n)
				}

				drainable[jobkey] = &drainingJob{
					job:    job,
					allocs: jobAllocs,
				}

				jobWatcher.watch(jobkey, nodeID)
			}

			// if node has no allocs, it's done draining!
			if !allocsLeft {
				s.logger.Printf("[TRACE] nomad.drain: node %s has no more allocs left to drain", nodeID)
				jobWatcher.nodeDone(nodeID)
				delete(nodes, nodeID)
				doneNodes[nodeID] = node
			}
		}

		// stoplist are the allocations to stop and their jobs to emit
		// evaluations for
		stoplist := &stopAllocs{
			allocBatch: make([]*structs.Allocation, 0, len(drainable)),
			jobBatch:   make(map[jobKey]*structs.Job),
		}

		// deadlineNodes is a map of node IDs that have reached their
		// deadline and allocs that will be stopped due to deadline
		deadlineNodes := map[string]int{}

		// build drain list considering deadline & max_parallel
		for _, drainingJob := range drainable {
			for _, alloc := range drainingJob.allocs {
				// Already draining/dead allocs don't need to be drained
				if alloc.TerminalStatus() {
					continue
				}

				node, ok := nodes[alloc.NodeID]
				if !ok {
					// Alloc's node is not draining so not elligible for draining!
					continue
				}

				tgKey := makeTaskGroupKey(alloc)

				if node.DrainStrategy.DeadlineTime().Before(now) {
					s.logger.Printf("[TRACE] nomad.drain: draining job %s alloc %s from node %s due to node's drain deadline", drainingJob.job.Name, alloc.ID[:6], alloc.NodeID[:6])
					// Alloc's Node has reached its deadline
					stoplist.add(drainingJob.job, alloc)
					upPerTG[tgKey]--

					deadlineNodes[node.ID]++
					continue
				}

				// Batch jobs are only stopped when the node
				// deadline is reached which has already been
				// done.
				if drainingJob.job.Type == structs.JobTypeBatch {
					continue
				}

				// Stop allocs with count=1, max_parallel==0, or draining<max_parallel
				tg := drainingJob.job.LookupTaskGroup(alloc.TaskGroup)
				//FIXME tg==nil here?

				// Only 1, drain
				if tg.Count == 1 {
					s.logger.Printf("[TRACE] nomad.drain: draining job %s alloc %s from node %s due to count=1", drainingJob.job.Name, alloc.ID[:6], alloc.NodeID[:6])
					stoplist.add(drainingJob.job, alloc)
					continue
				}

				// No migrate strategy or a max parallel of 0 mean force draining
				if tg.Migrate == nil || tg.Migrate.MaxParallel == 0 {
					s.logger.Printf("[TRACE] nomad.drain: draining job %s alloc %s from node %s due to force drain", drainingJob.job.Name, alloc.ID[:6], alloc.NodeID[:6])
					stoplist.add(drainingJob.job, alloc)
					continue
				}

				s.logger.Printf("[TRACE] nomad.drain: considering job %s alloc %s  count %d  maxp %d  up %d",
					drainingJob.job.Name, alloc.ID[:6], tg.Count, tg.Migrate.MaxParallel, upPerTG[tgKey])

				// Count - MaxParalell = minimum number of allocations that must be "up"
				minUp := (tg.Count - tg.Migrate.MaxParallel)

				// If minimum is < the current number up it is safe to stop one.
				if minUp < upPerTG[tgKey] {
					s.logger.Printf("[TRACE] nomad.drain: draining job %s alloc %s from node %s due to max parallel", drainingJob.job.Name, alloc.ID[:6], alloc.NodeID[:6])
					// More migrations are allowed, add to stoplist
					stoplist.add(drainingJob.job, alloc)
					upPerTG[tgKey]--
				}
			}
		}

		// log drains due to node deadlines
		for nodeID, remaining := range deadlineNodes {
			s.logger.Printf("[DEBUG] nomad.drain: node %s drain deadline reached; stopping %d remaining allocs", nodeID, remaining)
			jobWatcher.nodeDone(nodeID)
		}

		if len(stoplist.allocBatch) > 0 {
			s.logger.Printf("[DEBUG] nomad.drain: stopping %d alloc(s) for %d job(s)", len(stoplist.allocBatch), len(stoplist.jobBatch))

			// Stop allocs in stoplist and add them to drainingAllocs + prevAllocWatcher
			batch := &structs.AllocUpdateRequest{
				Alloc:        stoplist.allocBatch,
				WriteRequest: structs.WriteRequest{Region: s.config.Region},
			}

			// Commit this update via Raft
			//TODO Not the right request
			_, index, err := s.raftApply(structs.AllocClientUpdateRequestType, batch)
			if err != nil {
				//FIXME
				panic(err)
			}

			//TODO i bet there's something useful to do with this index
			_ = index

			// Reevaluate affected jobs
			evals := make([]*structs.Evaluation, 0, len(stoplist.jobBatch))
			for _, job := range stoplist.jobBatch {
				evals = append(evals, &structs.Evaluation{
					ID:             uuid.Generate(),
					Namespace:      job.Namespace,
					Priority:       job.Priority,
					Type:           job.Type,
					TriggeredBy:    structs.EvalTriggerNodeDrain,
					JobID:          job.ID,
					JobModifyIndex: job.ModifyIndex,
					Status:         structs.EvalStatusPending,
				})
			}

			evalUpdate := &structs.EvalUpdateRequest{
				Evals:        evals,
				WriteRequest: structs.WriteRequest{Region: s.config.Region},
			}

			// Commit this evaluation via Raft
			_, _, err = s.raftApply(structs.EvalUpdateRequestType, evalUpdate)
			if err != nil {
				//FIXME
				panic(err)
			}
		}

		// Unset drain for nodes done draining
		for nodeID, node := range doneNodes {
			args := structs.NodeUpdateDrainRequest{
				NodeID:       nodeID,
				Drain:        false,
				WriteRequest: structs.WriteRequest{Region: s.config.Region},
			}

			_, _, err := s.raftApply(structs.NodeUpdateDrainRequestType, &args)
			if err != nil {
				s.logger.Printf("[ERR] nomad.drain: failed to unset drain for: %v", err)
				//FIXME
				panic(err)
			}
			s.logger.Printf("[INFO] nomad.drain: node %s (%s) completed draining", nodeID, node.Name)
		}
	}
}

// nodeWatcher watches for nodes to start or stop draining
type nodeWatcher struct {
	index   uint64
	nodes   map[string]*structs.Node
	nodesCh chan map[string]*structs.Node
	state   *state.StateStore
	logger  *log.Logger
}

func newNodeWatcher(logger *log.Logger, nodes map[string]*structs.Node, index uint64, state *state.StateStore) *nodeWatcher {
	return &nodeWatcher{
		nodes:   nodes,
		nodesCh: make(chan map[string]*structs.Node),
		index:   index,
		state:   state,
		logger:  logger,
	}
}

func (n *nodeWatcher) run(ctx context.Context) {
	// Trigger an initial drain pass if there are already nodes draining
	//FIXME this is unneccessary if a node has reached a deadline
	n.logger.Printf("[TRACE] nomad.drain: initial draining nodes: %d", len(n.nodes))
	if len(n.nodes) > 0 {
		n.nodesCh <- n.nodes
	}

	for {
		//FIXME it seems possible for this to return a nil error and a 0 index, what to do in that case?
		resp, index, err := n.state.BlockingQuery(n.queryNodeDrain, n.index, ctx)
		if err != nil {
			if err == context.Canceled {
				n.logger.Printf("[TRACE] nomad.drain: draining node watcher shutting down")
				return
			}
			n.logger.Printf("[ERR] nomad.drain: error blocking on node updates at index %d: %v", n.index, err)
			return
		}

		// update index for next run
		n.index = index

		changed := false
		newNodes := resp.([]*structs.Node)
		n.logger.Printf("[TRACE] nomad.drain: %d nodes to consider", len(newNodes)) //FIXME remove
		for _, newNode := range newNodes {
			if existingNode, ok := n.nodes[newNode.ID]; ok {
				// Node was draining, see if it has changed
				if !newNode.Drain {
					// Node stopped draining
					delete(n.nodes, newNode.ID)
					changed = true
				} else if !newNode.DrainStrategy.DeadlineTime().Equal(existingNode.DrainStrategy.DeadlineTime()) {
					// Update deadline
					n.nodes[newNode.ID] = newNode
					changed = true
				}
			} else {
				// Node was not draining
				if newNode.Drain {
					// Node started draining
					n.nodes[newNode.ID] = newNode
					changed = true
				}
			}
		}

		// Send a copy of the draining nodes if there were changes
		if !changed {
			continue
		}

		nodesCopy := make(map[string]*structs.Node, len(n.nodes))
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

type jobWatcher struct {
	// allocsIndex to start watching from
	allocsIndex uint64

	// job -> node.ID
	jobs   map[jobKey]string
	jobsMu sync.Mutex

	jobsCh chan map[jobKey]struct{}

	state *state.StateStore

	logger *log.Logger
}

func newJobWatcher(logger *log.Logger, jobs map[jobKey]string, allocsIndex uint64, state *state.StateStore) *jobWatcher {
	return &jobWatcher{
		allocsIndex: allocsIndex,
		logger:      logger,
		jobs:        jobs,
		jobsCh:      make(chan map[jobKey]struct{}),
		state:       state,
	}
}

func (j *jobWatcher) watch(k jobKey, nodeID string) {
	j.logger.Printf("[TRACE] nomad.drain: watching job %s on draining node %s", k.jobid, nodeID[:6])
	j.jobsMu.Lock()
	j.jobs[k] = nodeID
	j.jobsMu.Unlock()
}

func (j *jobWatcher) nodeDone(nodeID string) {
	j.jobsMu.Lock()
	defer j.jobsMu.Unlock()
	for k, v := range j.jobs {
		if v == nodeID {
			j.logger.Printf("[TRACE] nomad.drain: UNwatching job %s on done draining node %s", k.jobid, nodeID[:6])
			delete(j.jobs, k)
		}
	}
}

func (j *jobWatcher) WaitCh() <-chan map[jobKey]struct{} {
	return j.jobsCh
}

func (j *jobWatcher) run(ctx context.Context) {
	var resp interface{}
	var err error

	for {
		//FIXME have watchAllocs create a closure and give it a copy of j.jobs to remove locking?
		//FIXME it seems possible for this to return a nil error and a 0 index, what to do in that case?
		var newIndex uint64
		resp, newIndex, err = j.state.BlockingQuery(j.watchAllocs, j.allocsIndex, ctx)
		if err != nil {
			if err == context.Canceled {
				j.logger.Printf("[TRACE] nomad.drain: job watcher shutting down")
				return
			}
			j.logger.Printf("[ERR] nomad.drain: error blocking on alloc updates: %v", err)
			return
		}

		j.logger.Printf("[TRACE] nomad.drain: job watcher old index: %d new index: %d", j.allocsIndex, newIndex)
		j.allocsIndex = newIndex

		changedJobs := resp.(map[jobKey]struct{})
		if len(changedJobs) > 0 {
			select {
			case j.jobsCh <- changedJobs:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (j *jobWatcher) watchAllocs(ws memdb.WatchSet, state *state.StateStore) (interface{}, uint64, error) {
	iter, err := state.Allocs(ws)
	if err != nil {
		return nil, 0, err
	}

	index, err := state.Index("allocs")
	if err != nil {
		return nil, 0, err
	}

	skipped := 0

	// job ids
	resp := map[jobKey]struct{}{}

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		alloc := raw.(*structs.Allocation)

		j.jobsMu.Lock()
		_, ok := j.jobs[jobKey{alloc.Namespace, alloc.JobID}]
		j.jobsMu.Unlock()

		if !ok {
			// alloc is not part of a draining job
			skipped++
			continue
		}

		// don't wake drain loop if alloc hasn't updated its health
		if alloc.DeploymentStatus.IsHealthy() || alloc.DeploymentStatus.IsUnhealthy() {
			j.logger.Printf("[TRACE] nomad.drain: job watcher found alloc %s - deployment status: %t", alloc.ID[:6], *alloc.DeploymentStatus.Healthy)
			resp[jobKey{alloc.Namespace, alloc.JobID}] = struct{}{}
		} else {
			j.logger.Printf("[TRACE] nomad.drain: job watcher ignoring alloc %s - no deployment status", alloc.ID[:6])
		}
	}

	j.logger.Printf("[TRACE] nomad.drain: job watcher ignoring %d allocs - not part of draining job at index %d", skipped, index)

	return resp, index, nil
}

// initDrainer initializes the node drainer state and returns a list of
// draining nodes as well as allocs that are draining that should be watched
// for a replacement.
func initDrainer(logger *log.Logger, state *state.StateStore) (map[string]*structs.Node, uint64, map[jobKey]string, uint64) {
	// StateStore.Snapshot never returns an error so don't bother checking it
	snapshot, _ := state.Snapshot()
	now := time.Now()

	iter, err := snapshot.Nodes(nil)
	if err != nil {
		logger.Printf("[ERR] nomad.drain: error iterating nodes: %v", err)
		panic(err) //FIXME
	}

	// map of draining nodes keyed by node ID
	nodes := map[string]*structs.Node{}

	// map of draining job IDs keyed by {namespace, job id} -> node.ID
	jobs := map[jobKey]string{}

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

		// Track draining node
		nodes[node.ID] = node

		// No point in tracking draining allocs as the deadline has been reached
		if node.DrainStrategy.DeadlineTime().Before(now) {
			continue
		}

		allocs, err := snapshot.AllocsByNode(nil, node.ID)
		if err != nil {
			logger.Printf("[ERR] nomad.drain: error iterating allocs for node %q: %v", node.ID, err)
			panic(err) //FIXME
		}

		for _, alloc := range allocs {
			jobs[jobKey{alloc.Namespace, alloc.JobID}] = node.ID
		}
	}

	nodesIndex, _ := snapshot.Index("nodes")
	if nodesIndex == 0 {
		nodesIndex = 1
	}
	allocsIndex, _ := snapshot.Index("allocs")
	if allocsIndex == 0 {
		allocsIndex = 1
	}
	return nodes, nodesIndex, jobs, allocsIndex
}
