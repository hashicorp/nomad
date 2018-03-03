package drainerv2

import (
	"sync"

	"github.com/hashicorp/nomad/nomad/structs"
)

type MockNodeTrackerEvent struct {
	NodeUpdate *structs.Node
	NodeRemove string
}

type MockNodeTracker struct {
	Nodes  map[string]*structs.Node
	Events []*MockNodeTrackerEvent
	sync.Mutex
}

func NewMockNodeTracker() *MockNodeTracker {
	return &MockNodeTracker{
		Nodes:  make(map[string]*structs.Node),
		Events: make([]*MockNodeTrackerEvent, 0, 16),
	}
}

func (m *MockNodeTracker) Tracking(nodeID string) (*structs.Node, bool) {
	m.Lock()
	defer m.Unlock()
	n, ok := m.Nodes[nodeID]
	return n, ok
}

func (m *MockNodeTracker) Remove(nodeID string) {
	m.Lock()
	defer m.Unlock()
	delete(m.Nodes, nodeID)
	m.Events = append(m.Events, &MockNodeTrackerEvent{NodeRemove: nodeID})
}

func (m *MockNodeTracker) Update(node *structs.Node) {
	m.Lock()
	defer m.Unlock()
	m.Nodes[node.ID] = node
	m.Events = append(m.Events, &MockNodeTrackerEvent{NodeUpdate: node})
}
