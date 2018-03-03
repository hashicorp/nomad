package drainerv2

import (
	"context"
	"log"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"golang.org/x/time/rate"
)

// DrainingNodeWatcher is the interface for watching for draining nodes.
type DrainingNodeWatcher interface{}

// Tracking returns the whether the node is being tracked and if so the copy of
// the node object that is tracked.
func (n *NodeDrainer) Tracking(nodeID string) (*structs.Node, bool) {
	n.l.RLock()
	defer n.l.RUnlock()

	draining, ok := n.nodes[nodeID]
	if !ok {
		return nil, false
	}

	return draining.GetNode(), true
}

// Remove removes the given node from being tracked
func (n *NodeDrainer) Remove(nodeID string) {
	n.l.Lock()
	defer n.l.Unlock()

	// TODO test the notifier is updated
	// Remove it from being tracked and remove it from the dealiner
	delete(n.nodes, nodeID)
	n.deadlineNotifier.Remove(nodeID)
}

// Update updates the node, either updating the tracked version or starting to
// track the node.
func (n *NodeDrainer) Update(node *structs.Node) {
	n.l.Lock()
	defer n.l.Unlock()

	if node == nil {
		return
	}

	draining, ok := n.nodes[node.ID]
	if !ok {
		n.nodes[node.ID] = NewDrainingNode(node, n.state, n)
		return
	}

	// Update it and update the dealiner
	draining.Update(node)

	// TODO test the notifier is updated
	if inf, deadline := node.DrainStrategy.DeadlineTime(); !inf {
		n.deadlineNotifier.Watch(node.ID, deadline)
	} else {
		// TODO think about handling any race that may occur. I believe it is
		// totally fine as long as the handlers are locked.

		// There is an infinite deadline so it shouldn't be tracked for
		// deadlining
		n.deadlineNotifier.Remove(node.ID)
	}

}

// nodeDrainWatcher is used to watch nodes that are entering, leaving or
// changing their drain strategy.
type nodeDrainWatcher struct {
	ctx    context.Context
	logger *log.Logger

	// state is the state that is watched for state changes.
	state *state.StateStore

	// limiter is used to limit the rate of blocking queries
	limiter *rate.Limiter

	// tracker is the object that is tracking the nodes and provides us with the
	// needed callbacks
	tracker NodeTracker
}

// NewNodeDrainWatcher returns a new node drain watcher.
func NewNodeDrainWatcher(ctx context.Context, limiter *rate.Limiter, state *state.StateStore, logger *log.Logger, tracker NodeTracker) *nodeDrainWatcher {
	w := &nodeDrainWatcher{
		ctx:     ctx,
		limiter: limiter,
		logger:  logger,
		tracker: tracker,
		state:   state,
	}

	go w.watch()
	return w
}

// watch is the long lived watching routine that detects node changes.
func (w *nodeDrainWatcher) watch() {
	nindex := uint64(1)
	for {
		w.logger.Printf("[TRACE] nomad.drain.node_watcher: getting nodes at index %d", nindex)
		nodes, index, err := w.getNodes(nindex)
		if err != nil {
			if err == context.Canceled {
				w.logger.Printf("[TRACE] nomad.drain.node_watcher: shutting down")
				return
			}

			w.logger.Printf("[ERR] nomad.drain.node_watcher: error watching node updates at index %d: %v", nindex, err)
			select {
			case <-w.ctx.Done():
				w.logger.Printf("[TRACE] nomad.drain.node_watcher: shutting down")
				return
			case <-time.After(stateReadErrorDelay):
				continue
			}
		}

		// update index for next run
		nindex = index

		for _, node := range nodes {
			newDraining := node.DrainStrategy != nil
			currentNode, tracked := w.tracker.Tracking(node.ID)

			switch {
			// If the node is tracked but not draining, untrack
			case tracked && !newDraining:
				w.logger.Printf("[TRACE] nomad.drain.node_watcher: tracked node %q is no longer draining", node.ID)
				w.tracker.Remove(node.ID)

				// If the node is not being tracked but is draining, track
			case !tracked && newDraining:
				w.logger.Printf("[TRACE] nomad.drain.node_watcher: untracked node %q is draining", node.ID)
				w.tracker.Update(node)

				// If the node is being tracked but has changed, update:
			case tracked && newDraining && !currentNode.DrainStrategy.Equal(node.DrainStrategy):
				w.logger.Printf("[TRACE] nomad.drain.node_watcher: tracked node %q has updated drain", node.ID)
				w.tracker.Update(node)
			default:
				w.logger.Printf("[TRACE] nomad.drain.node_watcher: node %q at index %v: tracked %v, draining %v", node.ID, node.ModifyIndex, tracked, newDraining)
			}
		}
	}
}

// getNodes returns all nodes blocking until the nodes are after the given index.
func (w *nodeDrainWatcher) getNodes(minIndex uint64) ([]*structs.Node, uint64, error) {
	if err := w.limiter.Wait(w.ctx); err != nil {
		return nil, 0, err
	}

	resp, index, err := w.state.BlockingQuery(w.getNodesImpl, minIndex, w.ctx)
	if err != nil {
		return nil, 0, err
	}

	return resp.([]*structs.Node), index, nil
}

// getNodesImpl is used to get nodes from the state store, returning the set of
// nodes and the given index.
func (w *nodeDrainWatcher) getNodesImpl(ws memdb.WatchSet, state *state.StateStore) (interface{}, uint64, error) {
	iter, err := state.Nodes(ws)
	if err != nil {
		return nil, 0, err
	}

	index, err := state.Index("nodes")
	if err != nil {
		return nil, 0, err
	}

	resp := make([]*structs.Node, 0, 64)
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
