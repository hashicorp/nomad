// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

// TestEventFromChange_SingleEventPerTable ensures that only a single event is
// created per table per memdb.Change
func TestEventFromChange_SingleEventPerTable(t *testing.T) {
	ci.Parallel(t)
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	changes := Changes{
		Index:   100,
		MsgType: structs.JobRegisterRequestType,
		Changes: memdb.Changes{
			{
				Table:  "job_version",
				Before: mock.Job(),
				After:  mock.Job(),
			},
			{
				Table:  "jobs",
				Before: mock.Job(),
				After:  mock.Job(),
			},
		},
	}

	out := eventsFromChanges(s.db.ReadTxn(), changes)
	must.Len(t, 1, out.Events)
	must.Eq(t, out.Events[0].Type, structs.TypeJobRegistered)
}

func TestEventFromChange_ACLTokenSecretID(t *testing.T) {
	ci.Parallel(t)
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	token := mock.ACLToken()
	must.NotEq(t, "", token.SecretID)

	// Create
	changes := Changes{
		Index:   100,
		MsgType: structs.NodeRegisterRequestType,
		Changes: memdb.Changes{
			{
				Table:  "acl_token",
				Before: nil,
				After:  token,
			},
		},
	}

	out := eventsFromChanges(s.db.ReadTxn(), changes)
	must.Len(t, 1, out.Events)
	// Ensure original value not altered
	must.NotEq(t, "", token.SecretID)

	aclTokenEvent, ok := out.Events[0].Payload.(*structs.ACLTokenEvent)
	must.True(t, ok)
	must.Eq(t, "", aclTokenEvent.ACLToken.SecretID)

	must.Eq(t, token.SecretID, aclTokenEvent.SecretID())

	// Delete
	changes = Changes{
		Index:   100,
		MsgType: structs.NodeDeregisterRequestType,
		Changes: memdb.Changes{
			{
				Table:  "acl_token",
				Before: token,
				After:  nil,
			},
		},
	}

	out2 := eventsFromChanges(s.db.ReadTxn(), changes)
	must.Len(t, 1, out2.Events)

	tokenEvent2, ok := out2.Events[0].Payload.(*structs.ACLTokenEvent)
	must.True(t, ok)
	must.Eq(t, "", tokenEvent2.ACLToken.SecretID)
}

func TestEventsFromChanges_DeploymentUpdate(t *testing.T) {
	ci.Parallel(t)
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	// setup
	setupTx := s.db.WriteTxn(10)

	j := mock.Job()
	e := mock.Eval()
	e.JobID = j.ID

	d := mock.Deployment()
	d.JobID = j.ID

	must.NoError(t, s.upsertJobImpl(10, nil, j, false, setupTx))
	must.NoError(t, s.upsertDeploymentImpl(10, d, setupTx))

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

	must.NoError(t, s.UpdateDeploymentStatus(msgType, 100, req))

	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	must.Len(t, 2, events)

	got := events[0]
	must.Eq(t, uint64(100), got.Index)
	must.Eq(t, d.ID, got.Key)

	de := got.Payload.(*structs.DeploymentEvent)
	must.Eq(t, structs.DeploymentStatusPaused, de.Deployment.Status)
	must.SliceContains(t, got.FilterKeys, j.ID)
}

func TestEventsFromChanges_DeploymentPromotion(t *testing.T) {
	ci.Parallel(t)
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	// setup
	setupTx := s.db.WriteTxn(10)

	j := mock.Job()
	tg1 := j.TaskGroups[0]
	tg2 := tg1.Copy()
	tg2.Name = "foo"
	j.TaskGroups = append(j.TaskGroups, tg2)
	must.NoError(t, s.upsertJobImpl(10, nil, j, false, setupTx))

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
	must.NoError(t, s.upsertDeploymentImpl(10, d, setupTx))

	// create set of allocs
	c1 := mock.Alloc()
	c1.JobID = j.ID
	c1.DeploymentID = d.ID
	d.TaskGroups[c1.TaskGroup].PlacedCanaries = append(d.TaskGroups[c1.TaskGroup].PlacedCanaries, c1.ID)
	c1.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: pointer.Of(true),
	}
	c2 := mock.Alloc()
	c2.JobID = j.ID
	c2.DeploymentID = d.ID
	d.TaskGroups[c2.TaskGroup].PlacedCanaries = append(d.TaskGroups[c2.TaskGroup].PlacedCanaries, c2.ID)
	c2.TaskGroup = tg2.Name
	c2.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: pointer.Of(true),
	}

	must.NoError(t, s.upsertAllocsImpl(10, []*structs.Allocation{c1, c2}, setupTx))

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

	must.NoError(t, s.UpdateDeploymentPromotion(msgType, 100, req))

	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	must.Len(t, 4, events)

	got := events[0]
	must.Eq(t, uint64(100), got.Index)
	must.Eq(t, d.ID, got.Key)

	de := got.Payload.(*structs.DeploymentEvent)
	must.Eq(t, structs.DeploymentStatusRunning, de.Deployment.Status)
	must.Eq(t, structs.TypeDeploymentPromotion, got.Type)
}

