package state

import (
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestEventsFromChanges_DeploymentUpdate(t *testing.T) {
	t.Parallel()
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	// setup
	setupTx := s.db.WriteTxn(10)

	j := mock.Job()
	e := mock.Eval()
	e.JobID = j.ID

	d := mock.Deployment()
	d.JobID = j.ID

	require.NoError(t, s.upsertJobImpl(10, j, false, setupTx))
	require.NoError(t, s.upsertDeploymentImpl(10, d, setupTx))

	setupTx.Txn.Commit()

	msgType := structs.DeploymentStatusUpdateRequestType

	req := &structs.DeploymentStatusUpdateRequest{
		DeploymentUpdate: &structs.DeploymentStatusUpdate{
			DeploymentID:      d.ID,
			Status:            structs.DeploymentStatusPaused,
			StatusDescription: structs.DeploymentStatusDescriptionPaused,
		},
		Eval: e,
		// Exlude Job and assert its added
	}

	require.NoError(t, s.UpdateDeploymentStatus(msgType, 100, req))

	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	require.Len(t, events, 2)

	got := events[0]
	require.Equal(t, uint64(100), got.Index)
	require.Equal(t, d.ID, got.Key)

	de := got.Payload.(*DeploymentEvent)
	require.Equal(t, structs.DeploymentStatusPaused, de.Deployment.Status)
	require.Contains(t, got.FilterKeys, j.ID)
}

func TestEventsFromChanges_DeploymentPromotion(t *testing.T) {
	t.Parallel()
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	// setup
	setupTx := s.db.WriteTxn(10)

	j := mock.Job()
	tg1 := j.TaskGroups[0]
	tg2 := tg1.Copy()
	tg2.Name = "foo"
	j.TaskGroups = append(j.TaskGroups, tg2)
	require.NoError(t, s.upsertJobImpl(10, j, false, setupTx))

	d := mock.Deployment()
	d.StatusDescription = structs.DeploymentStatusDescriptionRunningNeedsPromotion
	d.JobID = j.ID
	d.TaskGroups = map[string]*structs.DeploymentState{
		"web": {
			DesiredTotal:    10,
			DesiredCanaries: 1,
		},
		"foo": {
			DesiredTotal:    10,
			DesiredCanaries: 1,
		},
	}
	require.NoError(t, s.upsertDeploymentImpl(10, d, setupTx))

	// create set of allocs
	c1 := mock.Alloc()
	c1.JobID = j.ID
	c1.DeploymentID = d.ID
	d.TaskGroups[c1.TaskGroup].PlacedCanaries = append(d.TaskGroups[c1.TaskGroup].PlacedCanaries, c1.ID)
	c1.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: helper.BoolToPtr(true),
	}
	c2 := mock.Alloc()
	c2.JobID = j.ID
	c2.DeploymentID = d.ID
	d.TaskGroups[c2.TaskGroup].PlacedCanaries = append(d.TaskGroups[c2.TaskGroup].PlacedCanaries, c2.ID)
	c2.TaskGroup = tg2.Name
	c2.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: helper.BoolToPtr(true),
	}

	require.NoError(t, s.upsertAllocsImpl(10, []*structs.Allocation{c1, c2}, setupTx))

	// commit setup transaction
	setupTx.Txn.Commit()

	e := mock.Eval()
	// Request to promote canaries
	msgType := structs.DeploymentPromoteRequestType
	req := &structs.ApplyDeploymentPromoteRequest{
		DeploymentPromoteRequest: structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			All:          true,
		},
		Eval: e,
	}

	require.NoError(t, s.UpdateDeploymentPromotion(msgType, 100, req))

	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	require.Len(t, events, 4)

	got := events[0]
	require.Equal(t, uint64(100), got.Index)
	require.Equal(t, d.ID, got.Key)

	de := got.Payload.(*DeploymentEvent)
	require.Equal(t, structs.DeploymentStatusRunning, de.Deployment.Status)
	require.Equal(t, TypeDeploymentPromotion, got.Type)
}

