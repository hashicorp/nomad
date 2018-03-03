package drainerv2

import (
	"sync"
	"time"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// TODO make this an interface and then I can optimize the infinite case by
// using a singleton object

type drainCoordinator interface {
	nodeDone(nodeID string)
}

func (n *NodeDrainer) nodeDone(nodeID string) {
	select {
	case <-n.ctx.Done():
	case n.doneNodeCh <- nodeID:
	}
}

type drainingNode struct {
	coordinator drainCoordinator
	state       *state.StateStore
	node        *structs.Node
	l           sync.RWMutex
}

func NewDrainingNode(node *structs.Node, state *state.StateStore, coordinator drainCoordinator) *drainingNode {
	return &drainingNode{
		coordinator: coordinator,
		state:       state,
		node:        node,
	}
}

func (n *drainingNode) GetNode() *structs.Node {
	n.l.Lock()
	defer n.l.Unlock()
	return n.node
}

func (n *drainingNode) Update(node *structs.Node) {
	n.l.Lock()
	defer n.l.Unlock()
	n.node = node
}

// DeadlineTime returns if the node has a deadline and if so what it is
func (n *drainingNode) DeadlineTime() (bool, time.Time) {
	n.l.RLock()
	defer n.l.RUnlock()

	// Should never happen
	if n.node == nil || n.node.DrainStrategy == nil {
		return false, time.Time{}
	}

	return n.node.DrainStrategy.DeadlineTime()
}

// DeadlineAllocs returns the set of allocations that should be drained given a
// node is at its deadline
func (n *drainingNode) DeadlineAllocs() ([]*structs.Allocation, error) {
	n.l.RLock()
	defer n.l.RUnlock()
	return nil, nil
}
