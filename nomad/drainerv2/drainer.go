package drainerv2

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"golang.org/x/time/rate"
)

var (
	// stateReadErrorDelay is the delay to apply before retrying reading state
	// when there is an error
	stateReadErrorDelay = 1 * time.Second
)

const (
	// LimitStateQueriesPerSecond is the number of state queries allowed per
	// second
	LimitStateQueriesPerSecond = 100.0

	// BatchUpdateInterval is how long we wait to batch updates
	BatchUpdateInterval = 1 * time.Second

	// NodeDeadlineCoalesceWindow is the duration in which deadlining nodes will
	// be coalesced together
	NodeDeadlineCoalesceWindow = 5 * time.Second
)

// RaftApplier contains methods for applying the raft requests required by the
// NodeDrainer.
type RaftApplier interface {
	AllocUpdateDesiredTransition(allocs map[string]*structs.DesiredTransition, evals []*structs.Evaluation) (uint64, error)
	NodeDrainComplete(nodeID string) (uint64, error)
}

type NodeTracker interface {
	TrackedNodes() map[string]*structs.Node
	Remove(nodeID string)
	Update(node *structs.Node)
}

type DrainingJobWatcherFactory func(context.Context, *rate.Limiter, *state.StateStore, *log.Logger) DrainingJobWatcher
type DrainingNodeWatcherFactory func(context.Context, *rate.Limiter, *state.StateStore, *log.Logger, NodeTracker) DrainingNodeWatcher
type DrainDeadlineNotifierFactory func(context.Context) DrainDeadlineNotifier

func GetDrainingJobWatcher(ctx context.Context, limiter *rate.Limiter, state *state.StateStore, logger *log.Logger) DrainingJobWatcher {
	return NewDrainingJobWatcher(ctx, limiter, state, logger)
}

func GetDeadlineNotifier(ctx context.Context) DrainDeadlineNotifier {
	return NewDeadlineHeap(ctx, NodeDeadlineCoalesceWindow)
}

func GetNodeWatcherFactory() DrainingNodeWatcherFactory {
	return func(ctx context.Context, limiter *rate.Limiter, state *state.StateStore, logger *log.Logger, tracker NodeTracker) DrainingNodeWatcher {
		return NewNodeDrainWatcher(ctx, limiter, state, logger, tracker)
	}
}

type allocMigrateBatcher struct {
	// updates holds pending client status updates for allocations
	updates []*structs.Allocation

	// updateFuture is used to wait for the pending batch update
	// to complete. This may be nil if no batch is pending.
	updateFuture *structs.BatchFuture

	// updateTimer is the timer that will trigger the next batch
	// update, and may be nil if there is no batch pending.
	updateTimer *time.Timer

	batchWindow time.Duration

	// synchronizes access to the updates list, the future and the timer.
	sync.Mutex
}

type NodeDrainerConfig struct {
	Logger                *log.Logger
	Raft                  RaftApplier
	JobFactory            DrainingJobWatcherFactory
	NodeFactory           DrainingNodeWatcherFactory
	DrainDeadlineFactory  DrainDeadlineNotifierFactory
	StateQueriesPerSecond float64
	BatchUpdateInterval   time.Duration
}

// TODO Add stats
type NodeDrainer struct {
	enabled bool
	logger  *log.Logger

	// nodes is the set of draining nodes
	nodes map[string]*drainingNode

	nodeWatcher DrainingNodeWatcher
	nodeFactory DrainingNodeWatcherFactory

	jobWatcher DrainingJobWatcher
	jobFactory DrainingJobWatcherFactory

	deadlineNotifier        DrainDeadlineNotifier
	deadlineNotifierFactory DrainDeadlineNotifierFactory

	// state is the state that is watched for state changes.
	state *state.StateStore

	// queryLimiter is used to limit the rate of blocking queries
	queryLimiter *rate.Limiter

	// raft is a shim around the raft messages necessary for draining
	raft RaftApplier

	// batcher is used to batch alloc migrations.
	batcher allocMigrateBatcher

	// ctx and exitFn are used to cancel the watcher
	ctx    context.Context
	exitFn context.CancelFunc

	l sync.RWMutex
}

func NewNodeDrainer(c *NodeDrainerConfig) *NodeDrainer {
	return &NodeDrainer{
		raft:                    c.Raft,
		logger:                  c.Logger,
		jobFactory:              c.JobFactory,
		nodeFactory:             c.NodeFactory,
		deadlineNotifierFactory: c.DrainDeadlineFactory,
		queryLimiter:            rate.NewLimiter(rate.Limit(c.StateQueriesPerSecond), 100),
		batcher: allocMigrateBatcher{
			batchWindow: c.BatchUpdateInterval,
		},
	}
}

// SetEnabled will start or stop the node draining goroutine depending on the
// enabled boolean.
func (n *NodeDrainer) SetEnabled(enabled bool, state *state.StateStore) {
	n.l.Lock()
	defer n.l.Unlock()

	wasEnabled := n.enabled
	n.enabled = enabled

	if state != nil {
		n.state = state
	}

	// Flush the state to create the necessary objects
	n.flush()

	// If we are starting now, launch the watch daemon
	if enabled && !wasEnabled {
		n.run(n.ctx)
	}
}