func TestEventsFromChanges_DeploymentAllocHealthRequestType(t *testing.T) {
	t.Parallel()
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	// setup
	setupTx := s.db.WriteTxn(10)

	j := mock.Job()
	tg1 := j.TaskGroups[0]
	tg2 := tg1.Copy()
	tg2.Name = "foo"
	j.TaskGroups = append(j.TaskGroups, tg2)
	require.NoError(t, s.upsertJobImpl(10, j, false, setupTx))

	d := mock.Deployment()
	d.StatusDescription = structs.DeploymentStatusDescriptionRunningNeedsPromotion
	d.JobID = j.ID
	d.TaskGroups = map[string]*structs.DeploymentState{
		"web": {
			DesiredTotal:    10,
			DesiredCanaries: 1,
		},
		"foo": {
			DesiredTotal:    10,
			DesiredCanaries: 1,
		},
	}
	require.NoError(t, s.upsertDeploymentImpl(10, d, setupTx))

	// create set of allocs
	c1 := mock.Alloc()
	c1.JobID = j.ID
	c1.DeploymentID = d.ID
	d.TaskGroups[c1.TaskGroup].PlacedCanaries = append(d.TaskGroups[c1.TaskGroup].PlacedCanaries, c1.ID)
	c1.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: helper.BoolToPtr(true),
	}
	c2 := mock.Alloc()
	c2.JobID = j.ID
	c2.DeploymentID = d.ID
	d.TaskGroups[c2.TaskGroup].PlacedCanaries = append(d.TaskGroups[c2.TaskGroup].PlacedCanaries, c2.ID)
	c2.TaskGroup = tg2.Name
	c2.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: helper.BoolToPtr(true),
	}

	require.NoError(t, s.upsertAllocsImpl(10, []*structs.Allocation{c1, c2}, setupTx))

	// Commit setup
	setupTx.Commit()

	msgType := structs.DeploymentAllocHealthRequestType

	req := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:           d.ID,
			HealthyAllocationIDs:   []string{c1.ID},
			UnhealthyAllocationIDs: []string{c2.ID},
		},
		DeploymentUpdate: &structs.DeploymentStatusUpdate{
			DeploymentID: d.ID,
		},
	}

	require.NoError(t, s.UpdateDeploymentAllocHealth(msgType, 100, req))

	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	require.Len(t, events, 3)

	var allocEvents []structs.Event
	var deploymentEvent []structs.Event
	for _, e := range events {
		if e.Topic == structs.TopicAlloc {
			allocEvents = append(allocEvents, e)
		} else if e.Topic == structs.TopicDeployment {
			deploymentEvent = append(deploymentEvent, e)
		}
	}

	require.Len(t, allocEvents, 2)
	for _, e := range allocEvents {
		require.Equal(t, 100, int(e.Index))
		require.Equal(t, TypeDeploymentAllocHealth, e.Type)
		require.Equal(t, structs.TopicAlloc, e.Topic)
	}

	require.Len(t, deploymentEvent, 1)
	for _, e := range deploymentEvent {
		require.Equal(t, 100, int(e.Index))
		require.Equal(t, TypeDeploymentAllocHealth, e.Type)
		require.Equal(t, structs.TopicDeployment, e.Topic)
		require.Equal(t, d.ID, e.Key)
	}
}

func TestEventsFromChanges_UpsertNodeEventsType(t *testing.T) {
	t.Parallel()
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	// setup
	n1 := mock.Node()
	n2 := mock.Node()

	require.NoError(t, s.UpsertNode(structs.MsgTypeTestSetup, 10, n1))
	require.NoError(t, s.UpsertNode(structs.MsgTypeTestSetup, 12, n2))

	msgType := structs.UpsertNodeEventsType
	req := &structs.EmitNodeEventsRequest{
		NodeEvents: map[string][]*structs.NodeEvent{
			n1.ID: {
				{
					Message: "update",
				},
			},
			n2.ID: {
				{
					Message: "update",
				},
			},
		},
	}

	require.NoError(t, s.UpsertNodeEvents(msgType, 100, req.NodeEvents))
	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	require.Len(t, events, 2)

	for _, e := range events {
		require.Equal(t, structs.TopicNode, e.Topic)
		require.Equal(t, TypeNodeEvent, e.Type)
		event := e.Payload.(*NodeEvent)
		require.Equal(t, "update", event.Node.Events[len(event.Node.Events)-1].Message)
	}

}

