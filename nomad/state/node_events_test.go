package state

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestNodeEventsFromChanges(t *testing.T) {
	cases := []struct {
		Name       string
		MsgType    structs.MessageType
		Setup      func(s *StateStore, tx *txn) error
		Mutate     func(s *StateStore, tx *txn) error
		WantEvents []structs.Event
		WantErr    bool
		WantTopic  structs.Topic
	}{
		{
			MsgType:   structs.NodeRegisterRequestType,
			WantTopic: TopicNode,
			Name:      "node registered",
			Mutate: func(s *StateStore, tx *txn) error {
				return upsertNodeTxn(tx, tx.Index, testNode())
			},
			WantEvents: []structs.Event{{
				Topic: TopicNode,
				Type:  TypeNodeRegistration,
				Key:   testNodeID(),
				Index: 100,
				Payload: &NodeEvent{
					Node: testNode(),
				},
			}},
			WantErr: false,
		},
		{
			MsgType:   structs.NodeRegisterRequestType,
			WantTopic: TopicNode,
			Name:      "node registered initializing",
			Mutate: func(s *StateStore, tx *txn) error {
				return upsertNodeTxn(tx, tx.Index, testNode(nodeNotReady))
			},
			WantEvents: []structs.Event{{
				Topic: TopicNode,
				Type:  TypeNodeRegistration,
				Key:   testNodeID(),
				Index: 100,
				Payload: &NodeEvent{
					Node: testNode(nodeNotReady),
				},
			}},
			WantErr: false,
		},
		{
			MsgType:   structs.NodeDeregisterRequestType,
			WantTopic: TopicNode,
			Name:      "node deregistered",
			Setup: func(s *StateStore, tx *txn) error {
				return upsertNodeTxn(tx, tx.Index, testNode())
			},
			Mutate: func(s *StateStore, tx *txn) error {
				return deleteNodeTxn(tx, tx.Index, []string{testNodeID()})
			},
			WantEvents: []structs.Event{{
				Topic: TopicNode,
				Type:  TypeNodeDeregistration,
				Key:   testNodeID(),
				Index: 100,
				Payload: &NodeEvent{
					Node: testNode(),
				},
			}},
			WantErr: false,
		},
		{
			MsgType:   structs.NodeDeregisterRequestType,
			WantTopic: TopicNode,
			Name:      "batch node deregistered",
			Setup: func(s *StateStore, tx *txn) error {
				require.NoError(t, upsertNodeTxn(tx, tx.Index, testNode()))
				return upsertNodeTxn(tx, tx.Index, testNode(nodeIDTwo))
			},
			Mutate: func(s *StateStore, tx *txn) error {
				return deleteNodeTxn(tx, tx.Index, []string{testNodeID(), testNodeIDTwo()})
			},
			WantEvents: []structs.Event{
				{
					Topic: TopicNode,
					Type:  TypeNodeDeregistration,
					Key:   testNodeID(),
					Index: 100,
					Payload: &NodeEvent{
						Node: testNode(),
					},
				},
				{
					Topic: TopicNode,
					Type:  TypeNodeDeregistration,
					Key:   testNodeIDTwo(),
					Index: 100,
					Payload: &NodeEvent{
						Node: testNode(nodeIDTwo),
					},
				},
			},
			WantErr: false,
		},
		{
			MsgType:   structs.UpsertNodeEventsType,
			WantTopic: TopicNode,
			Name:      "batch node events upserted",
			Setup: func(s *StateStore, tx *txn) error {
				require.NoError(t, upsertNodeTxn(tx, tx.Index, testNode()))
				return upsertNodeTxn(tx, tx.Index, testNode(nodeIDTwo))
			},
			Mutate: func(s *StateStore, tx *txn) error {
				eventFn := func(id string) []*structs.NodeEvent {
					return []*structs.NodeEvent{
						{
							Message:   "test event one",
							Subsystem: "Cluster",
							Details: map[string]string{
								"NodeID": id,
							},
						},
						{
							Message:   "test event two",
							Subsystem: "Cluster",
							Details: map[string]string{
								"NodeID": id,
							},
						},
					}
				}
				require.NoError(t, s.upsertNodeEvents(tx.Index, testNodeID(), eventFn(testNodeID()), tx))
				return s.upsertNodeEvents(tx.Index, testNodeIDTwo(), eventFn(testNodeIDTwo()), tx)
			},
			WantEvents: []structs.Event{
				{
					Topic: TopicNode,
					Type:  TypeNodeEvent,
					Key:   testNodeID(),
					Index: 100,
					Payload: &NodeEvent{
						Node: testNode(),
					},
				},
				{
					Topic: TopicNode,
					Type:  TypeNodeEvent,
					Key:   testNodeIDTwo(),
					Index: 100,
					Payload: &NodeEvent{
						Node: testNode(nodeIDTwo),
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

			require.Equal(t, len(tc.WantEvents), len(got.Events))
			for idx, g := range got.Events {
				// assert equality of shared fields

				want := tc.WantEvents[idx]
				require.Equal(t, want.Index, g.Index)
				require.Equal(t, want.Key, g.Key)
				require.Equal(t, want.Topic, g.Topic)

				switch tc.MsgType {
				case structs.NodeRegisterRequestType:
					requireNodeRegistrationEventEqual(t, tc.WantEvents[idx], g)
				case structs.NodeDeregisterRequestType:
					requireNodeDeregistrationEventEqual(t, tc.WantEvents[idx], g)
				case structs.UpsertNodeEventsType:
					requireNodeEventEqual(t, tc.WantEvents[idx], g)
				default:
					require.Fail(t, "unhandled message type")
				}
			}
		})
	}
}

func TestNodeDrainEventFromChanges(t *testing.T) {
	t.Parallel()
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventPublisher()

	// setup
	setupTx := s.db.WriteTxn(10)

	node := mock.Node()
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()
	alloc1.NodeID = node.ID
	alloc2.NodeID = node.ID

	require.NoError(t, upsertNodeTxn(setupTx, 10, node))
	require.NoError(t, s.upsertAllocsImpl(100, []*structs.Allocation{alloc1, alloc2}, setupTx))
	setupTx.Txn.Commit()

	// changes
	tx := s.db.WriteTxn(100)

	strat := &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{
			Deadline:         10 * time.Minute,
			IgnoreSystemJobs: false,
		},
		StartedAt: time.Now(),
	}
	markEligible := false
	updatedAt := time.Now()
	event := &structs.NodeEvent{}

	require.NoError(t, s.updateNodeDrainImpl(tx, 100, node.ID, strat, markEligible, updatedAt.UnixNano(), event))
	changes := Changes{Changes: tx.Changes(), Index: 100, MsgType: structs.NodeUpdateDrainRequestType}
	got, err := processDBChanges(tx, changes)
	require.NoError(t, err)

	require.Len(t, got, 1)

	require.Equal(t, TopicNode, got.Events[0].Topic)
	require.Equal(t, TypeNodeDrain, got.Events[0].Type)
	require.Equal(t, uint64(100), got.Events[0].Index)

	nodeEvent, ok := got.Events[0].Payload.(*NodeDrainEvent)
	require.True(t, ok)

	require.Equal(t, structs.NodeSchedulingIneligible, nodeEvent.Node.SchedulingEligibility)
	require.Equal(t, strat, nodeEvent.Node.DrainStrategy)
}

func requireNodeRegistrationEventEqual(t *testing.T, want, got structs.Event) {
	t.Helper()

	wantPayload := want.Payload.(*NodeEvent)
	gotPayload := got.Payload.(*NodeEvent)

	// Check payload equality for the fields that we can easily control
	require.Equal(t, wantPayload.Node.Status, gotPayload.Node.Status)
	require.Equal(t, wantPayload.Node.ID, gotPayload.Node.ID)
	require.NotEqual(t, wantPayload.Node.Events, gotPayload.Node.Events)
}

func requireNodeDeregistrationEventEqual(t *testing.T, want, got structs.Event) {
	t.Helper()

	wantPayload := want.Payload.(*NodeEvent)
	gotPayload := got.Payload.(*NodeEvent)

	require.Equal(t, wantPayload.Node.ID, gotPayload.Node.ID)
	require.NotEqual(t, wantPayload.Node.Events, gotPayload.Node.Events)
}

func requireNodeEventEqual(t *testing.T, want, got structs.Event) {
	gotPayload := got.Payload.(*NodeEvent)

	require.Len(t, gotPayload.Node.Events, 3)
}

type nodeOpts func(n *structs.Node)

func nodeNotReady(n *structs.Node) {
	n.Status = structs.NodeStatusInit
}

func nodeReady(n *structs.Node) {
	n.Status = structs.NodeStatusReady
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