// flush is used to clear the state of the watcher
func (n *NodeDrainer) flush() {
	// Kill everything associated with the watcher
	if n.exitFn != nil {
		n.exitFn()
	}

	n.ctx, n.exitFn = context.WithCancel(context.Background())
	n.jobWatcher = n.jobFactory(n.ctx, n.queryLimiter, n.state, n.logger)
	n.nodeWatcher = n.nodeFactory(n.ctx, n.queryLimiter, n.state, n.logger, n)
	n.deadlineNotifier = n.deadlineNotifierFactory(n.ctx)
	n.nodes = make(map[string]*drainingNode, 32)
}

func (n *NodeDrainer) run(ctx context.Context) {
	for {
		select {
		case <-n.ctx.Done():
			return
		case nodes := <-n.deadlineNotifier.NextBatch():
			n.handleDeadlinedNodes(nodes)
		case req := <-n.jobWatcher.Drain():
			n.handleJobAllocDrain(req)
		case allocs := <-n.jobWatcher.Migrated():
			n.handleMigratedAllocs(allocs)
		}
	}
}

func (n *NodeDrainer) handleDeadlinedNodes(nodes []string) {
	// Retrieve the set of allocations that will be force stopped.
	n.l.RLock()
	var forceStop []*structs.Allocation
	for _, node := range nodes {
		draining, ok := n.nodes[node]
		if !ok {
			n.logger.Printf("[DEBUG] nomad.node_drainer: skipping untracked deadlined node %q", node)
			continue
		}

		allocs, err := draining.DeadlineAllocs()
		if err != nil {
			n.logger.Printf("[ERR] nomad.node_drainer: failed to retrive allocs on deadlined node %q: %v", node, err)
			continue
		}

		forceStop = append(forceStop, allocs...)
	}
	n.l.RUnlock()
	n.batchDrainAllocs(forceStop)
}

func (n *NodeDrainer) handleJobAllocDrain(req *DrainRequest) {
	// This should be syncronous
	index, err := n.batchDrainAllocs(req.Allocs)
	req.Resp.Respond(index, err)
}

func (n *NodeDrainer) handleMigratedAllocs(allocs []*structs.Allocation) {
	// Determine the set of nodes that were effected
	nodes := make(map[string]struct{})
	for _, alloc := range allocs {
		nodes[alloc.NodeID] = struct{}{}
	}

	// For each node, check if it is now done
	n.l.RLock()
	var done []string
	for node := range nodes {
		draining, ok := n.nodes[node]
		if !ok {
			continue
		}

		isDone, err := draining.IsDone()
		if err != nil {
			n.logger.Printf("[ERR] nomad.drain: checking if node %q is done draining: %v", node, err)
			continue
		}

		if !isDone {
			continue
		}

		done = append(done, node)
	}
	n.l.RUnlock()

	// TODO This should probably be a single Raft transaction
	for _, doneNode := range done {
		index, err := n.raft.NodeDrainComplete(doneNode)
		if err != nil {
			n.logger.Printf("[ERR] nomad.drain: failed to unset drain for node %q: %v", doneNode, err)
		} else {
			n.logger.Printf("[INFO] nomad.drain: node %q completed draining at index %d", doneNode, index)
		}
	}
}

func (n *NodeDrainer) batchDrainAllocs(allocs []*structs.Allocation) (uint64, error) {
	// Add this to the batch
	n.batcher.Lock()
	n.batcher.updates = append(n.batcher.updates, allocs...)

	// Start a new batch if none
	future := n.batcher.updateFuture
	if future == nil {
		future = structs.NewBatchFuture()
		n.batcher.updateFuture = future
		n.batcher.updateTimer = time.AfterFunc(n.batcher.batchWindow, func() {
			// Get the pending updates
			n.batcher.Lock()
			updates := n.batcher.updates
			future := n.batcher.updateFuture
			n.batcher.updates = nil
			n.batcher.updateFuture = nil
			n.batcher.updateTimer = nil
			n.batcher.Unlock()

			// Perform the batch update
			n.drainAllocs(future, updates)
		})
	}
	n.batcher.Unlock()

	// Wait for the future
	if err := future.Wait(); err != nil {
		return 0, err
	}

	return future.Index(), nil
}

func (n *NodeDrainer) drainAllocs(future *structs.BatchFuture, allocs []*structs.Allocation) {
	// TODO This should shard to limit the size of the transaction.

	// Compute the effected jobs and make the transistion map
	jobs := make(map[string]*structs.Allocation, 4)
	transistions := make(map[string]*structs.DesiredTransition, len(allocs))
	for _, alloc := range allocs {
		transistions[alloc.ID] = &structs.DesiredTransition{
			Migrate: helper.BoolToPtr(true),
		}
		jobs[alloc.JobID] = alloc
	}

	evals := make([]*structs.Evaluation, 0, len(jobs))
	for job, alloc := range jobs {
		evals = append(evals, &structs.Evaluation{
			ID:          uuid.Generate(),
			Namespace:   alloc.Namespace,
			Priority:    alloc.Job.Priority,
			Type:        alloc.Job.Type,
			TriggeredBy: structs.EvalTriggerNodeDrain,
			JobID:       job,
			Status:      structs.EvalStatusPending,
		})
	}

	// Commit this update via Raft
	index, err := n.raft.AllocUpdateDesiredTransition(transistions, evals)
	future.Respond(index, err)
}