func TestEventsFromChanges_NodeUpdateStatusRequest(t *testing.T) {
	t.Parallel()
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	// setup
	n1 := mock.Node()

	require.NoError(t, s.UpsertNode(structs.MsgTypeTestSetup, 10, n1))

	updated := time.Now()
	msgType := structs.NodeUpdateStatusRequestType
	req := &structs.NodeUpdateStatusRequest{
		NodeID:    n1.ID,
		Status:    structs.NodeStatusDown,
		UpdatedAt: updated.UnixNano(),
		NodeEvent: &structs.NodeEvent{Message: "down"},
	}

	require.NoError(t, s.UpdateNodeStatus(msgType, 100, req.NodeID, req.Status, req.UpdatedAt, req.NodeEvent))
	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	require.Len(t, events, 1)

	e := events[0]
	require.Equal(t, structs.TopicNode, e.Topic)
	require.Equal(t, TypeNodeEvent, e.Type)
	event := e.Payload.(*NodeEvent)
	require.Equal(t, "down", event.Node.Events[len(event.Node.Events)-1].Message)
	require.Equal(t, structs.NodeStatusDown, event.Node.Status)
}

func TestEventsFromChanges_EvalUpdateRequestType(t *testing.T) {
	t.Parallel()
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	// setup
	e1 := mock.Eval()

	require.NoError(t, s.UpsertEvals(structs.MsgTypeTestSetup, 10, []*structs.Evaluation{e1}))

	e2 := mock.Eval()
	e2.ID = e1.ID
	e2.JobID = e1.JobID
	e2.Status = structs.EvalStatusBlocked

	msgType := structs.EvalUpdateRequestType
	req := &structs.EvalUpdateRequest{
		Evals: []*structs.Evaluation{e2},
	}

	require.NoError(t, s.UpsertEvals(msgType, 100, req.Evals))

	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	require.Len(t, events, 1)

	e := events[0]
	require.Equal(t, structs.TopicEval, e.Topic)
	require.Equal(t, TypeEvalUpdated, e.Type)
	require.Contains(t, e.FilterKeys, e2.JobID)
	require.Contains(t, e.FilterKeys, e2.DeploymentID)
	event := e.Payload.(*EvalEvent)
	require.Equal(t, "blocked", event.Eval.Status)
}

func TestEventsFromChanges_ApplyPlanResultsRequestType(t *testing.T) {
	t.Parallel()
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	// setup
	alloc := mock.Alloc()
	alloc2 := mock.Alloc()
	job := alloc.Job
	alloc.Job = nil
	alloc2.Job = nil

	d := mock.Deployment()
	alloc.DeploymentID = d.ID
	alloc2.DeploymentID = d.ID

	require.NoError(t, s.UpsertJob(structs.MsgTypeTestSetup, 9, job))

	eval := mock.Eval()
	eval.JobID = job.ID

	// Create an eval
	require.NoError(t, s.UpsertEvals(structs.MsgTypeTestSetup, 10, []*structs.Evaluation{eval}))

	msgType := structs.ApplyPlanResultsRequestType
	req := &structs.ApplyPlanResultsRequest{
		AllocUpdateRequest: structs.AllocUpdateRequest{
			Alloc: []*structs.Allocation{alloc, alloc2},
			Job:   job,
		},
		Deployment: d,
		EvalID:     eval.ID,
	}

	require.NoError(t, s.UpsertPlanResults(msgType, 100, req))

	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	require.Len(t, events, 5)

	var allocs []structs.Event
	var evals []structs.Event
	var jobs []structs.Event
	var deploys []structs.Event
	for _, e := range events {
		if e.Topic == structs.TopicAlloc {
			allocs = append(allocs, e)
		} else if e.Topic == structs.TopicEval {
			evals = append(evals, e)
		} else if e.Topic == structs.TopicJob {
			jobs = append(jobs, e)
		} else if e.Topic == structs.TopicDeployment {
			deploys = append(deploys, e)
		}
		require.Equal(t, TypePlanResult, e.Type)
	}
	require.Len(t, allocs, 2)
	require.Len(t, evals, 1)
	require.Len(t, jobs, 1)
	require.Len(t, deploys, 1)
}