func TestEventsFromChanges_DeploymentAllocHealthRequestType(t *testing.T) {
	ci.Parallel(t)
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	// setup
	setupTx := s.db.WriteTxn(10)

	j := mock.Job()
	tg1 := j.TaskGroups[0]
	tg2 := tg1.Copy()
	tg2.Name = "foo"
	j.TaskGroups = append(j.TaskGroups, tg2)
	must.NoError(t, s.upsertJobImpl(10, nil, j, false, setupTx))

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
	must.NoError(t, s.upsertDeploymentImpl(10, d, setupTx))

	// create set of allocs
	c1 := mock.Alloc()
	c1.JobID = j.ID
	c1.DeploymentID = d.ID
	d.TaskGroups[c1.TaskGroup].PlacedCanaries = append(d.TaskGroups[c1.TaskGroup].PlacedCanaries, c1.ID)
	c1.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: pointer.Of(true),
	}
	c2 := mock.Alloc()
	c2.JobID = j.ID
	c2.DeploymentID = d.ID
	d.TaskGroups[c2.TaskGroup].PlacedCanaries = append(d.TaskGroups[c2.TaskGroup].PlacedCanaries, c2.ID)
	c2.TaskGroup = tg2.Name
	c2.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: pointer.Of(true),
	}

	must.NoError(t, s.upsertAllocsImpl(10, []*structs.Allocation{c1, c2}, setupTx))

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

	must.NoError(t, s.UpdateDeploymentAllocHealth(msgType, 100, req))

	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	must.Len(t, 3, events)

	var allocEvents []structs.Event
	var deploymentEvent []structs.Event
	for _, e := range events {
		if e.Topic == structs.TopicAllocation {
			allocEvents = append(allocEvents, e)
		} else if e.Topic == structs.TopicDeployment {
			deploymentEvent = append(deploymentEvent, e)
		}
	}

	must.Len(t, 2, allocEvents)
	for _, e := range allocEvents {
		must.Eq(t, 100, int(e.Index))
		must.Eq(t, structs.TypeDeploymentAllocHealth, e.Type)
		must.Eq(t, structs.TopicAllocation, e.Topic)
	}

	must.Len(t, 1, deploymentEvent)
	for _, e := range deploymentEvent {
		must.Eq(t, 100, int(e.Index))
		must.Eq(t, structs.TypeDeploymentAllocHealth, e.Type)
		must.Eq(t, structs.TopicDeployment, e.Topic)
		must.Eq(t, d.ID, e.Key)
	}
}

func TestEventsFromChanges_UpsertNodeEventsType(t *testing.T) {
	ci.Parallel(t)
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	// setup
	n1 := mock.Node()
	n2 := mock.Node()

	must.NoError(t, s.UpsertNode(structs.MsgTypeTestSetup, 10, n1))
	must.NoError(t, s.UpsertNode(structs.MsgTypeTestSetup, 12, n2))

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

	must.NoError(t, s.UpsertNodeEvents(msgType, 100, req.NodeEvents))
	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	must.Len(t, 2, events)

	for _, e := range events {
		must.Eq(t, structs.TopicNode, e.Topic)
		must.Eq(t, structs.TypeNodeEvent, e.Type)
		event := e.Payload.(*structs.NodeStreamEvent)
		must.Eq(t, "update", event.Node.Events[len(event.Node.Events)-1].Message)
	}

}

func TestEventsFromChanges_NodeUpdateStatusRequest(t *testing.T) {
	ci.Parallel(t)
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	// setup
	n1 := mock.Node()

	must.NoError(t, s.UpsertNode(structs.MsgTypeTestSetup, 10, n1))

	updated := time.Now()
	msgType := structs.NodeUpdateStatusRequestType
	req := &structs.NodeUpdateStatusRequest{
		NodeID:    n1.ID,
		Status:    structs.NodeStatusDown,
		UpdatedAt: updated.UnixNano(),
		NodeEvent: &structs.NodeEvent{Message: "down"},
	}

	must.NoError(t, s.UpdateNodeStatus(msgType, 100, req.NodeID, req.Status, req.UpdatedAt, req.NodeEvent))
	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	must.Len(t, 1, events)

	e := events[0]
	must.Eq(t, structs.TopicNode, e.Topic)
	must.Eq(t, structs.TypeNodeEvent, e.Type)
	event := e.Payload.(*structs.NodeStreamEvent)
	must.Eq(t, "down", event.Node.Events[len(event.Node.Events)-1].Message)
	must.Eq(t, structs.NodeStatusDown, event.Node.Status)
}

func TestEventsFromChanges_NodePoolUpsertRequestType(t *testing.T) {
	ci.Parallel(t)
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	// Create test node pool.
	pool := mock.NodePool()
	err := s.UpsertNodePools(structs.MsgTypeTestSetup, 1000, []*structs.NodePool{pool})
	must.NoError(t, err)

	// Update test node pool.
	updated := pool.Copy()
	updated.Meta["updated"] = "true"
	err = s.UpsertNodePools(structs.NodePoolUpsertRequestType, 1001, []*structs.NodePool{updated})
	must.NoError(t, err)

	// Wait and verify update event.
	events := WaitForEvents(t, s, 1001, 1, 1*time.Second)
	must.Len(t, 1, events)

	e := events[0]
	must.Eq(t, structs.TopicNodePool, e.Topic)
	must.Eq(t, structs.TypeNodePoolUpserted, e.Type)
	must.Eq(t, pool.Name, e.Key)

	payload := e.Payload.(*structs.NodePoolEvent)
	must.Eq(t, updated, payload.NodePool)
}

