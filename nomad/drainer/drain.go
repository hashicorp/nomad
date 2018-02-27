package drainer

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/nomad/helper"
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
	allocBatch map[string]*structs.DesiredTransition

	// namespace+jobid -> Job
	jobBatch map[jobKey]*structs.Job
}

func (s *stopAllocs) add(j *structs.Job, a *structs.Allocation) {
	// Add the desired migration transition to the batch
	s.allocBatch[a.ID] = &structs.DesiredTransition{
		Migrate: helper.BoolToPtr(true),
	}

	// Add job to the job batch
	s.jobBatch[jobKey{a.Namespace, a.JobID}] = j
}

// RaftApplier contains methods for applying the raft requests required by the
// NodeDrainer.
type RaftApplier interface {
	AllocUpdateDesiredTransition(allocs map[string]*structs.DesiredTransition, evals []*structs.Evaluation) error
	NodeDrainComplete(nodeID string) error
}

// nodeDrainerState is used to communicate the state set by
// NodeDrainer.SetEnabled to the concurrently executing Run loop.
type nodeDrainerState struct {
	enabled bool
	state   *state.StateStore
}

// NodeDrainer migrates allocations off of draining nodes. SetEnabled(true)
// should be called when a server establishes leadership and SetEnabled(false)
// called when leadership is lost.
type NodeDrainer struct {
	enabledCh chan nodeDrainerState

	raft RaftApplier

	shutdownCh <-chan struct{}

	logger *log.Logger
}

// NewNodeDrainer creates a new NodeDrainer which will exit when shutdownCh is
// closed. A RaftApplier shim must be supplied to allow NodeDrainer access to
// the raft messages it sends.
func NewNodeDrainer(logger *log.Logger, shutdownCh <-chan struct{}, raft RaftApplier) *NodeDrainer {
	return &NodeDrainer{
		enabledCh:  make(chan nodeDrainerState),
		raft:       raft,
		shutdownCh: shutdownCh,
		logger:     logger,
	}
}

// SetEnabled will start or stop the node draining goroutine depending on the
// enabled boolean. SetEnabled is meant to be called concurrently with Run.
func (n *NodeDrainer) SetEnabled(enabled bool, state *state.StateStore) {
	select {
	case n.enabledCh <- nodeDrainerState{enabled, state}:
	case <-n.shutdownCh:
	}
}

// Run monitors the shutdown chan as well as SetEnabled calls and starts/stops
// the node draining goroutine appropriately. As it blocks it should be called
// in a goroutine.
func (n *NodeDrainer) Run() {
	running := false
	var s nodeDrainerState
	ctx, cancel := context.WithCancel(context.Background())
	for {
		select {
		case s = <-n.enabledCh:
		case <-n.shutdownCh:
			// Stop drainer and exit
			cancel()
			return
		}

		switch {
		case s.enabled && running:
			// Already running, must restart to ensure the latest StateStore is used
			cancel()
			ctx, cancel = context.WithCancel(context.Background())
			go n.nodeDrainer(ctx, s.state)

		case !s.enabled && !running:
			// Already stopped; nothing to do

		case !s.enabled && running:
			// Stop running node drainer
			cancel()
			running = false

		case s.enabled && !running:
			// Start running node drainer
			ctx, cancel = context.WithCancel(context.Background())
			go n.nodeDrainer(ctx, s.state)
			running = true
		}
	}
}

