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

// runningJob contains the Job and allocations for that job meant to be used
// when collecting all allocations for a job with at least one allocation on a
// draining node.
//
// In order to drain an allocation we must also emit an evaluation for its job,
// so this struct bundles allocations with their job.
type runningJob struct {
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

// stopAllocs tracks allocs to drain by a unique TG key along with their jobs
// as we need to emit evaluations for each allocations job
type stopAllocs struct {
	allocBatch map[string]*structs.DesiredTransition

	// namespace+jobid -> Job
	jobBatch map[jobKey]*structs.Job
}

// newStopAllocs creates a list of allocs to migrate from an initial list of
// running jobs+allocs that need immediate draining.
func newStopAllocs(initial map[jobKey]*runningJob) *stopAllocs {
	s := &stopAllocs{
		allocBatch: make(map[string]*structs.DesiredTransition),
		jobBatch:   make(map[jobKey]*structs.Job),
	}

	// Add initial allocs
	for _, drainingJob := range initial {
		for _, a := range drainingJob.allocs {
			s.add(drainingJob.job, a)
		}
	}
	return s
}

// add an allocation to be migrated. Its job must also be specified in order to
// emit an evaluation.
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

// getNextDeadline is a helper that takes a set of draining nodes and returns the
// next deadline. It also returns a boolean if there is a deadline.
func getNextDeadline(nodes map[string]*structs.Node) (time.Time, bool) {
	var nextDeadline time.Time
	found := false
	for _, node := range nodes {
		inf, d := node.DrainStrategy.DeadlineTime()
		if !inf && (nextDeadline.IsZero() || d.Before(nextDeadline)) {
			nextDeadline = d
			found = true
		}
	}

	return nextDeadline, found
}

// nodeDrainer is the core node draining main loop and should be started in a
// goroutine when a server establishes leadership.
func (n *NodeDrainer) nodeDrainer(ctx context.Context, state *state.StateStore) {
	nodes, nodesIndex, drainingJobs, allocsIndex := initDrainer(n.logger, state)

	// Wait for a node's drain deadline to expire
	nextDeadline, ok := getNextDeadline(nodes)
	deadlineTimer := time.NewTimer(time.Until(nextDeadline))
	stopDeadlineTimer := func() {
		if !deadlineTimer.Stop() {
			select {
			case <-deadlineTimer.C:
			default:
			}
		}
	}
	if !ok {
		stopDeadlineTimer()
	}

	// Watch for nodes to start or stop draining
	nodeWatcher := newNodeWatcher(n.logger, nodes, nodesIndex, state)
	go nodeWatcher.run(ctx)

	// Watch for drained allocations to be replaced
	// Watch for changes in allocs for jobs with allocs on draining nodes
	jobWatcher := newJobWatcher(n.logger, drainingJobs, allocsIndex, state)
	go jobWatcher.run(ctx)

	for {
		n.logger.Printf("[TRACE] nomad.drain: LOOP next deadline: %s (%s)", nextDeadline, time.Until(nextDeadline))
		select {
		case nodes = <-nodeWatcher.nodesCh:
			// update draining nodes
			n.logger.Printf("[TRACE] nomad.drain: running due to node change (%d nodes draining)", len(nodes))

			d, ok := getNextDeadline(nodes)
			if ok && !nextDeadline.Equal(d) {
				nextDeadline = d
				n.logger.Printf("[TRACE] nomad.drain: new node deadline: %s", nextDeadline)
				stopDeadlineTimer()
				deadlineTimer.Reset(time.Until(nextDeadline))
			} else if !ok {
				stopDeadlineTimer()
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
		// non-terminal alloc on a draining node.
		// Invariants:
		//  - Only service jobs
		//  - No entries with 0 allocs
		//TODO could this be a helper method on prevAllocWatcher
		drainableSvcs := map[jobKey]*runningJob{}

		// drainNow are allocs for batch or system jobs that should be
		// drained due to a node deadline being reached
		drainNow := map[jobKey]*runningJob{}

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

			// drainableSys are allocs for system jobs that should be
			// drained if there are no other allocs left
			drainableSys := map[jobKey]*runningJob{}

			// track number of allocs left on this node to be drained
			allocsLeft := false
			inf, deadline := node.DrainStrategy.DeadlineTime()
			deadlineReached := !inf && deadline.Before(now)
			for _, alloc := range allocs {
				// Don't need to consider drained allocs
				if alloc.TerminalStatus() {
					continue
				}

				jobkey := jobKey{alloc.Namespace, alloc.JobID}

				// job does not found yet
				job, err := snapshot.JobByID(nil, alloc.Namespace, alloc.JobID)
				if err != nil {
					//FIXME
					panic(err)
				}

				// IgnoreSystemJobs if specified in the node's DrainStrategy
				if node.DrainStrategy.IgnoreSystemJobs && job.Type == structs.JobTypeSystem {
					continue
				}

				// When the node deadline is reached all batch
				// and service jobs will be drained
				if deadlineReached && job.Type != structs.JobTypeService {
					n.logger.Printf("[TRACE] nomad.drain: draining alloc %s due to node %s reaching drain deadline", alloc.ID, node.ID)
					if j, ok := drainNow[jobkey]; ok {
						j.allocs = append(j.allocs, alloc)
					} else {
						// First alloc for this job, create entry
						drainNow[jobkey] = &runningJob{
							job:    job,
							allocs: []*structs.Allocation{alloc},
						}
					}
					continue
				}

				// If deadline hasn't been reached, system jobs
				// may still be drained if there are no other
				// allocs left
				if !deadlineReached && job.Type == structs.JobTypeSystem {
					n.logger.Printf("[TRACE] nomad.drain: system alloc %s will be drained if no other allocs on node %s", alloc.ID, node.ID)
					if j, ok := drainableSys[jobkey]; ok {
						j.allocs = append(j.allocs, alloc)
					} else {
						// First alloc for this job, create entry
						drainableSys[jobkey] = &runningJob{
							job:    job,
							allocs: []*structs.Allocation{alloc},
						}
					}
					continue
				}

				// This alloc is still running on a draining
				// node, so treat the node as having allocs
				// remaining
				allocsLeft = true

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
							// Not terminal and health updated, count it as up!
							upPerTG[makeTaskGroupKey(a)]++
							num++
						}
					}
					n.logger.Printf("[TRACE] nomad.drain: job %s has %d allocs running", job.Name, num)
				}

				drainableSvcs[jobkey] = &runningJob{
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

				// Add all system jobs on this node to the drainNow slice
				for k, sysj := range drainableSys {
					if j, ok := drainNow[k]; ok {
						// Job already has at least one alloc draining, append this one
						j.allocs = append(j.allocs, sysj.allocs...)
					} else {
						// First draining alloc for this job, add the entry
						drainNow[k] = sysj
					}
				}
			}
		}

		// stoplist are the allocations to migrate and their jobs to emit
		// evaluations for. Initialized with allocations that should be
		// immediately drained regardless of MaxParallel
		stoplist := newStopAllocs(drainNow)

		// build drain list considering deadline & max_parallel
		for _, drainingJob := range drainableSvcs {
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

				if inf, d := node.DrainStrategy.DeadlineTime(); !inf && d.Before(now) {
					n.logger.Printf("[TRACE] nomad.drain: draining job %s alloc %s from node %s due to node's drain deadline", drainingJob.job.Name, alloc.ID[:6], alloc.NodeID[:6])
					// Alloc's Node has reached its deadline
					stoplist.add(drainingJob.job, alloc)
					upPerTG[tgKey]--

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
			if err := n.applyMigrations(stoplist); err != nil {
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

// applyMigrations applies the specified allocation migrations along with their
// evaluations to raft.
func (n *NodeDrainer) applyMigrations(stoplist *stopAllocs) error {
	n.logger.Printf("[DEBUG] nomad.drain: stopping %d alloc(s) for %d job(s)", len(stoplist.allocBatch), len(stoplist.jobBatch))

	for id, _ := range stoplist.allocBatch {
		n.logger.Printf("[TRACE] nomad.drain: migrating alloc %s", id[:6])
	}
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
	return n.raft.AllocUpdateDesiredTransition(stoplist.allocBatch, evals)
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
		if inf, d := node.DrainStrategy.DeadlineTime(); !inf && d.Before(now) {
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