func TestEventsFromChanges_BatchNodeUpdateDrainRequestType(t *testing.T) {
	t.Parallel()
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	// setup
	n1 := mock.Node()
	n2 := mock.Node()

	require.NoError(t, s.UpsertNode(structs.MsgTypeTestSetup, 10, n1))
	require.NoError(t, s.UpsertNode(structs.MsgTypeTestSetup, 11, n2))

	updated := time.Now()
	msgType := structs.BatchNodeUpdateDrainRequestType

	expectedDrain := &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{
			Deadline: -1 * time.Second,
		},
	}
	event := &structs.NodeEvent{
		Message:   "Drain strategy enabled",
		Subsystem: structs.NodeEventSubsystemDrain,
		Timestamp: time.Now(),
	}
	req := structs.BatchNodeUpdateDrainRequest{
		Updates: map[string]*structs.DrainUpdate{
			n1.ID: {
				DrainStrategy: expectedDrain,
			},
			n2.ID: {
				DrainStrategy: expectedDrain,
			},
		},
		NodeEvents: map[string]*structs.NodeEvent{
			n1.ID: event,
			n2.ID: event,
		},
		UpdatedAt: updated.UnixNano(),
	}

	require.NoError(t, s.BatchUpdateNodeDrain(msgType, 100, req.UpdatedAt, req.Updates, req.NodeEvents))

	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	require.Len(t, events, 2)

	for _, e := range events {
		require.Equal(t, 100, int(e.Index))
		require.Equal(t, TypeNodeDrain, e.Type)
		require.Equal(t, structs.TopicNode, e.Topic)
		ne := e.Payload.(*NodeEvent)
		require.Equal(t, event.Message, ne.Node.Events[len(ne.Node.Events)-1].Message)
	}
}

func TestEventsFromChanges_NodeUpdateEligibilityRequestType(t *testing.T) {
	t.Parallel()
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	// setup
	n1 := mock.Node()

	require.NoError(t, s.UpsertNode(structs.MsgTypeTestSetup, 10, n1))

	msgType := structs.NodeUpdateEligibilityRequestType

	event := &structs.NodeEvent{
		Message:   "Node marked as ineligible",
		Subsystem: structs.NodeEventSubsystemCluster,
		Timestamp: time.Now(),
	}

	req := structs.NodeUpdateEligibilityRequest{
		NodeID:      n1.ID,
		NodeEvent:   event,
		Eligibility: structs.NodeSchedulingIneligible,
		UpdatedAt:   time.Now().UnixNano(),
	}

	require.NoError(t, s.UpdateNodeEligibility(msgType, 100, req.NodeID, req.Eligibility, req.UpdatedAt, req.NodeEvent))

	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	require.Len(t, events, 1)

	for _, e := range events {
		require.Equal(t, 100, int(e.Index))
		require.Equal(t, TypeNodeDrain, e.Type)
		require.Equal(t, structs.TopicNode, e.Topic)
		ne := e.Payload.(*NodeEvent)
		require.Equal(t, event.Message, ne.Node.Events[len(ne.Node.Events)-1].Message)
		require.Equal(t, structs.NodeSchedulingIneligible, ne.Node.SchedulingEligibility)
	}
}

