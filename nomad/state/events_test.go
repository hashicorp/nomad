package state

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

// structs.AllocClientUpdateRequestType:
// structs.AllocUpdateRequestType
// JobDeregisterRequestType
// jobregisterrequesttype

func TestGenericEventsFromChanges_DeploymentUpdate(t *testing.T) {
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

func TestGenericEventsFromChanges_DeploymentPromotion(t *testing.T) {
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

func TestGenericEventsFromChanges_DeploymentAllocHealthRequestType(t *testing.T) {
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

func TestGenericEventsFromChanges_UpsertNodeEventsType(t *testing.T) {
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

func TestGenericEventsFromChanges_NodeUpdateStatusRequest(t *testing.T) {
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

func TestGenericEventsFromChanges_EvalUpdateRequestType(t *testing.T) {
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

func TestGenericEventsFromChanges_ApplyPlanResultsRequestType(t *testing.T) {
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

func TestGenericEventsFromChanges_BatchNodeUpdateDrainRequestType(t *testing.T) {
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

func TestGenericEventsFromChanges_NodeUpdateEligibilityRequestType(t *testing.T) {
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

func TestGenericEventsFromChanges_AllocUpdateDesiredTransitionRequestType(t *testing.T) {
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

func TestGenericEventsFromChanges_JobBatchDeregisterRequestType(t *testing.T) {
	// TODO Job batch deregister logic mostly occurs in the FSM
	t.SkipNow()

}
func TestGenericEventsFromChanges_AllocClientUpdateRequestType(t *testing.T) {
	t.SkipNow()
}

func TestGenericEventsFromChanges_AllocUpdateRequestType(t *testing.T) {
	t.SkipNow()
}

func TestGenericEventsFromChanges_JobDeregisterRequestType(t *testing.T) {
	t.SkipNow()
}