func TestEventsFromChanges_NodePoolDeleteRequestType(t *testing.T) {
	ci.Parallel(t)
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	// Create test node pool.
	pool := mock.NodePool()
	err := s.UpsertNodePools(structs.MsgTypeTestSetup, 1000, []*structs.NodePool{pool})
	must.NoError(t, err)

	// Delete test node pool.
	err = s.DeleteNodePools(structs.NodePoolDeleteRequestType, 1001, []string{pool.Name})
	must.NoError(t, err)

	// Wait and verify delete event.
	events := WaitForEvents(t, s, 1001, 1, 1*time.Second)
	must.Len(t, 1, events)

	e := events[0]
	must.Eq(t, structs.TopicNodePool, e.Topic)
	must.Eq(t, structs.TypeNodePoolDeleted, e.Type)
	must.Eq(t, pool.Name, e.Key)

	payload := e.Payload.(*structs.NodePoolEvent)
	must.Eq(t, pool, payload.NodePool)
}

func TestEventsFromChanges_EvalUpdateRequestType(t *testing.T) {
	ci.Parallel(t)
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	// setup
	e1 := mock.Eval()

	must.NoError(t, s.UpsertEvals(structs.MsgTypeTestSetup, 10, []*structs.Evaluation{e1}))

	e2 := mock.Eval()
	e2.ID = e1.ID
	e2.JobID = e1.JobID
	e2.Status = structs.EvalStatusBlocked

	msgType := structs.EvalUpdateRequestType
	req := &structs.EvalUpdateRequest{
		Evals: []*structs.Evaluation{e2},
	}

	must.NoError(t, s.UpsertEvals(msgType, 100, req.Evals))

	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	must.Len(t, 1, events)

	e := events[0]
	must.Eq(t, structs.TopicEvaluation, e.Topic)
	must.Eq(t, structs.TypeEvalUpdated, e.Type)
	must.SliceContains(t, e.FilterKeys, e2.JobID)
	must.SliceContains(t, e.FilterKeys, e2.DeploymentID)
	event := e.Payload.(*structs.EvaluationEvent)
	must.Eq(t, "blocked", event.Evaluation.Status)
}

func TestEventsFromChanges_ApplyPlanResultsRequestType(t *testing.T) {
	ci.Parallel(t)
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

	must.NoError(t, s.UpsertJob(structs.MsgTypeTestSetup, 9, nil, job))

	eval := mock.Eval()
	eval.JobID = job.ID

	// Create an eval
	must.NoError(t, s.UpsertEvals(structs.MsgTypeTestSetup, 10, []*structs.Evaluation{eval}))

	msgType := structs.ApplyPlanResultsRequestType
	req := &structs.ApplyPlanResultsRequest{
		AllocUpdateRequest: structs.AllocUpdateRequest{
			Alloc: []*structs.Allocation{alloc, alloc2},
			Job:   job,
		},
		Deployment: d,
		EvalID:     eval.ID,
	}

	must.NoError(t, s.UpsertPlanResults(msgType, 100, req))

	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	must.Len(t, 5, events)

	var allocs []structs.Event
	var evals []structs.Event
	var jobs []structs.Event
	var deploys []structs.Event
	for _, e := range events {
		if e.Topic == structs.TopicAllocation {
			allocs = append(allocs, e)
		} else if e.Topic == structs.TopicEvaluation {
			evals = append(evals, e)
		} else if e.Topic == structs.TopicJob {
			jobs = append(jobs, e)
		} else if e.Topic == structs.TopicDeployment {
			deploys = append(deploys, e)
		}
		must.Eq(t, structs.TypePlanResult, e.Type)
	}
	must.Len(t, 2, allocs)
	must.Len(t, 1, evals)
	must.Len(t, 1, jobs)
	must.Len(t, 1, deploys)
}

func TestEventsFromChanges_BatchNodeUpdateDrainRequestType(t *testing.T) {
	ci.Parallel(t)
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	// setup
	n1 := mock.Node()
	n2 := mock.Node()

	must.NoError(t, s.UpsertNode(structs.MsgTypeTestSetup, 10, n1))
	must.NoError(t, s.UpsertNode(structs.MsgTypeTestSetup, 11, n2))

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

	must.NoError(t, s.BatchUpdateNodeDrain(msgType, 100, req.UpdatedAt, req.Updates, req.NodeEvents))

	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	must.Len(t, 2, events)

	for _, e := range events {
		must.Eq(t, 100, int(e.Index))
		must.Eq(t, structs.TypeNodeDrain, e.Type)
		must.Eq(t, structs.TopicNode, e.Topic)
		ne := e.Payload.(*structs.NodeStreamEvent)
		must.Eq(t, event.Message, ne.Node.Events[len(ne.Node.Events)-1].Message)
	}
}

