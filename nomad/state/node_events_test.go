package state

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/event"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func allocID() string { return "e5bcbac313d6c-e29b-11c4-a5cd-5949157" }

func mockNode(id string) *structs.Node {
	node := mock.Node()
	node.ID = id
	return node
}

func TestNodeEventsFromChanges(t *testing.T) {
	cases := []struct {
		Name       string
		Setup      func(s *StateStore, index uint64) error
		Mutate     func(s *StateStore, tx *txn) error
		WantEvents []event.Event
		WantErr    bool
	}{
		{
			Name: "new node registered",
			Setup: func(s *StateStore, idx uint64) error {
				req := mockNode("8218b700-7e26-aac0-06d8-ff3b15f44e94")
				return s.UpsertNode(idx, req)
			},
			Mutate: func(s *StateStore, tx *txn) error {
				event := &structs.NodeEvent{
					Message:   "Node ready foo",
					Subsystem: structs.NodeEventSubsystemCluster,
					Timestamp: time.Now(),
				}
				return s.updateNodeStatusTxn(tx, "8218b700-7e26-aac0-06d8-ff3b15f44e94", structs.NodeStatusReady, time.Now().UnixNano(), event)
			},
			WantEvents: []event.Event{
				{
					Topic: "NodeEvent",
					Key:   "8218b700-7e26-aac0-06d8-ff3b15f44e94",
					Payload: NodeEvent{
						Message: "Node ready foo",
						NodeID:  "8218b700-7e26-aac0-06d8-ff3b15f44e94",
					},
				},
			},
		},
		{
			Name: "only new events",
			Setup: func(s *StateStore, idx uint64) error {
				req := mockNode("8218b700-7e26-aac0-06d8-ff3b15f44e94")
				require.NoError(t, s.UpsertNode(idx, req))
				event := &structs.NodeEvent{
					Message:   "Node foo initializing",
					Subsystem: structs.NodeEventSubsystemCluster,
					Timestamp: time.Now(),
				}
				return s.UpdateNodeStatus(idx, "8218b700-7e26-aac0-06d8-ff3b15f44e94", structs.NodeStatusInit, time.Now().UnixNano(), event)
			},
			Mutate: func(s *StateStore, tx *txn) error {
				event := &structs.NodeEvent{
					Message:   "Node foo ready",
					Subsystem: structs.NodeEventSubsystemCluster,
					Timestamp: time.Now(),
				}
				return s.updateNodeStatusTxn(tx, "8218b700-7e26-aac0-06d8-ff3b15f44e94", structs.NodeStatusReady, time.Now().UnixNano(), event)
			},
			WantEvents: []event.Event{
				{
					Topic: "NodeEvent",
					Key:   "8218b700-7e26-aac0-06d8-ff3b15f44e94",
					Payload: NodeEvent{
						Message: "Node foo ready",
						NodeID:  "8218b700-7e26-aac0-06d8-ff3b15f44e94",
					},
				},
			},
		},
		{
			Name: "node drain event",
			Setup: func(s *StateStore, idx uint64) error {

				req := mockNode("8218b700-7e26-aac0-06d8-ff3b15f44e94")
				require.NoError(t, s.UpsertNode(idx, req))
				event := &structs.NodeEvent{
					Message:   "Node foo initializing",
					Subsystem: structs.NodeEventSubsystemCluster,
					Timestamp: time.Now(),
				}
				require.NoError(t, s.UpdateNodeStatus(idx, "8218b700-7e26-aac0-06d8-ff3b15f44e94", structs.NodeStatusInit, time.Now().UnixNano(), event))
				alloc := mock.Alloc()
				alloc.NodeID = req.ID
				alloc.ID = allocID()
				return s.UpsertAllocs(idx, []*structs.Allocation{alloc})

			},
			Mutate: func(s *StateStore, tx *txn) error {
				event := &structs.NodeEvent{
					Subsystem: structs.NodeEventSubsystemCluster,
					Timestamp: time.Now(),
				}
				event.SetMessage("Node drain strategy set")
				event.SetSubsystem(structs.NodeEventSubsystemDrain)
				drain := &structs.DrainStrategy{}
				return s.updateNodeDrainImpl(tx, tx.Index, "8218b700-7e26-aac0-06d8-ff3b15f44e94", drain, false, time.Now().UnixNano(), event)
			},
			WantEvents: []event.Event{
				{
					Topic: "NodeEvent",
					Key:   "8218b700-7e26-aac0-06d8-ff3b15f44e94",
					Payload: NodeDrainEvent{
						Message: "Node drain strategy set",
						NodeID:  "8218b700-7e26-aac0-06d8-ff3b15f44e94",
						Allocs:  []string{allocID()},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			s := testStateStore(t)

			if tc.Setup != nil {
				require.NoError(t, tc.Setup(s, 10))
			}

			tx := s.db.WriteTxn(100)
			require.NoError(t, tc.Mutate(s, tx))

			got, err := s.NodeEventsFromChanges(tx, Changes{Changes: tx.Changes(), Index: 100})
			if tc.WantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.WantEvents, got)
		})
	}
}