func TestEventsFromChanges_AllocUpdateDesiredTransitionRequestType(t *testing.T) {
	t.Parallel()
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	alloc := mock.Alloc()

	require.Nil(t, s.UpsertJob(structs.MsgTypeTestSetup, 10, alloc.Job))
	require.Nil(t, s.UpsertAllocs(structs.MsgTypeTestSetup, 11, []*structs.Allocation{alloc}))

	msgType := structs.AllocUpdateDesiredTransitionRequestType

	eval := &structs.Evaluation{
		ID:             uuid.Generate(),
		Namespace:      alloc.Namespace,
		Priority:       alloc.Job.Priority,
		Type:           alloc.Job.Type,
		TriggeredBy:    structs.EvalTriggerNodeDrain,
		JobID:          alloc.Job.ID,
		JobModifyIndex: alloc.Job.ModifyIndex,
		Status:         structs.EvalStatusPending,
	}
	evals := []*structs.Evaluation{eval}

	req := &structs.AllocUpdateDesiredTransitionRequest{
		Allocs: map[string]*structs.DesiredTransition{
			alloc.ID: {Migrate: helper.BoolToPtr(true)},
		},
		Evals: evals,
	}

	require.NoError(t, s.UpdateAllocsDesiredTransitions(msgType, 100, req.Allocs, req.Evals))

	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	require.Len(t, events, 2)

	var allocs []structs.Event
	var evalEvents []structs.Event
	for _, e := range events {
		if e.Topic == structs.TopicEval {
			evalEvents = append(evalEvents, e)
		} else if e.Topic == structs.TopicAlloc {
			allocs = append(allocs, e)
		} else {
			require.Fail(t, "unexpected event type")
		}

		require.Equal(t, TypeAllocUpdateDesiredStatus, e.Type)
	}

	require.Len(t, allocs, 1)
	require.Len(t, evalEvents, 1)
}

func TestEventsFromChanges_JobBatchDeregisterRequestType(t *testing.T) {
	// TODO Job batch deregister logic mostly occurs in the FSM
	t.SkipNow()

}
func TestEventsFromChanges_AllocClientUpdateRequestType(t *testing.T) {
	t.SkipNow()
}

func TestEventsFromChanges_AllocUpdateRequestType(t *testing.T) {
	t.SkipNow()
}

func TestEventsFromChanges_JobDeregisterRequestType(t *testing.T) {
	t.SkipNow()
}

func TestEventsFromChanges_WithDeletion(t *testing.T) {
	t.Parallel()

	changes := Changes{
		Index: uint64(1),
		Changes: memdb.Changes{
			{
				Before: &structs.Job{},
				After:  &structs.Job{},
			},
			{
				Before: &structs.Job{},
				After:  nil, // deleted
			},
		},
		MsgType: structs.JobDeregisterRequestType,
	}

	event := eventsFromChanges(nil, changes)
	require.NotNil(t, event)

	require.Len(t, event.Events, 1)
}

func TestEventsFromChanges_WithNodeDeregistration(t *testing.T) {
	t.Parallel()

	before := &structs.Node{
		ID:         "some-id",
		Datacenter: "some-datacenter",
	}

	changes := Changes{
		Index: uint64(1),
		Changes: memdb.Changes{
			{
				Before: before,
				After:  nil, // deleted
			},
		},
		MsgType: structs.NodeDeregisterRequestType,
	}

	actual := eventsFromChanges(nil, changes)
	require.NotNil(t, actual)

	require.Len(t, actual.Events, 1)

	event := actual.Events[0]

	require.Equal(t, TypeNodeDeregistration, event.Type)
	require.Equal(t, uint64(1), event.Index)
	require.Equal(t, structs.TopicNode, event.Topic)
	require.Equal(t, "some-id", event.Key)

	require.Len(t, event.FilterKeys, 0)

	nodeEvent, ok := event.Payload.(*NodeEvent)
	require.True(t, ok)
	require.Equal(t, *before, *nodeEvent.Node)
}