func TestEventsFromChanges_NodeUpdateEligibilityRequestType(t *testing.T) {
	ci.Parallel(t)
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	// setup
	n1 := mock.Node()

	must.NoError(t, s.UpsertNode(structs.MsgTypeTestSetup, 10, n1))

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

	must.NoError(t, s.UpdateNodeEligibility(msgType, 100, req.NodeID, req.Eligibility, req.UpdatedAt, req.NodeEvent))

	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	must.Len(t, 1, events)

	for _, e := range events {
		must.Eq(t, 100, int(e.Index))
		must.Eq(t, structs.TypeNodeDrain, e.Type)
		must.Eq(t, structs.TopicNode, e.Topic)
		ne := e.Payload.(*structs.NodeStreamEvent)
		must.Eq(t, event.Message, ne.Node.Events[len(ne.Node.Events)-1].Message)
		must.Eq(t, structs.NodeSchedulingIneligible, ne.Node.SchedulingEligibility)
	}
}

func TestEventsFromChanges_AllocUpdateDesiredTransitionRequestType(t *testing.T) {
	ci.Parallel(t)
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	alloc := mock.Alloc()

	must.Nil(t, s.UpsertJob(structs.MsgTypeTestSetup, 10, nil, alloc.Job))
	must.Nil(t, s.UpsertAllocs(structs.MsgTypeTestSetup, 11, []*structs.Allocation{alloc}))

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
			alloc.ID: {Migrate: pointer.Of(true)},
		},
		Evals: evals,
	}

	must.NoError(t, s.UpdateAllocsDesiredTransitions(msgType, 100, req.Allocs, req.Evals))

	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	must.Len(t, 2, events)

	var allocs []structs.Event
	var evalEvents []structs.Event
	for _, e := range events {
		if e.Topic == structs.TopicEvaluation {
			evalEvents = append(evalEvents, e)
		} else if e.Topic == structs.TopicAllocation {
			allocs = append(allocs, e)
		} else {
			t.Fatal("unexpected event type")
		}

		must.Eq(t, structs.TypeAllocationUpdateDesiredStatus, e.Type)
	}

	must.Len(t, 1, allocs)
	must.Len(t, 1, evalEvents)
}

func TestEventsFromChanges_JobBatchDeregisterRequestType(t *testing.T) {
	// TODO Job batch deregister logic mostly occurs in the FSM
	t.SkipNow()

}
func TestEventsFromChanges_AllocClientUpdateRequestType(t *testing.T) {
	t.SkipNow()
}

func TestEventsFromChanges_JobDeregisterRequestType(t *testing.T) {
	t.SkipNow()
}

func TestEventsFromChanges_WithDeletion(t *testing.T) {
	ci.Parallel(t)

	changes := Changes{
		Index: uint64(1),
		Changes: memdb.Changes{
			{
				Table:  "jobs",
				Before: &structs.Job{},
				After:  &structs.Job{},
			},
			{
				Table:  "jobs",
				Before: &structs.Job{},
				After:  nil, // deleted
			},
		},
		MsgType: structs.JobDeregisterRequestType,
	}

	event := eventsFromChanges(nil, changes)
	must.NotNil(t, event)

	must.Len(t, 2, event.Events)
}

func TestEventsFromChanges_WithNodeDeregistration(t *testing.T) {
	ci.Parallel(t)

	before := &structs.Node{
		ID:         "some-id",
		Datacenter: "some-datacenter",
	}

	changes := Changes{
		Index: uint64(1),
		Changes: memdb.Changes{
			{
				Table:  "nodes",
				Before: before,
				After:  nil, // deleted
			},
		},
		MsgType: structs.NodeDeregisterRequestType,
	}

	actual := eventsFromChanges(nil, changes)
	must.NotNil(t, actual)

	must.Len(t, 1, actual.Events)

	event := actual.Events[0]

	must.Eq(t, structs.TypeNodeDeregistration, event.Type)
	must.Eq(t, uint64(1), event.Index)
	must.Eq(t, structs.TopicNode, event.Topic)
	must.Eq(t, "some-id", event.Key)

	must.Len(t, 0, event.FilterKeys)

	nodeEvent, ok := event.Payload.(*structs.NodeStreamEvent)
	must.True(t, ok)
	must.Eq(t, *before, *nodeEvent.Node)
}

