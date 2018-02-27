package drainer

import (
	"context"
	"log"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

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

	resp := make([]*structs.Node, 0, 8)

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
