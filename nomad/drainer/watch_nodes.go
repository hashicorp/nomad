package drainer

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

// TrackedNodes returns the set of tracked nodes
func (n *NodeDrainer) TrackedNodes() map[string]*structs.Node {
	n.l.RLock()
	defer n.l.RUnlock()

	t := make(map[string]*structs.Node, len(n.nodes))
	for n, d := range n.nodes {
		t[n] = d.GetNode()
	}

	return t
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
		draining = NewDrainingNode(node, n.state)
		n.nodes[node.ID] = draining
	} else {
		// Update it
		draining.Update(node)
	}

	// TODO test the notifier is updated
	if inf, deadline := node.DrainStrategy.DeadlineTime(); !inf {
		n.deadlineNotifier.Watch(node.ID, deadline)
	} else {
		// There is an infinite deadline so it shouldn't be tracked for
		// deadlining
		n.deadlineNotifier.Remove(node.ID)
	}

	// TODO Test this
	// Register interest in the draining jobs.
	jobs, err := draining.DrainingJobs()
	if err != nil {
		n.logger.Printf("[ERR] nomad.drain: error retrieving draining jobs on node %q: %v", node.ID, err)
		return
	}
	n.logger.Printf("[TRACE] nomad.drain: node %q has %d draining jobs on it", node.ID, len(jobs))
	n.jobWatcher.RegisterJobs(jobs)

	// TODO Test at this layer as well that a node drain on a node without
	// allocs immediately gets unmarked as draining
	// Check if the node is done such that if an operator drains a node with
	// nothing on it we unset drain
	done, err := draining.IsDone()
	if err != nil {
		n.logger.Printf("[ERR] nomad.drain: failed to check if node %q is done draining: %v", node.ID, err)
		return
	}

	if done {
		// Node is done draining. Stop remaining system allocs before
		// marking node as complete.
		remaining, err := draining.RemainingAllocs()
		if err != nil {
			n.logger.Printf("[ERR] nomad.drain: error getting remaining allocs on drained node %q: %v",
				node.ID, err)
		} else if len(remaining) > 0 {
			future := structs.NewBatchFuture()
			n.drainAllocs(future, remaining)
			if err := future.Wait(); err != nil {
				n.logger.Printf("[ERR] nomad.drain: failed to drain %d remaining allocs from done node %q: %v",
					len(remaining), node.ID, err)
			}
		}

		index, err := n.raft.NodesDrainComplete([]string{node.ID})
		if err != nil {
			n.logger.Printf("[ERR] nomad.drain: failed to unset drain for node %q: %v", node.ID, err)
		} else {
			n.logger.Printf("[INFO] nomad.drain: node %q completed draining at index %d", node.ID, index)
		}
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
		w.logger.Printf("[TRACE] nomad.drain.node_watcher: got nodes %d at index %d: %v", len(nodes), nindex, err)
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

		tracked := w.tracker.TrackedNodes()
		for nodeID, node := range nodes {
			newDraining := node.DrainStrategy != nil
			currentNode, tracked := tracked[nodeID]

			switch {
			// If the node is tracked but not draining, untrack
			case tracked && !newDraining:
				w.logger.Printf("[TRACE] nomad.drain.node_watcher: tracked node %q is no longer draining", nodeID)
				w.tracker.Remove(nodeID)

				// If the node is not being tracked but is draining, track
			case !tracked && newDraining:
				w.logger.Printf("[TRACE] nomad.drain.node_watcher: untracked node %q is draining", nodeID)
				w.tracker.Update(node)

				// If the node is being tracked but has changed, update:
			case tracked && newDraining && !currentNode.DrainStrategy.Equal(node.DrainStrategy):
				w.logger.Printf("[TRACE] nomad.drain.node_watcher: tracked node %q has updated drain", nodeID)
				w.tracker.Update(node)
			default:
				w.logger.Printf("[TRACE] nomad.drain.node_watcher: node %q at index %v: tracked %v, draining %v", nodeID, node.ModifyIndex, tracked, newDraining)
			}

			// TODO(schmichael) handle the case of a lost node
		}

		for nodeID := range tracked {
			if _, ok := nodes[nodeID]; !ok {
				w.logger.Printf("[TRACE] nomad.drain.node_watcher: tracked node %q is no longer exists", nodeID)
				w.tracker.Remove(nodeID)
			}
		}
	}
}

// getNodes returns all nodes blocking until the nodes are after the given index.
func (w *nodeDrainWatcher) getNodes(minIndex uint64) (map[string]*structs.Node, uint64, error) {
	if err := w.limiter.Wait(w.ctx); err != nil {
		return nil, 0, err
	}

	resp, index, err := w.state.BlockingQuery(w.getNodesImpl, minIndex, w.ctx)
	if err != nil {
		return nil, 0, err
	}

	return resp.(map[string]*structs.Node), index, nil
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

	resp := make(map[string]*structs.Node, 64)
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		node := raw.(*structs.Node)
		resp[node.ID] = node
	}

	return resp, index, nil
}