func TestNodeEventsFromChanges(t *testing.T) {
	ci.Parallel(t)

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
				Type:  structs.TypeNodeRegistration,
				Key:   testNodeID(),
				Index: 100,
				Payload: &structs.NodeStreamEvent{
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
				Type:  structs.TypeNodeRegistration,
				Key:   testNodeID(),
				Index: 100,
				Payload: &structs.NodeStreamEvent{
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
				return s.deleteNodeTxn(tx, tx.Index, []string{testNodeID()})
			},
			WantEvents: []structs.Event{{
				Topic: structs.TopicNode,
				Type:  structs.TypeNodeDeregistration,
				Key:   testNodeID(),
				Index: 100,
				Payload: &structs.NodeStreamEvent{
					Node: testNode(),
				},
			}},
		},
		{
			MsgType:   structs.NodeDeregisterRequestType,
			WantTopic: structs.TopicNode,
			Name:      "batch node deregistered",
			Setup: func(s *StateStore, tx *txn) error {
				must.NoError(t, upsertNodeTxn(tx, tx.Index, testNode()))
				return upsertNodeTxn(tx, tx.Index, testNode(nodeIDTwo))
			},
			Mutate: func(s *StateStore, tx *txn) error {
				return s.deleteNodeTxn(tx, tx.Index, []string{testNodeID(), testNodeIDTwo()})
			},
			WantEvents: []structs.Event{
				{
					Topic: structs.TopicNode,
					Type:  structs.TypeNodeDeregistration,
					Key:   testNodeID(),
					Index: 100,
					Payload: &structs.NodeStreamEvent{
						Node: testNode(),
					},
				},
				{
					Topic: structs.TopicNode,
					Type:  structs.TypeNodeDeregistration,
					Key:   testNodeIDTwo(),
					Index: 100,
					Payload: &structs.NodeStreamEvent{
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
				must.NoError(t, upsertNodeTxn(tx, tx.Index, testNode()))
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
				must.NoError(t, s.upsertNodeEvents(tx.Index, testNodeID(), eventFn(testNodeID()), tx))
				return s.upsertNodeEvents(tx.Index, testNodeIDTwo(), eventFn(testNodeIDTwo()), tx)
			},
			WantEvents: []structs.Event{
				{
					Topic: structs.TopicNode,
					Type:  structs.TypeNodeEvent,
					Key:   testNodeID(),
					Index: 100,
					Payload: &structs.NodeStreamEvent{
						Node: testNode(),
					},
				},
				{
					Topic: structs.TopicNode,
					Type:  structs.TypeNodeEvent,
					Key:   testNodeIDTwo(),
					Index: 100,
					Payload: &structs.NodeStreamEvent{
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
				must.NoError(t, tc.Setup(s, setupTx))
				setupTx.Txn.Commit()
			}

			tx := s.db.WriteTxnMsgT(tc.MsgType, 100)
			must.NoError(t, tc.Mutate(s, tx))

			changes := Changes{Changes: tx.Changes(), Index: 100, MsgType: tc.MsgType}
			got := eventsFromChanges(tx, changes)

			must.NotNil(t, got)

			must.Eq(t, len(tc.WantEvents), len(got.Events))
			for idx, g := range got.Events {
				// assert equality of shared fields

				want := tc.WantEvents[idx]
				must.Eq(t, want.Index, g.Index)
				must.Eq(t, want.Key, g.Key)
				must.Eq(t, want.Topic, g.Topic)

				switch tc.MsgType {
				case structs.NodeRegisterRequestType:
					requireNodeRegistrationEventEqual(t, tc.WantEvents[idx], g)
				case structs.NodeDeregisterRequestType:
					requireNodeDeregistrationEventEqual(t, tc.WantEvents[idx], g)
				case structs.UpsertNodeEventsType:
					requireNodeEventCount(t, 3, g)
				default:
					t.Fatal("unhandled message type")
				}
			}
		})
	}
}

func TestNodeDrainEventFromChanges(t *testing.T) {
	ci.Parallel(t)
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	// setup
	setupTx := s.db.WriteTxn(10)

	node := mock.Node()
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()
	alloc1.NodeID = node.ID
	alloc2.NodeID = node.ID

	must.NoError(t, upsertNodeTxn(setupTx, 10, node))
	must.NoError(t, s.upsertAllocsImpl(100, []*structs.Allocation{alloc1, alloc2}, setupTx))
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

	must.NoError(t, s.updateNodeDrainImpl(tx, 100, node.ID, strat, markEligible, updatedAt.UnixNano(), event, nil, "", false))
	changes := Changes{Changes: tx.Changes(), Index: 100, MsgType: structs.NodeUpdateDrainRequestType}
	got := eventsFromChanges(tx, changes)

	must.Len(t, 1, got.Events)

	must.Eq(t, structs.TopicNode, got.Events[0].Topic)
	must.Eq(t, structs.TypeNodeDrain, got.Events[0].Type)
	must.Eq(t, uint64(100), got.Events[0].Index)

	nodeEvent, ok := got.Events[0].Payload.(*structs.NodeStreamEvent)
	must.True(t, ok)

	must.Eq(t, structs.NodeSchedulingIneligible, nodeEvent.Node.SchedulingEligibility)
	must.Eq(t, strat, nodeEvent.Node.DrainStrategy)
}

func Test_eventsFromChanges_ServiceRegistration(t *testing.T) {
	ci.Parallel(t)
	testState := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer testState.StopEventBroker()

	// Generate test service registration.
	service := mock.ServiceRegistrations()[0]

	// Upsert a service registration.
	writeTxn := testState.db.WriteTxn(10)
	updated, err := testState.upsertServiceRegistrationTxn(10, writeTxn, service)
	must.True(t, updated)
	must.NoError(t, err)
	writeTxn.Txn.Commit()

	// Pull the events from the stream.
	registerChange := Changes{Changes: writeTxn.Changes(), Index: 10, MsgType: structs.ServiceRegistrationUpsertRequestType}
	receivedChange := eventsFromChanges(writeTxn, registerChange)

	// Check the event, and it's payload are what we are expecting.
	must.Len(t, 1, receivedChange.Events)
	must.Eq(t, structs.TopicService, receivedChange.Events[0].Topic)
	must.Eq(t, structs.TypeServiceRegistration, receivedChange.Events[0].Type)
	must.Eq(t, uint64(10), receivedChange.Events[0].Index)

	eventPayload := receivedChange.Events[0].Payload.(*structs.ServiceRegistrationStreamEvent)
	must.Eq(t, service, eventPayload.Service)

	// Delete the previously upserted service registration.
	deleteTxn := testState.db.WriteTxn(20)
	must.NoError(t, testState.deleteServiceRegistrationByIDTxn(uint64(20), deleteTxn, service.Namespace, service.ID))
	writeTxn.Txn.Commit()

	// Pull the events from the stream.
	deregisterChange := Changes{Changes: deleteTxn.Changes(), Index: 20, MsgType: structs.ServiceRegistrationDeleteByIDRequestType}
	receivedDeleteChange := eventsFromChanges(deleteTxn, deregisterChange)

	// Check the event, and it's payload are what we are expecting.
	must.Len(t, 1, receivedDeleteChange.Events)
	must.Eq(t, structs.TopicService, receivedDeleteChange.Events[0].Topic)
	must.Eq(t, structs.TypeServiceDeregistration, receivedDeleteChange.Events[0].Type)
	must.Eq(t, uint64(20), receivedDeleteChange.Events[0].Index)

	eventPayload = receivedChange.Events[0].Payload.(*structs.ServiceRegistrationStreamEvent)
	must.Eq(t, service, eventPayload.Service)
}

func Test_eventsFromChanges_ACLRole(t *testing.T) {
	ci.Parallel(t)
	testState := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer testState.StopEventBroker()

	// Generate a test ACL role.
	aclRole := mock.ACLRole()

	// Upsert the role into state, skipping the checks perform to ensure the
	// linked policies exist.
	writeTxn := testState.db.WriteTxn(10)
	updated, err := testState.upsertACLRoleTxn(10, writeTxn, aclRole, true)
	must.True(t, updated)
	must.NoError(t, err)
	writeTxn.Txn.Commit()

	// Pull the events from the stream.
	upsertChange := Changes{Changes: writeTxn.Changes(), Index: 10, MsgType: structs.ACLRolesUpsertRequestType}
	receivedChange := eventsFromChanges(writeTxn, upsertChange)

	// Check the event, and it's payload are what we are expecting.
	must.Len(t, 1, receivedChange.Events)
	must.Eq(t, structs.TopicACLRole, receivedChange.Events[0].Topic)
	must.Eq(t, aclRole.ID, receivedChange.Events[0].Key)
	must.Eq(t, aclRole.Name, receivedChange.Events[0].FilterKeys[0])
	must.Eq(t, structs.TypeACLRoleUpserted, receivedChange.Events[0].Type)
	must.Eq(t, uint64(10), receivedChange.Events[0].Index)

	eventPayload := receivedChange.Events[0].Payload.(*structs.ACLRoleStreamEvent)
	must.Eq(t, aclRole, eventPayload.ACLRole)

	// Delete the previously upserted ACL role.
	deleteTxn := testState.db.WriteTxn(20)
	must.NoError(t, testState.deleteACLRoleByIDTxn(deleteTxn, aclRole.ID))
	must.NoError(t, deleteTxn.Insert(tableIndex, &IndexEntry{TableACLRoles, 20}))
	deleteTxn.Txn.Commit()

	// Pull the events from the stream.
	deleteChange := Changes{Changes: deleteTxn.Changes(), Index: 20, MsgType: structs.ACLRolesDeleteByIDRequestType}
	receivedDeleteChange := eventsFromChanges(deleteTxn, deleteChange)

	// Check the event, and it's payload are what we are expecting.
	must.Len(t, 1, receivedDeleteChange.Events)
	must.Eq(t, structs.TopicACLRole, receivedDeleteChange.Events[0].Topic)
	must.Eq(t, aclRole.ID, receivedDeleteChange.Events[0].Key)
	must.Eq(t, aclRole.Name, receivedDeleteChange.Events[0].FilterKeys[0])
	must.Eq(t, structs.TypeACLRoleDeleted, receivedDeleteChange.Events[0].Type)
	must.Eq(t, uint64(20), receivedDeleteChange.Events[0].Index)

	eventPayload = receivedChange.Events[0].Payload.(*structs.ACLRoleStreamEvent)
	must.Eq(t, aclRole, eventPayload.ACLRole)
}

func Test_eventsFromChanges_ACLAuthMethod(t *testing.T) {
	ci.Parallel(t)
	testState := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer testState.StopEventBroker()

	// Generate a test ACL auth method
	authMethod := mock.ACLOIDCAuthMethod()

	// Upsert the auth method straight into state
	writeTxn := testState.db.WriteTxn(10)
	updated, err := testState.upsertACLAuthMethodTxn(10, writeTxn, authMethod)
	must.True(t, updated)
	must.NoError(t, err)
	writeTxn.Txn.Commit()

	// Pull the events from the stream.
	upsertChange := Changes{Changes: writeTxn.Changes(), Index: 10, MsgType: structs.ACLAuthMethodsUpsertRequestType}
	receivedChange := eventsFromChanges(writeTxn, upsertChange)
	must.NotNil(t, receivedChange)

	// Check the event, and its payload are what we are expecting.
	must.Len(t, 1, receivedChange.Events)
	must.Eq(t, structs.TopicACLAuthMethod, receivedChange.Events[0].Topic)
	must.Eq(t, authMethod.Name, receivedChange.Events[0].Key)
	must.Eq(t, structs.TypeACLAuthMethodUpserted, receivedChange.Events[0].Type)
	must.Eq(t, uint64(10), receivedChange.Events[0].Index)

	eventPayload := receivedChange.Events[0].Payload.(*structs.ACLAuthMethodEvent)
	must.Eq(t, authMethod, eventPayload.AuthMethod)

	// Delete the previously upserted auth method
	deleteTxn := testState.db.WriteTxn(20)
	must.NoError(t, testState.deleteACLAuthMethodTxn(deleteTxn, authMethod.Name))
	must.NoError(t, deleteTxn.Insert(tableIndex, &IndexEntry{TableACLAuthMethods, 20}))
	deleteTxn.Txn.Commit()

	// Pull the events from the stream.
	deleteChange := Changes{Changes: deleteTxn.Changes(), Index: 20, MsgType: structs.ACLAuthMethodsDeleteRequestType}
	receivedDeleteChange := eventsFromChanges(deleteTxn, deleteChange)
	must.NotNil(t, receivedDeleteChange)

	// Check the event, and its payload are what we are expecting.
	must.Len(t, 1, receivedDeleteChange.Events)
	must.Eq(t, structs.TopicACLAuthMethod, receivedDeleteChange.Events[0].Topic)
	must.Eq(t, authMethod.Name, receivedDeleteChange.Events[0].Key)
	must.Eq(t, structs.TypeACLAuthMethodDeleted, receivedDeleteChange.Events[0].Type)
	must.Eq(t, uint64(20), receivedDeleteChange.Events[0].Index)

	eventPayload = receivedChange.Events[0].Payload.(*structs.ACLAuthMethodEvent)
	must.Eq(t, authMethod, eventPayload.AuthMethod)
}

func Test_eventsFromChanges_ACLBindingRule(t *testing.T) {
	ci.Parallel(t)
	testState := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer testState.StopEventBroker()

	// Generate a test ACL binding rule.
	bindingRule := mock.ACLBindingRule()

	// Upsert the binding rule straight into state.
	writeTxn := testState.db.WriteTxn(10)
	updated, err := testState.upsertACLBindingRuleTxn(10, writeTxn, bindingRule, true)
	must.True(t, updated)
	must.NoError(t, err)
	writeTxn.Txn.Commit()

	// Pull the events from the stream.
	upsertChange := Changes{Changes: writeTxn.Changes(), Index: 10, MsgType: structs.ACLBindingRulesUpsertRequestType}
	receivedChange := eventsFromChanges(writeTxn, upsertChange)
	must.NotNil(t, receivedChange)

	// Check the event, and its payload are what we are expecting.
	must.Len(t, 1, receivedChange.Events)
	must.Eq(t, structs.TopicACLBindingRule, receivedChange.Events[0].Topic)
	must.Eq(t, bindingRule.ID, receivedChange.Events[0].Key)
	must.SliceContainsAll(t, []string{bindingRule.AuthMethod}, receivedChange.Events[0].FilterKeys)
	must.Eq(t, structs.TypeACLBindingRuleUpserted, receivedChange.Events[0].Type)
	must.Eq(t, 10, receivedChange.Events[0].Index)

	must.Eq(t, bindingRule, receivedChange.Events[0].Payload.(*structs.ACLBindingRuleEvent).ACLBindingRule)

	// Delete the previously upserted binding rule.
	deleteTxn := testState.db.WriteTxn(20)
	must.NoError(t, testState.deleteACLBindingRuleTxn(deleteTxn, bindingRule.ID))
	must.NoError(t, deleteTxn.Insert(tableIndex, &IndexEntry{TableACLBindingRules, 20}))
	deleteTxn.Txn.Commit()

	// Pull the events from the stream.
	deleteChange := Changes{Changes: deleteTxn.Changes(), Index: 20, MsgType: structs.ACLBindingRulesDeleteRequestType}
	receivedDeleteChange := eventsFromChanges(deleteTxn, deleteChange)
	must.NotNil(t, receivedDeleteChange)

	// Check the event, and its payload are what we are expecting.
	must.Len(t, 1, receivedDeleteChange.Events)
	must.Eq(t, structs.TopicACLBindingRule, receivedDeleteChange.Events[0].Topic)
	must.Eq(t, bindingRule.ID, receivedDeleteChange.Events[0].Key)
	must.SliceContainsAll(t, []string{bindingRule.AuthMethod}, receivedDeleteChange.Events[0].FilterKeys)
	must.Eq(t, structs.TypeACLBindingRuleDeleted, receivedDeleteChange.Events[0].Type)
	must.Eq(t, uint64(20), receivedDeleteChange.Events[0].Index)

	must.Eq(t, bindingRule, receivedDeleteChange.Events[0].Payload.(*structs.ACLBindingRuleEvent).ACLBindingRule)
}

func TestEvents_HostVolumes(t *testing.T) {
	ci.Parallel(t)
	store := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer store.StopEventBroker()

	index, err := store.LatestIndex()
	must.NoError(t, err)

	node := mock.Node()
	index++
	must.NoError(t, store.UpsertNode(structs.NodeRegisterRequestType, index, node, NodeUpsertWithNodePool))

	vol := mock.HostVolume()
	vol.NodeID = node.ID
	index++
	must.NoError(t, store.UpsertHostVolume(index, vol))

	node = node.Copy()
	node.HostVolumes = map[string]*structs.ClientHostVolumeConfig{vol.Name: {
		Name: vol.Name,
		Path: "/var/nomad/alloc_mounts" + uuid.Generate(),
	}}
	index++
	must.NoError(t, store.UpsertNode(structs.NodeRegisterRequestType, index, node, NodeUpsertWithNodePool))

	index++
	must.NoError(t, store.DeleteHostVolume(index, vol.Namespace, vol.ID))

	events := WaitForEvents(t, store, 0, 5, 1*time.Second)
	must.Len(t, 5, events)
	must.Eq(t, "Node", events[0].Topic)
	must.Eq(t, "NodeRegistration", events[0].Type)
	must.Eq(t, "HostVolume", events[1].Topic)
	must.Eq(t, "HostVolumeRegistered", events[1].Type)
	must.Eq(t, "Node", events[2].Topic)
	must.Eq(t, "NodeRegistration", events[2].Type)
	must.Eq(t, "HostVolume", events[3].Topic)
	must.Eq(t, "NodeRegistration", events[3].Type)
	must.Eq(t, "HostVolume", events[4].Topic)
	must.Eq(t, "HostVolumeDeleted", events[4].Type)
}

func TestEvents_CSIVolumes(t *testing.T) {
	ci.Parallel(t)
	store := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer store.StopEventBroker()

	index, err := store.LatestIndex()
	must.NoError(t, err)

	plugin := mock.CSIPlugin()
	vol := mock.CSIVolume(plugin)

	index++
	must.NoError(t, store.UpsertCSIVolume(index, []*structs.CSIVolume{vol}))

	alloc := mock.Alloc()
	index++
	store.UpsertAllocs(structs.MsgTypeTestSetup, index, []*structs.Allocation{alloc})

	claim := &structs.CSIVolumeClaim{
		AllocationID:   alloc.ID,
		NodeID:         uuid.Generate(),
		Mode:           structs.CSIVolumeClaimGC,
		AccessMode:     structs.CSIVolumeAccessModeSingleNodeWriter,
		AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		State:          structs.CSIVolumeClaimStateReadyToFree,
	}
	index++
	must.NoError(t, store.CSIVolumeClaim(index, time.Now().UnixNano(), vol.Namespace, vol.ID, claim))

	index++
	must.NoError(t, store.CSIVolumeDeregister(index, vol.Namespace, []string{vol.ID}, false))

	events := WaitForEvents(t, store, 0, 3, 1*time.Second)
	must.Len(t, 3, events)
	must.Eq(t, "CSIVolume", events[0].Topic)
	must.Eq(t, "CSIVolumeRegistered", events[0].Type)
	must.Eq(t, "CSIVolume", events[1].Topic)
	must.Eq(t, "CSIVolumeClaim", events[1].Type)
	must.Eq(t, "CSIVolume", events[2].Topic)
	must.Eq(t, "CSIVolumeDeregistered", events[2].Type)

}

func TestEvents_CSIPlugins(t *testing.T) {
	ci.Parallel(t)
	store := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer store.StopEventBroker()

	index, err := store.LatestIndex()
	must.NoError(t, err)

	node := mock.Node()
	plugin := mock.CSIPlugin()

	index++
	must.NoError(t, store.UpsertNode(structs.NodeRegisterRequestType, index, node))

	node = node.Copy()
	node.CSINodePlugins = map[string]*structs.CSIInfo{
		plugin.ID: {
			PluginID:   plugin.ID,
			Healthy:    true,
			UpdateTime: time.Now(),
		},
	}
	index++
	must.NoError(t, store.UpsertNode(structs.NodeRegisterRequestType, index, node))

	events := WaitForEvents(t, store, 0, 3, 1*time.Second)
	must.Len(t, 3, events)
	must.Eq(t, "Node", events[0].Topic)
	must.Eq(t, "NodeRegistration", events[0].Type)
	must.Eq(t, "Node", events[1].Topic)
	must.Eq(t, "NodeRegistration", events[1].Type)
	must.Eq(t, "CSIPlugin", events[2].Topic)
	must.Eq(t, "NodeRegistration", events[2].Type)
}

func requireNodeRegistrationEventEqual(t *testing.T, want, got structs.Event) {
	t.Helper()

	wantPayload := want.Payload.(*structs.NodeStreamEvent)
	gotPayload := got.Payload.(*structs.NodeStreamEvent)

	// Check payload equality for the fields that we can easily control
	must.Eq(t, wantPayload.Node.Status, gotPayload.Node.Status)
	must.Eq(t, wantPayload.Node.ID, gotPayload.Node.ID)
	must.NotEq(t, wantPayload.Node.Events, gotPayload.Node.Events)
}

func requireNodeDeregistrationEventEqual(t *testing.T, want, got structs.Event) {
	t.Helper()

	wantPayload := want.Payload.(*structs.NodeStreamEvent)
	gotPayload := got.Payload.(*structs.NodeStreamEvent)

	must.Eq(t, wantPayload.Node.ID, gotPayload.Node.ID)
	must.NotEq(t, wantPayload.Node.Events, gotPayload.Node.Events)
}

func requireNodeEventCount(t *testing.T, want int, got structs.Event) {
	gotPayload := got.Payload.(*structs.NodeStreamEvent)
	must.Len(t, want, gotPayload.Node.Events)
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