func TestNodeEventsFromChanges(t *testing.T) {
	cases := []struct {
		Name       string
		MsgType    structs.MessageType
		Setup      func(s *StateStore, tx *txn) error
		Mutate     func(s *StateStore, tx *txn) error
		WantEvents []structs.Event
		WantTopic  structs.Topic
	}{
		{
			MsgType:   structs.NodeRegisterRequestType,
			WantTopic: structs.TopicNode,
			Name:      "node registered",
			Mutate: func(s *StateStore, tx *txn) error {
				return upsertNodeTxn(tx, tx.Index, testNode())
			},
			WantEvents: []structs.Event{{
				Topic: structs.TopicNode,
				Type:  TypeNodeRegistration,
				Key:   testNodeID(),
				Index: 100,
				Payload: &NodeEvent{
					Node: testNode(),
				},
			}},
		},
		{
			MsgType:   structs.NodeRegisterRequestType,
			WantTopic: structs.TopicNode,
			Name:      "node registered initializing",
			Mutate: func(s *StateStore, tx *txn) error {
				return upsertNodeTxn(tx, tx.Index, testNode(nodeNotReady))
			},
			WantEvents: []structs.Event{{
				Topic: structs.TopicNode,
				Type:  TypeNodeRegistration,
				Key:   testNodeID(),
				Index: 100,
				Payload: &NodeEvent{
					Node: testNode(nodeNotReady),
				},
			}},
		},
		{
			MsgType:   structs.NodeDeregisterRequestType,
			WantTopic: structs.TopicNode,
			Name:      "node deregistered",
			Setup: func(s *StateStore, tx *txn) error {
				return upsertNodeTxn(tx, tx.Index, testNode())
			},
			Mutate: func(s *StateStore, tx *txn) error {
				return deleteNodeTxn(tx, tx.Index, []string{testNodeID()})
			},
			WantEvents: []structs.Event{{
				Topic: structs.TopicNode,
				Type:  TypeNodeDeregistration,
				Key:   testNodeID(),
				Index: 100,
				Payload: &NodeEvent{
					Node: testNode(),
				},
			}},
		},
		{
			MsgType:   structs.NodeDeregisterRequestType,
			WantTopic: structs.TopicNode,
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
					Topic: structs.TopicNode,
					Type:  TypeNodeDeregistration,
					Key:   testNodeID(),
					Index: 100,
					Payload: &NodeEvent{
						Node: testNode(),
					},
				},
				{
					Topic: structs.TopicNode,
					Type:  TypeNodeDeregistration,
					Key:   testNodeIDTwo(),
					Index: 100,
					Payload: &NodeEvent{
						Node: testNode(nodeIDTwo),
					},
				},
			},
		},
		{
			MsgType:   structs.UpsertNodeEventsType,
			WantTopic: structs.TopicNode,
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
					Topic: structs.TopicNode,
					Type:  TypeNodeEvent,
					Key:   testNodeID(),
					Index: 100,
					Payload: &NodeEvent{
						Node: testNode(),
					},
				},
				{
					Topic: structs.TopicNode,
					Type:  TypeNodeEvent,
					Key:   testNodeIDTwo(),
					Index: 100,
					Payload: &NodeEvent{
						Node: testNode(nodeIDTwo),
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			s := TestStateStoreCfg(t, TestStateStorePublisher(t))
			defer s.StopEventBroker()

			if tc.Setup != nil {
				// Bypass publish mechanism for setup
				setupTx := s.db.WriteTxn(10)
				require.NoError(t, tc.Setup(s, setupTx))
				setupTx.Txn.Commit()
			}

			tx := s.db.WriteTxnMsgT(tc.MsgType, 100)
			require.NoError(t, tc.Mutate(s, tx))

			changes := Changes{Changes: tx.Changes(), Index: 100, MsgType: tc.MsgType}
			got := eventsFromChanges(tx, changes)

			require.NotNil(t, got)

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
	defer s.StopEventBroker()

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
	got := eventsFromChanges(tx, changes)

	require.Len(t, got.Events, 1)

	require.Equal(t, structs.TopicNode, got.Events[0].Topic)
	require.Equal(t, TypeNodeDrain, got.Events[0].Type)
	require.Equal(t, uint64(100), got.Events[0].Index)

	nodeEvent, ok := got.Events[0].Payload.(*NodeEvent)
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
