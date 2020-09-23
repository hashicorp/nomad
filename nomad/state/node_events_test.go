package state

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/stream"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestNodeRegisterEventFromChanges(t *testing.T) {
	cases := []struct {
		Name       string
		MsgType    structs.MessageType
		Setup      func(s *StateStore, tx *txn) error
		Mutate     func(s *StateStore, tx *txn) error
		WantEvents []stream.Event
		WantErr    bool
		WantTopic  string
	}{
		{
			MsgType:   structs.NodeRegisterRequestType,
			WantTopic: TopicNodeRegistration,
			Name:      "node registered",
			Mutate: func(s *StateStore, tx *txn) error {
				return upsertNodeTxn(tx, tx.Index, testNode())
			},
			WantEvents: []stream.Event{{
				Topic: TopicNodeRegistration,
				Key:   testNodeID(),
				Index: 100,
				Payload: &NodeRegistrationEvent{
					Event: &structs.NodeEvent{
						Message:   "Node registered",
						Subsystem: "Cluster",
					},
					NodeStatus: structs.NodeStatusReady,
				},
			}},
			WantErr: false,
		},
		{
			MsgType:   structs.NodeRegisterRequestType,
			WantTopic: TopicNodeRegistration,
			Name:      "node registered initializing",
			Mutate: func(s *StateStore, tx *txn) error {
				return upsertNodeTxn(tx, tx.Index, testNode(nodeNotReady))
			},
			WantEvents: []stream.Event{{
				Topic: TopicNodeRegistration,
				Key:   testNodeID(),
				Index: 100,
				Payload: &NodeRegistrationEvent{
					Event: &structs.NodeEvent{
						Message:   "Node registered",
						Subsystem: "Cluster",
					},
					NodeStatus: structs.NodeStatusInit,
				},
			}},
			WantErr: false,
		},
		{
			MsgType:   structs.NodeDeregisterRequestType,
			WantTopic: TopicNodeDeregistration,
			Name:      "node deregistered",
			Setup: func(s *StateStore, tx *txn) error {
				return upsertNodeTxn(tx, tx.Index, testNode())
			},
			Mutate: func(s *StateStore, tx *txn) error {
				return deleteNodeTxn(tx, tx.Index, []string{testNodeID()})
			},
			WantEvents: []stream.Event{{
				Topic: TopicNodeDeregistration,
				Key:   testNodeID(),
				Index: 100,
				Payload: &NodeDeregistrationEvent{
					NodeID: testNodeID(),
				},
			}},
			WantErr: false,
		},
		{
			MsgType:   structs.NodeDeregisterRequestType,
			WantTopic: TopicNodeDeregistration,
			Name:      "batch node deregistered",
			Setup: func(s *StateStore, tx *txn) error {
				require.NoError(t, upsertNodeTxn(tx, tx.Index, testNode()))
				return upsertNodeTxn(tx, tx.Index, testNode(nodeIDTwo))
			},
			Mutate: func(s *StateStore, tx *txn) error {
				return deleteNodeTxn(tx, tx.Index, []string{testNodeID(), testNodeIDTwo()})
			},
			WantEvents: []stream.Event{
				{
					Topic: TopicNodeDeregistration,
					Key:   testNodeID(),
					Index: 100,
					Payload: &NodeDeregistrationEvent{
						NodeID: testNodeID(),
					},
				},
				{
					Topic: TopicNodeDeregistration,
					Key:   testNodeIDTwo(),
					Index: 100,
					Payload: &NodeDeregistrationEvent{
						NodeID: testNodeIDTwo(),
					},
				},
			},
			WantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			s := TestStateStoreCfg(t, TestStateStorePublisher(t))
			defer s.StopEventPublisher()

			if tc.Setup != nil {
				// Bypass publish mechanism for setup
				setupTx := s.db.WriteTxn(10)
				require.NoError(t, tc.Setup(s, setupTx))
				setupTx.Txn.Commit()
			}

			tx := s.db.WriteTxn(100)
			require.NoError(t, tc.Mutate(s, tx))

			changes := Changes{Changes: tx.Changes(), Index: 100, MsgType: tc.MsgType}
			got, err := processDBChanges(tx, changes)

			if tc.WantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			require.Equal(t, len(tc.WantEvents), len(got))
			for idx, g := range got {
				switch tc.MsgType {
				case structs.NodeRegisterRequestType:
					requireNodeRegistrationEventEqual(t, tc.WantEvents[idx], g)
				case structs.NodeDeregisterRequestType:
					requireNodeDeregistrationEventEqual(t, tc.WantEvents[idx], g)
				}
			}
		})
	}
}

func requireNodeRegistrationEventEqual(t *testing.T, want, got stream.Event) {
	t.Helper()

	require.Equal(t, want.Index, got.Index)
	require.Equal(t, want.Key, got.Key)
	require.Equal(t, want.Topic, got.Topic)

	wantPayload := want.Payload.(*NodeRegistrationEvent)
	gotPayload := got.Payload.(*NodeRegistrationEvent)

	// Check payload equality for the fields that we can easily control
	require.Equal(t, wantPayload.NodeStatus, gotPayload.NodeStatus)
	require.Equal(t, wantPayload.Event.Message, gotPayload.Event.Message)
	require.Equal(t, wantPayload.Event.Subsystem, gotPayload.Event.Subsystem)
}

func requireNodeDeregistrationEventEqual(t *testing.T, want, got stream.Event) {
	t.Helper()

	require.Equal(t, want.Index, got.Index)
	require.Equal(t, want.Key, got.Key)
	require.Equal(t, want.Topic, got.Topic)

	wantPayload := want.Payload.(*NodeDeregistrationEvent)
	gotPayload := got.Payload.(*NodeDeregistrationEvent)

	require.Equal(t, wantPayload, gotPayload)
}

type nodeOpts func(n *structs.Node)

func nodeNotReady(n *structs.Node) {
	n.Status = structs.NodeStatusInit
}

func nodeIDTwo(n *structs.Node) {
	n.ID = testNodeIDTwo()
}

func testNode(opts ...nodeOpts) *structs.Node {
	n := mock.Node()
	n.ID = testNodeID()

	n.SecretID = "ab9812d3-6a21-40d3-973d-d9d2174a23ee"

	for _, opt := range opts {
		opt(n)
	}
	return n
}

func testNodeID() string {
	return "9d5741c1-3899-498a-98dd-eb3c05665863"
}

func testNodeIDTwo() string {
	return "694ff31d-8c59-4030-ac83-e15692560c8d"
}
