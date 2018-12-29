package drainer

import (
	"context"
	"time"

	log "github.com/hashicorp/go-hclog"
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
		n.logger.Error("error retrieving draining jobs on node", "node_id", node.ID, "error", err)
		return
	}
	n.logger.Trace("node has draining jobs on it", "node_id", node.ID, "num_jobs", len(jobs))
	n.jobWatcher.RegisterJobs(jobs)

	// TODO Test at this layer as well that a node drain on a node without
	// allocs immediately gets unmarked as draining
	// Check if the node is done such that if an operator drains a node with
	// nothing on it we unset drain
	done, err := draining.IsDone()
	if err != nil {
		n.logger.Error("failed to check if node is done draining", "node_id", node.ID, "error", err)
		return
	}

	if done {
		// Node is done draining. Stop remaining system allocs before
		// marking node as complete.
		remaining, err := draining.RemainingAllocs()
		if err != nil {
			n.logger.Error("error getting remaining allocs on drained node", "node_id", node.ID, "error", err)
		} else if len(remaining) > 0 {
			future := structs.NewBatchFuture()
			n.drainAllocs(future, remaining)
			if err := future.Wait(); err != nil {
				n.logger.Error("failed to drain remaining allocs from done node", "num_allocs", len(remaining), "node_id", node.ID, "error", err)
			}
		}

		// Create the node event
		event := structs.NewNodeEvent().
			SetSubsystem(structs.NodeEventSubsystemDrain).
			SetMessage(NodeDrainEventComplete)

		index, err := n.raft.NodesDrainComplete([]string{node.ID}, event)
		if err != nil {
			n.logger.Error("failed to unset drain for node", "node_id", node.ID, "error", err)
		} else {
			n.logger.Info("node completed draining at index", "node_id", node.ID, "index", index)
		}
	}
}

// nodeDrainWatcher is used to watch nodes that are entering, leaving or
// changing their drain strategy.
type nodeDrainWatcher struct {
	ctx    context.Context
	logger log.Logger

	// state is the state that is watched for state changes.
	state *state.StateStore

	// limiter is used to limit the rate of blocking queries
	limiter *rate.Limiter

	// tracker is the object that is tracking the nodes and provides us with the
	// needed callbacks
	tracker NodeTracker
}

// NewNodeDrainWatcher returns a new node drain watcher.
func NewNodeDrainWatcher(ctx context.Context, limiter *rate.Limiter, state *state.StateStore, logger log.Logger, tracker NodeTracker) *nodeDrainWatcher {
	w := &nodeDrainWatcher{
		ctx:     ctx,
		limiter: limiter,
		logger:  logger.Named("node_watcher"),
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
		w.logger.Trace("getting nodes at index", "index", nindex)
		nodes, index, err := w.getNodes(nindex)
		w.logger.Trace("got nodes at index", "num_nodes", len(nodes), "index", nindex, "error", err)
		if err != nil {
			if err == context.Canceled {
				w.logger.Trace("shutting down")
				return
			}

			w.logger.Error("error watching node updates at index", "index", nindex, "error", err)
			select {
			case <-w.ctx.Done():
				w.logger.Trace("shutting down")
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
				w.logger.Trace("tracked node is no longer draining", "node_id", nodeID)
				w.tracker.Remove(nodeID)

				// If the node is not being tracked but is draining, track
			case !tracked && newDraining:
				w.logger.Trace("untracked node is draining", "node_id", nodeID)
				w.tracker.Update(node)

				// If the node is being tracked but has changed, update:
			case tracked && newDraining && !currentNode.DrainStrategy.Equal(node.DrainStrategy):
				w.logger.Trace("tracked node has updated drain", "node_id", nodeID)
				w.tracker.Update(node)
			default:
				w.logger.Trace("no changes for node", "node_id", nodeID, "node_modify_index", node.ModifyIndex, "tracked", tracked, "newly_draining", newDraining)
			}

			// TODO(schmichael) handle the case of a lost node
		}

		for nodeID := range tracked {
			if _, ok := nodes[nodeID]; !ok {
				w.logger.Trace("tracked node no longer exists", "node_id", nodeID)
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

	var maxIndex uint64 = 0
	resp := make(map[string]*structs.Node, 64)
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		node := raw.(*structs.Node)
		resp[node.ID] = node
		if maxIndex < node.ModifyIndex {
			maxIndex = node.ModifyIndex
		}
	}

	// Prefer using the actual max index of affected nodes since it means less
	// unblocking
	if maxIndex != 0 {
		index = maxIndex
	}

	return resp, index, nil
}
