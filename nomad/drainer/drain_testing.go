package drainer

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

func (m *MockNodeTracker) TrackedNodes() map[string]*structs.Node {
	m.Lock()
	defer m.Unlock()
	return m.Nodes
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

func (m *MockNodeTracker) events() []*MockNodeTrackerEvent {
	m.Lock()
	defer m.Unlock()

	return m.Events
}