// nodeDrainer is the core node draining main loop and should be started in a
// goroutine when a server establishes leadership.
func (n *NodeDrainer) nodeDrainer(ctx context.Context, state *state.StateStore) {
	nodes, nodesIndex, drainingJobs, allocsIndex := initDrainer(n.logger, state)

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
	nodeWatcher := newNodeWatcher(n.logger, nodes, nodesIndex, state)
	go nodeWatcher.run(ctx)

	// Watch for drained allocations to be replaced
	// Watch for changes in allocs for jobs with allocs on draining nodes
	jobWatcher := newJobWatcher(n.logger, drainingJobs, allocsIndex, state)
	go jobWatcher.run(ctx)

	for {
		//TODO this method of async node updates means we could make
		//migration decisions on out of date information. the worst
		//possible outcome of this is that an allocation could be
		//stopped on a node that recently had its drain cancelled which
		//doesn't seem like that bad of a pathological case
		n.logger.Printf("[TRACE] nomad.drain: LOOP next deadline: %s (%s)", nextDeadline, time.Until(nextDeadline))
		select {
		case nodes = <-nodeWatcher.nodesCh:
			// update draining nodes
			n.logger.Printf("[TRACE] nomad.drain: running due to node change (%d nodes draining)", len(nodes))

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
				n.logger.Printf("[TRACE] nomad.drain: new node deadline: %s", nextDeadline)
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
			n.logger.Printf("[TRACE] nomad.drain: running due to alloc change (%d jobs updated)", len(jobs))
		case when := <-deadlineTimer.C:
			// deadline for a node was reached
			n.logger.Printf("[TRACE] nomad.drain: running due to deadline reached (at %s)", when)
		case <-ctx.Done():
			// exit
			return
		}

		// Tracks nodes that are done draining
		doneNodes := map[string]*structs.Node{}

		// Capture state (statestore and time) to do consistent comparisons
		snapshot, err := state.Snapshot()
		if err != nil {
			//FIXME
			panic(err)
		}
		now := time.Now()

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
			deadlineReached := node.DrainStrategy.DeadlineTime().Before(now)
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

				// If alloc isn't yet terminal this node has
				// allocs left to be drained
				if !alloc.TerminalStatus() {
					if !allocsLeft {
						n.logger.Printf("[TRACE] nomad.drain: node %s has allocs left to drain", nodeID[:6])
						allocsLeft = true
					}
				}

				// Don't bother collecting system/batch jobs for nodes that haven't hit their deadline
				if job.Type != structs.JobTypeService && !deadlineReached {
					n.logger.Printf("[TRACE] nomad.drain: not draining %s job %s because deadline isn't for %s",
						job.Type, job.Name, node.DrainStrategy.DeadlineTime().Sub(now))
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
					num := 0
					for _, a := range jobAllocs {
						if !a.TerminalStatus() && a.DeploymentStatus != nil {
							upPerTG[makeTaskGroupKey(a)]++
							num++
						}
					}
					n.logger.Printf("[TRACE] nomad.drain: job %s has %d task groups running", job.Name, num)
				}

				drainable[jobkey] = &drainingJob{
					job:    job,
					allocs: jobAllocs,
				}

				jobWatcher.watch(jobkey, nodeID)
			}

			// if node has no allocs or has hit its deadline, it's done draining!
			if !allocsLeft || deadlineReached {
				n.logger.Printf("[TRACE] nomad.drain: node %s has no more allocs left to drain or has reached deadline", nodeID)
				jobWatcher.nodeDone(nodeID)
				doneNodes[nodeID] = node
			}
		}

		// stoplist are the allocations to migrate and their jobs to emit
		// evaluations for
		stoplist := &stopAllocs{
			allocBatch: make(map[string]*structs.DesiredTransition),
			jobBatch:   make(map[jobKey]*structs.Job),
		}

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
					n.logger.Printf("[TRACE] nomad.drain: draining job %s alloc %s from node %s due to node's drain deadline", drainingJob.job.Name, alloc.ID[:6], alloc.NodeID[:6])
					// Alloc's Node has reached its deadline
					stoplist.add(drainingJob.job, alloc)
					upPerTG[tgKey]--

					continue
				}

				// Batch/System jobs are only stopped when the
				// node deadline is reached which has already
				// been done.
				if drainingJob.job.Type != structs.JobTypeService {
					continue
				}

				// Stop allocs with count=1, max_parallel==0, or draining<max_parallel
				tg := drainingJob.job.LookupTaskGroup(alloc.TaskGroup)
				//FIXME tg==nil here?

				// Only 1, drain
				if tg.Count == 1 {
					n.logger.Printf("[TRACE] nomad.drain: draining job %s alloc %s from node %s due to count=1", drainingJob.job.Name, alloc.ID[:6], alloc.NodeID[:6])
					stoplist.add(drainingJob.job, alloc)
					continue
				}

				// No migrate strategy or a max parallel of 0 mean force draining
				if tg.Migrate == nil || tg.Migrate.MaxParallel == 0 {
					n.logger.Printf("[TRACE] nomad.drain: draining job %s alloc %s from node %s due to force drain", drainingJob.job.Name, alloc.ID[:6], alloc.NodeID[:6])
					stoplist.add(drainingJob.job, alloc)
					continue
				}

				n.logger.Printf("[TRACE] nomad.drain: considering job %s alloc %s  count %d  maxp %d  up %d",
					drainingJob.job.Name, alloc.ID[:6], tg.Count, tg.Migrate.MaxParallel, upPerTG[tgKey])

				// Count - MaxParalell = minimum number of allocations that must be "up"
				minUp := (tg.Count - tg.Migrate.MaxParallel)

				// If minimum is < the current number up it is safe to stop one.
				if minUp < upPerTG[tgKey] {
					n.logger.Printf("[TRACE] nomad.drain: draining job %s alloc %s from node %s due to max parallel", drainingJob.job.Name, alloc.ID[:6], alloc.NodeID[:6])
					// More migrations are allowed, add to stoplist
					stoplist.add(drainingJob.job, alloc)
					upPerTG[tgKey]--
				}
			}
		}

		if len(stoplist.allocBatch) > 0 {
			n.logger.Printf("[DEBUG] nomad.drain: stopping %d alloc(s) for %d job(s)", len(stoplist.allocBatch), len(stoplist.jobBatch))

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

			// Commit this update via Raft
			if err := n.raft.AllocUpdateDesiredTransition(stoplist.allocBatch, evals); err != nil {
				//FIXME
				panic(err)
			}
		}

		// Unset drain for nodes done draining
		for nodeID, node := range doneNodes {
			if err := n.raft.NodeDrainComplete(nodeID); err != nil {
				n.logger.Printf("[ERR] nomad.drain: failed to unset drain for: %v", err)
				//FIXME
				panic(err)
			}
			n.logger.Printf("[INFO] nomad.drain: node %s (%s) completed draining", nodeID, node.Name)
			delete(nodes, nodeID)
		}
	}
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
