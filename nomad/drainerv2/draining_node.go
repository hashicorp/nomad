package drainerv2

import (
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

type drainingNode struct {
	state *state.StateStore
	node  *structs.Node
	l     sync.RWMutex
}

func NewDrainingNode(node *structs.Node, state *state.StateStore) *drainingNode {
	return &drainingNode{
		state: state,
		node:  node,
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

// IsDone returns if the node is done draining
func (n *drainingNode) IsDone() (bool, error) {
	n.l.RLock()
	defer n.l.RUnlock()

	// Should never happen
	if n.node == nil || n.node.DrainStrategy == nil {
		return false, fmt.Errorf("node doesn't have a drain strategy set")
	}

	// Grab the relevant drain info
	ignoreSystem := n.node.DrainStrategy.IgnoreSystemJobs

	// Retrieve the allocs on the node
	allocs, err := n.state.AllocsByNode(nil, n.node.ID)
	if err != nil {
		return false, err
	}

	for _, alloc := range allocs {
		// Skip system if configured to
		if alloc.Job.Type == structs.JobTypeSystem && ignoreSystem {
			continue
		}

		// If there is a non-terminal we aren't done
		if !alloc.TerminalStatus() {
			return false, nil
		}
	}

	return true, nil
}

// TODO test that we return the right thing given the strategies
// DeadlineAllocs returns the set of allocations that should be drained given a
// node is at its deadline
func (n *drainingNode) DeadlineAllocs() ([]*structs.Allocation, error) {
	n.l.RLock()
	defer n.l.RUnlock()

	// Should never happen
	if n.node == nil || n.node.DrainStrategy == nil {
		return nil, fmt.Errorf("node doesn't have a drain strategy set")
	}

	// Grab the relevant drain info
	inf, _ := n.node.DrainStrategy.DeadlineTime()
	if inf {
		return nil, nil
	}
	ignoreSystem := n.node.DrainStrategy.IgnoreSystemJobs

	// Retrieve the allocs on the node
	allocs, err := n.state.AllocsByNode(nil, n.node.ID)
	if err != nil {
		return nil, err
	}

	var drain []*structs.Allocation
	for _, alloc := range allocs {
		// Nothing to do on a terminal allocation
		if alloc.TerminalStatus() {
			continue
		}

		// Skip system if configured to
		if alloc.Job.Type == structs.JobTypeSystem && ignoreSystem {
			continue
		}

		drain = append(drain, alloc)
	}

	return drain, nil
}

// RunningServices returns the set of jobs on the node
func (n *drainingNode) RunningServices() ([]structs.JobNs, error) {
	n.l.RLock()
	defer n.l.RUnlock()

	// Retrieve the allocs on the node
	allocs, err := n.state.AllocsByNode(nil, n.node.ID)
	if err != nil {
		return nil, err
	}

	jobIDs := make(map[structs.JobNs]struct{})
	var jobs []structs.JobNs
	for _, alloc := range allocs {
		if alloc.TerminalStatus() || alloc.Job.Type != structs.JobTypeService {
			continue
		}

		jns := structs.NewJobNs(alloc.Namespace, alloc.JobID)
		if _, ok := jobIDs[jns]; ok {
			continue
		}
		jobIDs[jns] = struct{}{}
		jobs = append(jobs, jns)
	}

	return jobs, nil
}
