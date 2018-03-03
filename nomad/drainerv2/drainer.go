package drainerv2

import (
	"context"
	"log"
	"sync"
	"time"

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
)

// RaftApplier contains methods for applying the raft requests required by the
// NodeDrainer.
type RaftApplier interface {
	AllocUpdateDesiredTransition(allocs map[string]*structs.DesiredTransition, evals []*structs.Evaluation) error
	NodeDrainComplete(nodeID string) error
}

type AllocDrainer interface {
	drain(allocs []*structs.Allocation)
}

type NodeTracker interface {
	Tracking(nodeID string) (*structs.Node, bool)
	Remove(nodeID string)
	Update(node *structs.Node)
}

type DrainingJobWatcherFactory func(context.Context, *rate.Limiter, *state.StateStore, *log.Logger, AllocDrainer) DrainingJobWatcher
type DrainingNodeWatcherFactory func(context.Context, *rate.Limiter, *state.StateStore, *log.Logger, NodeTracker) DrainingNodeWatcher
type DrainDeadlineNotifierFactory func(context.Context) DrainDeadlineNotifier

type NodeDrainerConfig struct {
	Logger                *log.Logger
	Raft                  RaftApplier
	JobFactory            DrainingJobWatcherFactory
	NodeFactory           DrainingNodeWatcherFactory
	DrainDeadlineFactory  DrainDeadlineNotifierFactory
	StateQueriesPerSecond float64
}

type NodeDrainer struct {
	enabled bool
	logger  *log.Logger

	// nodes is the set of draining nodes
	nodes map[string]*drainingNode

	// doneNodeCh is used to signal that a node is done draining
	doneNodeCh chan string

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
	n.jobWatcher = n.jobFactory(n.ctx, n.queryLimiter, n.state, n.logger, n)
	n.nodeWatcher = n.nodeFactory(n.ctx, n.queryLimiter, n.state, n.logger, n)
	n.deadlineNotifier = n.deadlineNotifierFactory(n.ctx)
	n.nodes = make(map[string]*drainingNode, 32)
	n.doneNodeCh = make(chan string, 4)
}

func (n *NodeDrainer) run(ctx context.Context) {
	for {
		select {
		case <-n.ctx.Done():
			return
		case nodes := <-n.deadlineNotifier.NextBatch():
			n.handleDeadlinedNodes(nodes)
		case allocs := <-n.jobWatcher.Drain():
			n.handleJobAllocDrain(allocs)
		case node := <-n.doneNodeCh:
			n.handleDoneNode(node)
		}
	}
}

func (n *NodeDrainer) handleDeadlinedNodes(nodes []string) {
	// TODO
}

func (n *NodeDrainer) handleJobAllocDrain(allocs []*structs.Allocation) {
	// TODO

	// TODO Call check on the appropriate nodes when the final allocs
	// transistion to stop so we have a place to determine with the node
	// is done and the final drain of system allocs
	// TODO This probably requires changing the interface such that it
	// returns replaced allocs as well.
}

func (n *NodeDrainer) handleDoneNode(nodeID string) {
	// TODO
}

func (n *NodeDrainer) drain(allocs []*structs.Allocation) {
	// TODO
}
