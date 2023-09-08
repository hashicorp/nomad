// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/raft"
	"github.com/kr/pretty"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/stream"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

type MockSink struct {
	*bytes.Buffer
	cancel bool
}

func (m *MockSink) ID() string {
	return "Mock"
}

func (m *MockSink) Cancel() error {
	m.cancel = true
	return nil
}

func (m *MockSink) Close() error {
	return nil
}

func testStateStore(t *testing.T) *state.StateStore {
	return state.TestStateStore(t)
}

func testFSM(t *testing.T) *nomadFSM {
	broker := testBroker(t, 0)
	dispatcher, _ := testPeriodicDispatcher(t)
	logger := testlog.HCLogger(t)
	fsmConfig := &FSMConfig{
		EvalBroker:         broker,
		Periodic:           dispatcher,
		Blocked:            NewBlockedEvals(broker, logger),
		Logger:             logger,
		Region:             "global",
		EnableEventBroker:  true,
		EventBufferSize:    100,
		JobTrackedVersions: structs.JobDefaultTrackedVersions,
	}
	fsm, err := NewFSM(fsmConfig)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if fsm == nil {
		t.Fatalf("missing fsm")
	}
	return fsm
}

func makeLog(buf []byte) *raft.Log {
	return &raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  buf,
	}
}

func TestFSM_UpsertNodeEvents(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	fsm := testFSM(t)
	state := fsm.State()

	node := mock.Node()

	err := state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	nodeEvent := &structs.NodeEvent{
		Message:   "Heartbeating failed",
		Subsystem: "Heartbeat",
		Timestamp: time.Now(),
	}

	nodeEvents := []*structs.NodeEvent{nodeEvent}
	allEvents := map[string][]*structs.NodeEvent{node.ID: nodeEvents}

	req := structs.EmitNodeEventsRequest{
		NodeEvents:   allEvents,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	buf, err := structs.Encode(structs.UpsertNodeEventsType, req)
	require.Nil(err)

	// the response in this case will be an error
	resp := fsm.Apply(makeLog(buf))
	require.Nil(resp)

	ws := memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	require.Nil(err)

	require.Equal(2, len(out.Events))

	first := out.Events[1]
	require.Equal(uint64(1), first.CreateIndex)
	require.Equal("Heartbeating failed", first.Message)
}

func TestFSM_UpsertNode(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)
	fsm.blockedEvals.SetEnabled(true)

	node := mock.Node()

	// Mark an eval as blocked.
	eval := mock.Eval()
	eval.ClassEligibility = map[string]bool{node.ComputedClass: true}
	fsm.blockedEvals.Block(eval)

	req := structs.NodeRegisterRequest{
		Node: node,
	}
	buf, err := structs.Encode(structs.NodeRegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	ws := memdb.NewWatchSet()
	n, err := fsm.State().NodeByID(ws, req.Node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n == nil {
		t.Fatalf("not found!")
	}
	if n.CreateIndex != 1 {
		t.Fatalf("bad index: %d", node.CreateIndex)
	}

	tt := fsm.TimeTable()
	index := tt.NearestIndex(time.Now().UTC())
	if index != 1 {
		t.Fatalf("bad: %d", index)
	}

	// Verify the eval was unblocked.
	testutil.WaitForResult(func() (bool, error) {
		bStats := fsm.blockedEvals.Stats()
		if bStats.TotalBlocked != 0 {
			return false, fmt.Errorf("bad: %#v", bStats)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

}

func TestFSM_UpsertNode_Canonicalize(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	fsm := testFSM(t)
	fsm.blockedEvals.SetEnabled(true)

	// Setup a node without eligibility, ensure that upsert/canonicalize put it back
	node := mock.Node()
	node.SchedulingEligibility = ""

	req := structs.NodeRegisterRequest{
		Node: node,
	}
	buf, err := structs.Encode(structs.NodeRegisterRequestType, req)
	require.Nil(err)

	require.Nil(fsm.Apply(makeLog(buf)))

	// Verify we are registered
	n, err := fsm.State().NodeByID(nil, req.Node.ID)
	require.Nil(err)
	require.NotNil(n)
	require.EqualValues(1, n.CreateIndex)
	require.Equal(structs.NodeSchedulingEligible, n.SchedulingEligibility)
}

func TestFSM_UpsertNode_Canonicalize_Ineligible(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	fsm := testFSM(t)
	fsm.blockedEvals.SetEnabled(true)

	// Setup a node without eligibility, ensure that upsert/canonicalize put it back
	node := mock.DrainNode()
	node.SchedulingEligibility = ""

	req := structs.NodeRegisterRequest{
		Node: node,
	}
	buf, err := structs.Encode(structs.NodeRegisterRequestType, req)
	require.Nil(err)

	require.Nil(fsm.Apply(makeLog(buf)))

	// Verify we are registered
	n, err := fsm.State().NodeByID(nil, req.Node.ID)
	require.Nil(err)
	require.NotNil(n)
	require.EqualValues(1, n.CreateIndex)
	require.Equal(structs.NodeSchedulingIneligible, n.SchedulingEligibility)
}

func TestFSM_UpsertNode_NodePool(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name       string
		setupReqFn func(*structs.NodeRegisterRequest)
		validateFn func(*testing.T, *structs.Node, *structs.NodePool)
	}{
		{
			name: "node with empty node pool is placed in defualt",
			setupReqFn: func(req *structs.NodeRegisterRequest) {
				req.Node.NodePool = ""
			},
			validateFn: func(t *testing.T, node *structs.Node, pool *structs.NodePool) {
				must.Eq(t, structs.NodePoolDefault, node.NodePool)
				must.Eq(t, 1, pool.ModifyIndex)
			},
		},
		{
			name: "create new node pool with node",
			setupReqFn: func(req *structs.NodeRegisterRequest) {
				req.Node.NodePool = "new"
				req.CreateNodePool = true
			},
			validateFn: func(t *testing.T, node *structs.Node, pool *structs.NodePool) {
				must.NotNil(t, pool)
				must.Eq(t, "new", pool.Name)
				must.Eq(t, pool.Name, node.NodePool)
				must.Eq(t, node.ModifyIndex, pool.CreateIndex)
			},
		},
		{
			name: "don't create new node pool with node",
			setupReqFn: func(req *structs.NodeRegisterRequest) {
				req.Node.NodePool = "new"
				req.CreateNodePool = false
			},
			validateFn: func(t *testing.T, node *structs.Node, pool *structs.NodePool) {
				must.Nil(t, pool)
				must.Eq(t, "new", node.NodePool)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fsm := testFSM(t)

			node := mock.Node()
			req := structs.NodeRegisterRequest{
				Node: node,
			}
			if tc.setupReqFn != nil {
				tc.setupReqFn(&req)
			}
			buf, err := structs.Encode(structs.NodeRegisterRequestType, req)
			must.NoError(t, err)

			resp := fsm.Apply(makeLog(buf))
			must.Nil(t, resp)

			// Snapshot the state.
			s := fsm.State()

			gotNode, err := s.NodeByID(nil, node.ID)
			must.NoError(t, err)

			gotPool, err := s.NodePoolByName(nil, gotNode.NodePool)
			must.NoError(t, err)

			if tc.validateFn != nil {
				tc.validateFn(t, gotNode, gotPool)
			}
		})
	}
}

func TestFSM_DeregisterNode(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	node := mock.Node()
	req := structs.NodeRegisterRequest{
		Node: node,
	}
	buf, err := structs.Encode(structs.NodeRegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	req2 := structs.NodeBatchDeregisterRequest{
		NodeIDs: []string{node.ID},
	}
	buf, err = structs.Encode(structs.NodeBatchDeregisterRequestType, req2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are NOT registered
	ws := memdb.NewWatchSet()
	node, err = fsm.State().NodeByID(ws, req.Node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if node != nil {
		t.Fatalf("node found!")
	}
}

func TestFSM_UpdateNodeStatus(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	fsm := testFSM(t)
	fsm.blockedEvals.SetEnabled(true)

	node := mock.Node()
	req := structs.NodeRegisterRequest{
		Node: node,
	}
	buf, err := structs.Encode(structs.NodeRegisterRequestType, req)
	require.NoError(err)

	resp := fsm.Apply(makeLog(buf))
	require.Nil(resp)

	// Mark an eval as blocked.
	eval := mock.Eval()
	eval.ClassEligibility = map[string]bool{node.ComputedClass: true}
	fsm.blockedEvals.Block(eval)

	event := &structs.NodeEvent{
		Message:   "Node ready foo",
		Subsystem: structs.NodeEventSubsystemCluster,
		Timestamp: time.Now(),
	}
	req2 := structs.NodeUpdateStatusRequest{
		NodeID:    node.ID,
		Status:    structs.NodeStatusReady,
		NodeEvent: event,
	}
	buf, err = structs.Encode(structs.NodeUpdateStatusRequestType, req2)
	require.NoError(err)

	resp = fsm.Apply(makeLog(buf))
	require.Nil(resp)

	// Verify the status is ready.
	ws := memdb.NewWatchSet()
	node, err = fsm.State().NodeByID(ws, req.Node.ID)
	require.NoError(err)
	require.Equal(structs.NodeStatusReady, node.Status)
	require.Len(node.Events, 2)
	require.Equal(event.Message, node.Events[1].Message)

	// Verify the eval was unblocked.
	testutil.WaitForResult(func() (bool, error) {
		bStats := fsm.blockedEvals.Stats()
		if bStats.TotalBlocked != 0 {
			return false, fmt.Errorf("bad: %#v", bStats)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})
}

func TestFSM_BatchUpdateNodeDrain(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	fsm := testFSM(t)

	node := mock.Node()
	req := structs.NodeRegisterRequest{
		Node: node,
	}
	buf, err := structs.Encode(structs.NodeRegisterRequestType, req)
	require.Nil(err)

	resp := fsm.Apply(makeLog(buf))
	require.Nil(resp)

	strategy := &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{
			Deadline: 10 * time.Second,
		},
	}
	event := &structs.NodeEvent{
		Message:   "Drain strategy enabled",
		Subsystem: structs.NodeEventSubsystemDrain,
		Timestamp: time.Now(),
	}
	req2 := structs.BatchNodeUpdateDrainRequest{
		Updates: map[string]*structs.DrainUpdate{
			node.ID: {
				DrainStrategy: strategy,
			},
		},
		NodeEvents: map[string]*structs.NodeEvent{
			node.ID: event,
		},
	}
	buf, err = structs.Encode(structs.BatchNodeUpdateDrainRequestType, req2)
	require.Nil(err)

	resp = fsm.Apply(makeLog(buf))
	require.Nil(resp)

	// Verify drain is set
	ws := memdb.NewWatchSet()
	node, err = fsm.State().NodeByID(ws, req.Node.ID)
	require.Nil(err)
	require.Equal(node.DrainStrategy, strategy)
	require.Len(node.Events, 2)
}

func TestFSM_UpdateNodeDrain(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	fsm := testFSM(t)

	node := mock.Node()
	req := structs.NodeRegisterRequest{
		Node: node,
	}
	buf, err := structs.Encode(structs.NodeRegisterRequestType, req)
	require.Nil(err)

	resp := fsm.Apply(makeLog(buf))
	require.Nil(resp)

	strategy := &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{
			Deadline: 10 * time.Second,
		},
	}
	req2 := structs.NodeUpdateDrainRequest{
		NodeID:        node.ID,
		DrainStrategy: strategy,
		NodeEvent: &structs.NodeEvent{
			Message:   "Drain strategy enabled",
			Subsystem: structs.NodeEventSubsystemDrain,
			Timestamp: time.Now(),
		},
	}
	buf, err = structs.Encode(structs.NodeUpdateDrainRequestType, req2)
	require.Nil(err)

	resp = fsm.Apply(makeLog(buf))
	require.Nil(resp)

	// Verify we are NOT registered
	ws := memdb.NewWatchSet()
	node, err = fsm.State().NodeByID(ws, req.Node.ID)
	require.Nil(err)
	require.Equal(node.DrainStrategy, strategy)
	require.Len(node.Events, 2)
}

func TestFSM_UpdateNodeEligibility(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	fsm := testFSM(t)

	node := mock.Node()
	req := structs.NodeRegisterRequest{
		Node: node,
	}
	buf, err := structs.Encode(structs.NodeRegisterRequestType, req)
	require.Nil(err)

	resp := fsm.Apply(makeLog(buf))
	require.Nil(resp)

	event := &structs.NodeEvent{
		Message:   "Node marked as ineligible",
		Subsystem: structs.NodeEventSubsystemCluster,
		Timestamp: time.Now(),
	}

	// Set the eligibility
	req2 := structs.NodeUpdateEligibilityRequest{
		NodeID:      node.ID,
		Eligibility: structs.NodeSchedulingIneligible,
		NodeEvent:   event,
	}
	buf, err = structs.Encode(structs.NodeUpdateEligibilityRequestType, req2)
	require.Nil(err)

	resp = fsm.Apply(makeLog(buf))
	require.Nil(resp)

	// Lookup the node and check
	node, err = fsm.State().NodeByID(nil, req.Node.ID)
	require.Nil(err)
	require.Equal(node.SchedulingEligibility, structs.NodeSchedulingIneligible)
	require.Len(node.Events, 2)
	require.Equal(event.Message, node.Events[1].Message)

	// Update the drain
	strategy := &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{
			Deadline: 10 * time.Second,
		},
	}
	req3 := structs.NodeUpdateDrainRequest{
		NodeID:        node.ID,
		DrainStrategy: strategy,
	}
	buf, err = structs.Encode(structs.NodeUpdateDrainRequestType, req3)
	require.Nil(err)
	resp = fsm.Apply(makeLog(buf))
	require.Nil(resp)

	// Try forcing eligibility
	req4 := structs.NodeUpdateEligibilityRequest{
		NodeID:      node.ID,
		Eligibility: structs.NodeSchedulingEligible,
	}
	buf, err = structs.Encode(structs.NodeUpdateEligibilityRequestType, req4)
	require.Nil(err)

	resp = fsm.Apply(makeLog(buf))
	require.NotNil(resp)
	err, ok := resp.(error)
	require.True(ok)
	require.Contains(err.Error(), "draining")
}

func TestFSM_UpdateNodeEligibility_Unblock(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	fsm := testFSM(t)

	node := mock.Node()
	req := structs.NodeRegisterRequest{
		Node: node,
	}
	buf, err := structs.Encode(structs.NodeRegisterRequestType, req)
	require.Nil(err)

	resp := fsm.Apply(makeLog(buf))
	require.Nil(resp)

	// Set the eligibility
	req2 := structs.NodeUpdateEligibilityRequest{
		NodeID:      node.ID,
		Eligibility: structs.NodeSchedulingIneligible,
	}
	buf, err = structs.Encode(structs.NodeUpdateEligibilityRequestType, req2)
	require.Nil(err)

	resp = fsm.Apply(makeLog(buf))
	require.Nil(resp)

	// Mark an eval as blocked.
	eval := mock.Eval()
	eval.ClassEligibility = map[string]bool{node.ComputedClass: true}
	fsm.blockedEvals.Block(eval)

	// Set eligible
	req4 := structs.NodeUpdateEligibilityRequest{
		NodeID:      node.ID,
		Eligibility: structs.NodeSchedulingEligible,
	}
	buf, err = structs.Encode(structs.NodeUpdateEligibilityRequestType, req4)
	require.Nil(err)

	resp = fsm.Apply(makeLog(buf))
	require.Nil(resp)

	// Verify the eval was unblocked.
	testutil.WaitForResult(func() (bool, error) {
		bStats := fsm.blockedEvals.Stats()
		if bStats.TotalBlocked != 0 {
			return false, fmt.Errorf("bad: %#v", bStats)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})
}

func TestFSM_NodePoolDelete(t *testing.T) {
	ci.Parallel(t)

	// Create FSM and populate state.
	fsm := testFSM(t)
	pools := []*structs.NodePool{
		mock.NodePool(),
		mock.NodePool(),
		mock.NodePool(),
		mock.NodePool(),
	}
	err := fsm.State().UpsertNodePools(structs.MsgTypeTestSetup, 1000, pools)
	must.NoError(t, err)

	// Delete some of the node pools.
	req := structs.NodePoolDeleteRequest{
		Names: []string{pools[0].Name, pools[1].Name},
	}
	buf, err := structs.Encode(structs.NodePoolDeleteRequestType, req)
	must.NoError(t, err)

	resp := fsm.Apply(makeLog(buf))
	must.Nil(t, resp)

	// Verify selected node pools were deleted.
	ws := memdb.NewWatchSet()
	for i, pool := range pools {
		got, err := fsm.State().NodePoolByName(ws, pool.Name)
		must.NoError(t, err)

		switch i {
		// Node pools 0 and 1 were deleted.
		case 0, 1:
			must.Nil(t, got)
		default:
			must.NotNil(t, got)
		}
	}
}

func TestFSM_NodePoolUpsert(t *testing.T) {
	ci.Parallel(t)

	// Create FSM and create some node pools.
	fsm := testFSM(t)
	pools := []*structs.NodePool{
		mock.NodePool(),
		mock.NodePool(),
		mock.NodePool(),
	}
	req := structs.NodePoolUpsertRequest{
		NodePools: pools,
	}
	buf, err := structs.Encode(structs.NodePoolUpsertRequestType, req)
	must.NoError(t, err)

	resp := fsm.Apply(makeLog(buf))
	must.Nil(t, resp)

	// Verify node pools were created.
	ws := memdb.NewWatchSet()
	for _, pool := range pools {
		got, err := fsm.State().NodePoolByName(ws, pool.Name)

		must.NoError(t, err)
		must.Eq(t, pool, got, must.Cmp(cmpopts.IgnoreFields(
			structs.NodePool{},
			"CreateIndex",
			"ModifyIndex",
		)))
	}

	// Update one of the node pools.
	updatedPool := pools[0].Copy()
	updatedPool.Description = "updated"
	updatedPool.Meta = map[string]string{
		"update": "true",
	}

	req = structs.NodePoolUpsertRequest{
		NodePools: []*structs.NodePool{updatedPool},
	}
	buf, err = structs.Encode(structs.NodePoolUpsertRequestType, req)
	must.NoError(t, err)

	resp = fsm.Apply(makeLog(buf))
	must.Nil(t, resp)

	// Verify node pool was updated.
	ws = memdb.NewWatchSet()
	got, err := fsm.State().NodePoolByName(ws, updatedPool.Name)
	must.NoError(t, err)
	must.Eq(t, updatedPool, got, must.Cmp(cmpopts.IgnoreFields(
		structs.NodePool{},
		"CreateIndex",
		"ModifyIndex",
	)))
}

func TestFSM_RegisterJob(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	job := mock.PeriodicJob()
	req := structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Namespace: job.Namespace,
		},
	}
	buf, err := structs.Encode(structs.JobRegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	ws := memdb.NewWatchSet()
	jobOut, err := fsm.State().JobByID(ws, req.Namespace, req.Job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if jobOut == nil {
		t.Fatalf("not found!")
	}
	if jobOut.CreateIndex != 1 {
		t.Fatalf("bad index: %d", jobOut.CreateIndex)
	}

	// Verify it was added to the periodic runner.
	tuple := structs.NamespacedID{
		ID:        job.ID,
		Namespace: job.Namespace,
	}
	if _, ok := fsm.periodicDispatcher.tracked[tuple]; !ok {
		t.Fatal("job not added to periodic runner")
	}

	// Verify the launch time was tracked.
	launchOut, err := fsm.State().PeriodicLaunchByID(ws, req.Namespace, req.Job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if launchOut == nil {
		t.Fatalf("not found!")
	}
	if launchOut.Launch.IsZero() {
		t.Fatalf("bad launch time: %v", launchOut.Launch)
	}
}

func TestFSM_RegisterPeriodicJob_NonLeader(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	// Disable the dispatcher
	fsm.periodicDispatcher.SetEnabled(false)

	job := mock.PeriodicJob()
	req := structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Namespace: job.Namespace,
		},
	}
	buf, err := structs.Encode(structs.JobRegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	ws := memdb.NewWatchSet()
	jobOut, err := fsm.State().JobByID(ws, req.Namespace, req.Job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if jobOut == nil {
		t.Fatalf("not found!")
	}
	if jobOut.CreateIndex != 1 {
		t.Fatalf("bad index: %d", jobOut.CreateIndex)
	}

	// Verify it wasn't added to the periodic runner.
	tuple := structs.NamespacedID{
		ID:        job.ID,
		Namespace: job.Namespace,
	}
	if _, ok := fsm.periodicDispatcher.tracked[tuple]; ok {
		t.Fatal("job added to periodic runner")
	}

	// Verify the launch time was tracked.
	launchOut, err := fsm.State().PeriodicLaunchByID(ws, req.Namespace, req.Job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if launchOut == nil {
		t.Fatalf("not found!")
	}
	if launchOut.Launch.IsZero() {
		t.Fatalf("bad launch time: %v", launchOut.Launch)
	}
}

func TestFSM_RegisterJob_BadNamespace(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	job := mock.Job()
	job.Namespace = "foo"
	req := structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Namespace: job.Namespace,
		},
	}
	buf, err := structs.Encode(structs.JobRegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp == nil {
		t.Fatalf("no resp: %v", resp)
	}
	err, ok := resp.(error)
	if !ok {
		t.Fatalf("resp not of error type: %T %v", resp, resp)
	}
	if !strings.Contains(err.Error(), "nonexistent namespace") {
		t.Fatalf("bad error: %v", err)
	}

	// Verify we are not registered
	ws := memdb.NewWatchSet()
	jobOut, err := fsm.State().JobByID(ws, req.Namespace, req.Job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if jobOut != nil {
		t.Fatalf("job found!")
	}
}

func TestFSM_DeregisterJob_Error(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	job := mock.Job()

	deregReq := structs.JobDeregisterRequest{
		JobID: job.ID,
		Purge: true,
		WriteRequest: structs.WriteRequest{
			Namespace: job.Namespace,
		},
	}
	buf, err := structs.Encode(structs.JobDeregisterRequestType, deregReq)
	require.NoError(t, err)

	resp := fsm.Apply(makeLog(buf))
	require.NotNil(t, resp)
	respErr, ok := resp.(error)
	require.Truef(t, ok, "expected response to be an error but found: %T", resp)
	require.Error(t, respErr)
}

func TestFSM_DeregisterJob_Purge(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	job := mock.PeriodicJob()
	req := structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Namespace: job.Namespace,
		},
	}
	buf, err := structs.Encode(structs.JobRegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	req2 := structs.JobDeregisterRequest{
		JobID: job.ID,
		Purge: true,
		WriteRequest: structs.WriteRequest{
			Namespace: job.Namespace,
		},
	}
	buf, err = structs.Encode(structs.JobDeregisterRequestType, req2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are NOT registered
	ws := memdb.NewWatchSet()
	jobOut, err := fsm.State().JobByID(ws, req.Namespace, req.Job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if jobOut != nil {
		t.Fatalf("job found!")
	}

	// Verify it was removed from the periodic runner.
	tuple := structs.NamespacedID{
		ID:        job.ID,
		Namespace: job.Namespace,
	}
	if _, ok := fsm.periodicDispatcher.tracked[tuple]; ok {
		t.Fatal("job not removed from periodic runner")
	}

	// Verify it was removed from the periodic launch table.
	launchOut, err := fsm.State().PeriodicLaunchByID(ws, req.Namespace, req.Job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if launchOut != nil {
		t.Fatalf("launch found!")
	}
}

func TestFSM_DeregisterJob_NoPurge(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	job := mock.PeriodicJob()
	req := structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Namespace: job.Namespace,
		},
	}
	buf, err := structs.Encode(structs.JobRegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	req2 := structs.JobDeregisterRequest{
		JobID: job.ID,
		Purge: false,
		WriteRequest: structs.WriteRequest{
			Namespace: job.Namespace,
		},
	}
	buf, err = structs.Encode(structs.JobDeregisterRequestType, req2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are NOT registered
	ws := memdb.NewWatchSet()
	jobOut, err := fsm.State().JobByID(ws, req.Namespace, req.Job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if jobOut == nil {
		t.Fatalf("job not found!")
	}
	if !jobOut.Stop {
		t.Fatalf("job not stopped found!")
	}

	// Verify it was removed from the periodic runner.
	tuple := structs.NamespacedID{
		ID:        job.ID,
		Namespace: job.Namespace,
	}
	if _, ok := fsm.periodicDispatcher.tracked[tuple]; ok {
		t.Fatal("job not removed from periodic runner")
	}

	// Verify it was removed from the periodic launch table.
	launchOut, err := fsm.State().PeriodicLaunchByID(ws, req.Namespace, req.Job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if launchOut == nil {
		t.Fatalf("launch not found!")
	}
}

func TestFSM_BatchDeregisterJob(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	fsm := testFSM(t)

	job := mock.PeriodicJob()
	req := structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Namespace: job.Namespace,
		},
	}
	buf, err := structs.Encode(structs.JobRegisterRequestType, req)
	require.Nil(err)
	resp := fsm.Apply(makeLog(buf))
	require.Nil(resp)

	job2 := mock.Job()
	req2 := structs.JobRegisterRequest{
		Job: job2,
		WriteRequest: structs.WriteRequest{
			Namespace: job2.Namespace,
		},
	}

	buf, err = structs.Encode(structs.JobRegisterRequestType, req2)
	require.Nil(err)
	resp = fsm.Apply(makeLog(buf))
	require.Nil(resp)

	req3 := structs.JobBatchDeregisterRequest{
		Jobs: map[structs.NamespacedID]*structs.JobDeregisterOptions{
			{
				ID:        job.ID,
				Namespace: job.Namespace,
			}: {},
			{
				ID:        job2.ID,
				Namespace: job2.Namespace,
			}: {
				Purge: true,
			},
		},
		WriteRequest: structs.WriteRequest{
			Namespace: job.Namespace,
		},
	}
	buf, err = structs.Encode(structs.JobBatchDeregisterRequestType, req3)
	require.Nil(err)

	resp = fsm.Apply(makeLog(buf))
	require.Nil(resp)

	// Verify we are NOT registered
	ws := memdb.NewWatchSet()
	jobOut, err := fsm.State().JobByID(ws, req.Namespace, req.Job.ID)
	require.Nil(err)
	require.NotNil(jobOut)
	require.True(jobOut.Stop)

	// Verify it was removed from the periodic runner.
	tuple := structs.NamespacedID{
		ID:        job.ID,
		Namespace: job.Namespace,
	}
	require.NotContains(fsm.periodicDispatcher.tracked, tuple)

	// Verify it was not removed from the periodic launch table.
	launchOut, err := fsm.State().PeriodicLaunchByID(ws, job.Namespace, job.ID)
	require.Nil(err)
	require.NotNil(launchOut)

	// Verify the other jbo was purged
	jobOut2, err := fsm.State().JobByID(ws, job2.Namespace, job2.ID)
	require.Nil(err)
	require.Nil(jobOut2)
}

func TestFSM_UpdateEval(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)
	fsm.evalBroker.SetEnabled(true)

	req := structs.EvalUpdateRequest{
		Evals: []*structs.Evaluation{mock.Eval()},
	}
	buf, err := structs.Encode(structs.EvalUpdateRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	ws := memdb.NewWatchSet()
	eval, err := fsm.State().EvalByID(ws, req.Evals[0].ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if eval == nil {
		t.Fatalf("not found!")
	}
	if eval.CreateIndex != 1 {
		t.Fatalf("bad index: %d", eval.CreateIndex)
	}

	// Verify enqueued
	stats := fsm.evalBroker.Stats()
	if stats.TotalReady != 1 {
		t.Fatalf("bad: %#v %#v", stats, eval)
	}
}

func TestFSM_UpdateEval_Blocked(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)
	fsm.evalBroker.SetEnabled(true)
	fsm.blockedEvals.SetEnabled(true)

	// Create a blocked eval.
	eval := mock.Eval()
	eval.Status = structs.EvalStatusBlocked

	req := structs.EvalUpdateRequest{
		Evals: []*structs.Evaluation{eval},
	}
	buf, err := structs.Encode(structs.EvalUpdateRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("not found!")
	}
	if out.CreateIndex != 1 {
		t.Fatalf("bad index: %d", out.CreateIndex)
	}

	// Verify the eval wasn't enqueued
	stats := fsm.evalBroker.Stats()
	if stats.TotalReady != 0 {
		t.Fatalf("bad: %#v %#v", stats, out)
	}

	// Verify the eval was added to the blocked tracker.
	bStats := fsm.blockedEvals.Stats()
	if bStats.TotalBlocked != 1 {
		t.Fatalf("bad: %#v %#v", bStats, out)
	}
}

func TestFSM_UpdateEval_Untrack(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)
	fsm.evalBroker.SetEnabled(true)
	fsm.blockedEvals.SetEnabled(true)

	// Mark an eval as blocked.
	bEval := mock.Eval()
	bEval.ClassEligibility = map[string]bool{"v1:123": true}
	fsm.blockedEvals.Block(bEval)

	// Create a successful eval for the same job
	eval := mock.Eval()
	eval.JobID = bEval.JobID
	eval.Status = structs.EvalStatusComplete

	req := structs.EvalUpdateRequest{
		Evals: []*structs.Evaluation{eval},
	}
	buf, err := structs.Encode(structs.EvalUpdateRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("not found!")
	}
	if out.CreateIndex != 1 {
		t.Fatalf("bad index: %d", out.CreateIndex)
	}

	// Verify the eval wasn't enqueued
	stats := fsm.evalBroker.Stats()
	if stats.TotalReady != 0 {
		t.Fatalf("bad: %#v %#v", stats, out)
	}

	// Verify the eval was untracked in the blocked tracker.
	bStats := fsm.blockedEvals.Stats()
	if bStats.TotalBlocked != 0 {
		t.Fatalf("bad: %#v %#v", bStats, out)
	}
}

func TestFSM_UpdateEval_NoUntrack(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)
	fsm.evalBroker.SetEnabled(true)
	fsm.blockedEvals.SetEnabled(true)

	// Mark an eval as blocked.
	bEval := mock.Eval()
	bEval.ClassEligibility = map[string]bool{"v1:123": true}
	fsm.blockedEvals.Block(bEval)

	// Create a successful eval for the same job but with placement failures
	eval := mock.Eval()
	eval.JobID = bEval.JobID
	eval.Status = structs.EvalStatusComplete
	eval.FailedTGAllocs = make(map[string]*structs.AllocMetric)
	eval.FailedTGAllocs["test"] = new(structs.AllocMetric)

	req := structs.EvalUpdateRequest{
		Evals: []*structs.Evaluation{eval},
	}
	buf, err := structs.Encode(structs.EvalUpdateRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("not found!")
	}
	if out.CreateIndex != 1 {
		t.Fatalf("bad index: %d", out.CreateIndex)
	}

	// Verify the eval wasn't enqueued
	stats := fsm.evalBroker.Stats()
	if stats.TotalReady != 0 {
		t.Fatalf("bad: %#v %#v", stats, out)
	}

	// Verify the eval was not untracked in the blocked tracker.
	bStats := fsm.blockedEvals.Stats()
	if bStats.TotalBlocked != 1 {
		t.Fatalf("bad: %#v %#v", bStats, out)
	}
}

func TestFSM_DeleteEval(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	eval := mock.Eval()
	req := structs.EvalUpdateRequest{
		Evals: []*structs.Evaluation{eval},
	}
	buf, err := structs.Encode(structs.EvalUpdateRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	req2 := structs.EvalReapRequest{
		Evals: []string{eval.ID},
	}
	buf, err = structs.Encode(structs.EvalDeleteRequestType, req2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are NOT registered
	ws := memdb.NewWatchSet()
	eval, err = fsm.State().EvalByID(ws, req.Evals[0].ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if eval != nil {
		t.Fatalf("eval found!")
	}
}

func TestFSM_UpdateAllocFromClient_Unblock(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)
	fsm.blockedEvals.SetEnabled(true)
	state := fsm.State()

	node := mock.Node()
	state.UpsertNode(structs.MsgTypeTestSetup, 1, node)

	// Mark an eval as blocked.
	eval := mock.Eval()
	eval.ClassEligibility = map[string]bool{node.ComputedClass: true}
	fsm.blockedEvals.Block(eval)

	bStats := fsm.blockedEvals.Stats()
	if bStats.TotalBlocked != 1 {
		t.Fatalf("bad: %#v", bStats)
	}

	// Create a completed eval
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	alloc2 := mock.Alloc()
	alloc2.NodeID = node.ID
	state.UpsertJobSummary(8, mock.JobSummary(alloc.JobID))
	state.UpsertJobSummary(9, mock.JobSummary(alloc2.JobID))
	state.UpsertAllocs(structs.MsgTypeTestSetup, 10, []*structs.Allocation{alloc, alloc2})

	clientAlloc := new(structs.Allocation)
	*clientAlloc = *alloc
	clientAlloc.ClientStatus = structs.AllocClientStatusComplete
	update2 := &structs.Allocation{
		ID:           alloc2.ID,
		NodeID:       alloc2.NodeID,
		ClientStatus: structs.AllocClientStatusRunning,
	}

	req := structs.AllocUpdateRequest{
		Alloc: []*structs.Allocation{clientAlloc, update2},
	}
	buf, err := structs.Encode(structs.AllocClientUpdateRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are updated
	ws := memdb.NewWatchSet()
	out, err := fsm.State().AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	clientAlloc.CreateIndex = out.CreateIndex
	clientAlloc.ModifyIndex = out.ModifyIndex
	if !reflect.DeepEqual(clientAlloc, out) {
		t.Fatalf("bad: %#v %#v", clientAlloc, out)
	}

	out, err = fsm.State().AllocByID(ws, alloc2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	alloc2.CreateIndex = out.CreateIndex
	alloc2.ModifyIndex = out.ModifyIndex
	alloc2.ClientStatus = structs.AllocClientStatusRunning
	alloc2.TaskStates = nil
	if !reflect.DeepEqual(alloc2, out) {
		t.Fatalf("bad: %#v %#v", alloc2, out)
	}

	// Verify the eval was unblocked.
	testutil.WaitForResult(func() (bool, error) {
		bStats = fsm.blockedEvals.Stats()
		if bStats.TotalBlocked != 0 {
			return false, fmt.Errorf("bad: %#v %#v", bStats, out)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})
}

func TestFSM_UpdateAllocFromClient(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)
	state := fsm.State()
	require := require.New(t)

	alloc := mock.Alloc()
	state.UpsertJobSummary(9, mock.JobSummary(alloc.JobID))
	state.UpsertAllocs(structs.MsgTypeTestSetup, 10, []*structs.Allocation{alloc})

	clientAlloc := new(structs.Allocation)
	*clientAlloc = *alloc
	clientAlloc.ClientStatus = structs.AllocClientStatusFailed

	eval := mock.Eval()
	eval.JobID = alloc.JobID
	eval.TriggeredBy = structs.EvalTriggerRetryFailedAlloc
	eval.Type = alloc.Job.Type

	req := structs.AllocUpdateRequest{
		Alloc: []*structs.Allocation{clientAlloc},
		Evals: []*structs.Evaluation{eval},
	}
	buf, err := structs.Encode(structs.AllocClientUpdateRequestType, req)
	require.Nil(err)

	resp := fsm.Apply(makeLog(buf))
	require.Nil(resp)

	// Verify we are registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().AllocByID(ws, alloc.ID)
	require.Nil(err)
	clientAlloc.CreateIndex = out.CreateIndex
	clientAlloc.ModifyIndex = out.ModifyIndex
	require.Equal(clientAlloc, out)

	// Verify eval was inserted
	ws = memdb.NewWatchSet()
	evals, err := fsm.State().EvalsByJob(ws, eval.Namespace, eval.JobID)
	require.Nil(err)
	require.Equal(1, len(evals))
	res := evals[0]
	eval.CreateIndex = res.CreateIndex
	eval.ModifyIndex = res.ModifyIndex
	require.Equal(eval, res)
}

func TestFSM_UpdateAllocDesiredTransition(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)
	state := fsm.State()
	require := require.New(t)

	alloc := mock.Alloc()
	alloc2 := mock.Alloc()
	alloc2.Job = alloc.Job
	alloc2.JobID = alloc.JobID
	state.UpsertJobSummary(9, mock.JobSummary(alloc.JobID))
	state.UpsertAllocs(structs.MsgTypeTestSetup, 10, []*structs.Allocation{alloc, alloc2})

	t1 := &structs.DesiredTransition{
		Migrate: pointer.Of(true),
	}

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
	req := structs.AllocUpdateDesiredTransitionRequest{
		Allocs: map[string]*structs.DesiredTransition{
			alloc.ID:  t1,
			alloc2.ID: t1,
		},
		Evals: []*structs.Evaluation{eval},
	}
	buf, err := structs.Encode(structs.AllocUpdateDesiredTransitionRequestType, req)
	require.Nil(err)

	resp := fsm.Apply(makeLog(buf))
	require.Nil(resp)

	// Verify we are registered
	ws := memdb.NewWatchSet()
	out1, err := fsm.State().AllocByID(ws, alloc.ID)
	require.Nil(err)
	out2, err := fsm.State().AllocByID(ws, alloc2.ID)
	require.Nil(err)
	evalOut, err := fsm.State().EvalByID(ws, eval.ID)
	require.Nil(err)
	require.NotNil(evalOut)
	require.Equal(eval.ID, evalOut.ID)

	require.NotNil(out1.DesiredTransition.Migrate)
	require.NotNil(out2.DesiredTransition.Migrate)
	require.True(*out1.DesiredTransition.Migrate)
	require.True(*out2.DesiredTransition.Migrate)
}

func TestFSM_UpsertVaultAccessor(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)
	fsm.blockedEvals.SetEnabled(true)

	va := mock.VaultAccessor()
	va2 := mock.VaultAccessor()
	req := structs.VaultAccessorsRequest{
		Accessors: []*structs.VaultAccessor{va, va2},
	}
	buf, err := structs.Encode(structs.VaultAccessorRegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	ws := memdb.NewWatchSet()
	out1, err := fsm.State().VaultAccessor(ws, va.Accessor)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out1 == nil {
		t.Fatalf("not found!")
	}
	if out1.CreateIndex != 1 {
		t.Fatalf("bad index: %d", out1.CreateIndex)
	}
	out2, err := fsm.State().VaultAccessor(ws, va2.Accessor)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out2 == nil {
		t.Fatalf("not found!")
	}
	if out1.CreateIndex != 1 {
		t.Fatalf("bad index: %d", out2.CreateIndex)
	}

	tt := fsm.TimeTable()
	index := tt.NearestIndex(time.Now().UTC())
	if index != 1 {
		t.Fatalf("bad: %d", index)
	}
}

func TestFSM_DeregisterVaultAccessor(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)
	fsm.blockedEvals.SetEnabled(true)

	va := mock.VaultAccessor()
	va2 := mock.VaultAccessor()
	accessors := []*structs.VaultAccessor{va, va2}

	// Insert the accessors
	if err := fsm.State().UpsertVaultAccessor(1000, accessors); err != nil {
		t.Fatalf("bad: %v", err)
	}

	req := structs.VaultAccessorsRequest{
		Accessors: accessors,
	}
	buf, err := structs.Encode(structs.VaultAccessorDeregisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	ws := memdb.NewWatchSet()
	out1, err := fsm.State().VaultAccessor(ws, va.Accessor)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out1 != nil {
		t.Fatalf("not deleted!")
	}

	tt := fsm.TimeTable()
	index := tt.NearestIndex(time.Now().UTC())
	if index != 1 {
		t.Fatalf("bad: %d", index)
	}
}

func TestFSM_UpsertSITokenAccessor(t *testing.T) {
	ci.Parallel(t)
	r := require.New(t)

	fsm := testFSM(t)
	fsm.blockedEvals.SetEnabled(true)

	a1 := mock.SITokenAccessor()
	a2 := mock.SITokenAccessor()
	request := structs.SITokenAccessorsRequest{
		Accessors: []*structs.SITokenAccessor{a1, a2},
	}
	buf, err := structs.Encode(structs.ServiceIdentityAccessorRegisterRequestType, request)
	r.NoError(err)

	response := fsm.Apply(makeLog(buf))
	r.Nil(response)

	// Verify the accessors got registered
	ws := memdb.NewWatchSet()
	result1, err := fsm.State().SITokenAccessor(ws, a1.AccessorID)
	r.NoError(err)
	r.NotNil(result1)
	r.Equal(uint64(1), result1.CreateIndex)

	result2, err := fsm.State().SITokenAccessor(ws, a2.AccessorID)
	r.NoError(err)
	r.NotNil(result2)
	r.Equal(uint64(1), result2.CreateIndex)

	tt := fsm.TimeTable()
	latestIndex := tt.NearestIndex(time.Now())
	r.Equal(uint64(1), latestIndex)
}

func TestFSM_DeregisterSITokenAccessor(t *testing.T) {
	ci.Parallel(t)
	r := require.New(t)

	fsm := testFSM(t)
	fsm.blockedEvals.SetEnabled(true)

	a1 := mock.SITokenAccessor()
	a2 := mock.SITokenAccessor()
	accessors := []*structs.SITokenAccessor{a1, a2}
	var err error

	// Insert the accessors
	err = fsm.State().UpsertSITokenAccessors(1000, accessors)
	r.NoError(err)

	request := structs.SITokenAccessorsRequest{Accessors: accessors}
	buf, err := structs.Encode(structs.ServiceIdentityAccessorDeregisterRequestType, request)
	r.NoError(err)

	response := fsm.Apply(makeLog(buf))
	r.Nil(response)

	ws := memdb.NewWatchSet()

	result1, err := fsm.State().SITokenAccessor(ws, a1.AccessorID)
	r.NoError(err)
	r.Nil(result1) // should have been deleted

	result2, err := fsm.State().SITokenAccessor(ws, a2.AccessorID)
	r.NoError(err)
	r.Nil(result2) // should have been deleted

	tt := fsm.TimeTable()
	latestIndex := tt.NearestIndex(time.Now())
	r.Equal(uint64(1), latestIndex)
}

func TestFSM_ApplyPlanResults(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)
	fsm.evalBroker.SetEnabled(true)
	// Create the request and create a deployment
	alloc := mock.Alloc()
	alloc.Resources = &structs.Resources{} // COMPAT(0.11): Remove in 0.11, used to bypass resource creation in state store
	job := alloc.Job
	alloc.Job = nil

	d := mock.Deployment()
	d.JobID = job.ID
	d.JobModifyIndex = job.ModifyIndex
	d.JobVersion = job.Version

	alloc.DeploymentID = d.ID

	eval := mock.Eval()
	eval.JobID = job.ID
	fsm.State().UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{eval})

	fsm.State().UpsertJobSummary(1, mock.JobSummary(alloc.JobID))

	// set up preempted jobs and allocs
	job1 := mock.Job()
	job2 := mock.Job()

	alloc1 := mock.Alloc()
	alloc1.Job = job1
	alloc1.JobID = job1.ID
	alloc1.PreemptedByAllocation = alloc.ID

	alloc2 := mock.Alloc()
	alloc2.Job = job2
	alloc2.JobID = job2.ID
	alloc2.PreemptedByAllocation = alloc.ID

	fsm.State().UpsertAllocs(structs.MsgTypeTestSetup, 1, []*structs.Allocation{alloc1, alloc2})

	// evals for preempted jobs
	eval1 := mock.Eval()
	eval1.JobID = job1.ID

	eval2 := mock.Eval()
	eval2.JobID = job2.ID

	req := structs.ApplyPlanResultsRequest{
		AllocUpdateRequest: structs.AllocUpdateRequest{
			Job:   job,
			Alloc: []*structs.Allocation{alloc},
		},
		Deployment:      d,
		EvalID:          eval.ID,
		NodePreemptions: []*structs.Allocation{alloc1, alloc2},
		PreemptionEvals: []*structs.Evaluation{eval1, eval2},
	}
	buf, err := structs.Encode(structs.ApplyPlanResultsRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify the allocation is registered
	ws := memdb.NewWatchSet()
	assert := assert.New(t)
	out, err := fsm.State().AllocByID(ws, alloc.ID)
	assert.Nil(err)
	alloc.CreateIndex = out.CreateIndex
	alloc.ModifyIndex = out.ModifyIndex
	alloc.AllocModifyIndex = out.AllocModifyIndex

	// Job should be re-attached
	alloc.Job = job
	assert.Equal(alloc, out)

	// Verify that evals for preempted jobs have been created
	e1, err := fsm.State().EvalByID(ws, eval1.ID)
	require := require.New(t)
	require.Nil(err)
	require.NotNil(e1)

	e2, err := fsm.State().EvalByID(ws, eval2.ID)
	require.Nil(err)
	require.NotNil(e2)

	// Verify that eval broker has both evals
	_, ok := fsm.evalBroker.evals[e1.ID]
	require.True(ok)

	_, ok = fsm.evalBroker.evals[e1.ID]
	require.True(ok)

	dout, err := fsm.State().DeploymentByID(ws, d.ID)
	assert.Nil(err)
	tg, ok := dout.TaskGroups[alloc.TaskGroup]
	assert.True(ok)
	assert.NotNil(tg)
	assert.Equal(1, tg.PlacedAllocs)

	// Ensure that the original job is used
	evictAlloc := alloc.Copy()
	job = mock.Job()
	job.Priority = 123
	eval = mock.Eval()
	eval.JobID = job.ID

	fsm.State().UpsertEvals(structs.MsgTypeTestSetup, 2, []*structs.Evaluation{eval})

	evictAlloc.Job = nil
	evictAlloc.DesiredStatus = structs.AllocDesiredStatusEvict
	req2 := structs.ApplyPlanResultsRequest{
		AllocUpdateRequest: structs.AllocUpdateRequest{
			Job:   job,
			Alloc: []*structs.Allocation{evictAlloc},
		},
		EvalID: eval.ID,
	}
	buf, err = structs.Encode(structs.ApplyPlanResultsRequestType, req2)
	assert.Nil(err)

	log := makeLog(buf)
	//set the index to something other than 1
	log.Index = 25
	resp = fsm.Apply(log)
	assert.Nil(resp)

	// Verify we are evicted
	out, err = fsm.State().AllocByID(ws, alloc.ID)
	assert.Nil(err)
	assert.Equal(structs.AllocDesiredStatusEvict, out.DesiredStatus)
	assert.NotNil(out.Job)
	assert.NotEqual(123, out.Job.Priority)

	evalOut, err := fsm.State().EvalByID(ws, eval.ID)
	assert.Nil(err)
	assert.Equal(log.Index, evalOut.ModifyIndex)

}

func TestFSM_DeploymentStatusUpdate(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)
	fsm.evalBroker.SetEnabled(true)
	state := fsm.State()

	// Upsert a deployment
	d := mock.Deployment()
	if err := state.UpsertDeployment(1, d); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Create a request to update the deployment, create an eval and job
	e := mock.Eval()
	j := mock.Job()
	status, desc := structs.DeploymentStatusFailed, "foo"
	req := &structs.DeploymentStatusUpdateRequest{
		DeploymentUpdate: &structs.DeploymentStatusUpdate{
			DeploymentID:      d.ID,
			Status:            status,
			StatusDescription: desc,
		},
		Job:  j,
		Eval: e,
	}
	buf, err := structs.Encode(structs.DeploymentStatusUpdateRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Check that the status was updated properly
	ws := memdb.NewWatchSet()
	dout, err := state.DeploymentByID(ws, d.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if dout.Status != status || dout.StatusDescription != desc {
		t.Fatalf("bad: %#v", dout)
	}

	// Check that the evaluation was created
	eout, _ := state.EvalByID(ws, e.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if eout == nil {
		t.Fatalf("bad: %#v", eout)
	}

	// Check that the job was created
	jout, _ := state.JobByID(ws, j.Namespace, j.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if jout == nil {
		t.Fatalf("bad: %#v", jout)
	}

	// Assert the eval was enqueued
	stats := fsm.evalBroker.Stats()
	if stats.TotalReady != 1 {
		t.Fatalf("bad: %#v %#v", stats, e)
	}
}

func TestFSM_JobStabilityUpdate(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)
	fsm.evalBroker.SetEnabled(true)
	state := fsm.State()

	// Upsert a deployment
	job := mock.Job()
	if err := state.UpsertJob(structs.MsgTypeTestSetup, 1, nil, job); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Create a request to update the job to stable
	req := &structs.JobStabilityRequest{
		JobID:      job.ID,
		JobVersion: job.Version,
		Stable:     true,
		WriteRequest: structs.WriteRequest{
			Namespace: job.Namespace,
		},
	}
	buf, err := structs.Encode(structs.JobStabilityRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Check that the stability was updated properly
	ws := memdb.NewWatchSet()
	jout, _ := state.JobByIDAndVersion(ws, job.Namespace, job.ID, job.Version)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if jout == nil || !jout.Stable {
		t.Fatalf("bad: %#v", jout)
	}
}

func TestFSM_DeploymentPromotion(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)
	fsm.evalBroker.SetEnabled(true)
	state := fsm.State()

	// Create a job with two task groups
	j := mock.Job()
	tg1 := j.TaskGroups[0]
	tg2 := tg1.Copy()
	tg2.Name = "foo"
	j.TaskGroups = append(j.TaskGroups, tg2)
	if err := state.UpsertJob(structs.MsgTypeTestSetup, 1, nil, j); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Create a deployment
	d := mock.Deployment()
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
	if err := state.UpsertDeployment(2, d); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Create a set of allocations
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

	if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 3, []*structs.Allocation{c1, c2}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create an eval
	e := mock.Eval()

	// Promote the canaries
	req := &structs.ApplyDeploymentPromoteRequest{
		DeploymentPromoteRequest: structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			All:          true,
		},
		Eval: e,
	}
	buf, err := structs.Encode(structs.DeploymentPromoteRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Check that the status per task group was updated properly
	ws := memdb.NewWatchSet()
	dout, err := state.DeploymentByID(ws, d.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if len(dout.TaskGroups) != 2 {
		t.Fatalf("bad: %#v", dout.TaskGroups)
	}
	for tg, state := range dout.TaskGroups {
		if !state.Promoted {
			t.Fatalf("bad: group %q not promoted %#v", tg, state)
		}
	}

	// Check that the evaluation was created
	eout, _ := state.EvalByID(ws, e.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if eout == nil {
		t.Fatalf("bad: %#v", eout)
	}

	// Assert the eval was enqueued
	stats := fsm.evalBroker.Stats()
	if stats.TotalReady != 1 {
		t.Fatalf("bad: %#v %#v", stats, e)
	}
}

func TestFSM_DeploymentAllocHealth(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)
	fsm.evalBroker.SetEnabled(true)
	state := fsm.State()

	// Insert a deployment
	d := mock.Deployment()
	if err := state.UpsertDeployment(1, d); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Insert two allocations
	a1 := mock.Alloc()
	a1.DeploymentID = d.ID
	a2 := mock.Alloc()
	a2.DeploymentID = d.ID
	if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 2, []*structs.Allocation{a1, a2}); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Create a job to roll back to
	j := mock.Job()

	// Create an eval that should be upserted
	e := mock.Eval()

	// Create a status update for the deployment
	status, desc := structs.DeploymentStatusFailed, "foo"
	u := &structs.DeploymentStatusUpdate{
		DeploymentID:      d.ID,
		Status:            status,
		StatusDescription: desc,
	}

	// Set health against the deployment
	req := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:           d.ID,
			HealthyAllocationIDs:   []string{a1.ID},
			UnhealthyAllocationIDs: []string{a2.ID},
		},
		Job:              j,
		Eval:             e,
		DeploymentUpdate: u,
	}
	buf, err := structs.Encode(structs.DeploymentAllocHealthRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Check that the status was updated properly
	ws := memdb.NewWatchSet()
	dout, err := state.DeploymentByID(ws, d.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if dout.Status != status || dout.StatusDescription != desc {
		t.Fatalf("bad: %#v", dout)
	}

	// Check that the evaluation was created
	eout, _ := state.EvalByID(ws, e.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if eout == nil {
		t.Fatalf("bad: %#v", eout)
	}

	// Check that the job was created
	jout, _ := state.JobByID(ws, j.Namespace, j.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if jout == nil {
		t.Fatalf("bad: %#v", jout)
	}

	// Check the status of the allocs
	out1, err := state.AllocByID(ws, a1.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	out2, err := state.AllocByID(ws, a2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !out1.DeploymentStatus.IsHealthy() {
		t.Fatalf("bad: alloc %q not healthy", out1.ID)
	}
	if !out2.DeploymentStatus.IsUnhealthy() {
		t.Fatalf("bad: alloc %q not unhealthy", out2.ID)
	}

	// Assert the eval was enqueued
	stats := fsm.evalBroker.Stats()
	if stats.TotalReady != 1 {
		t.Fatalf("bad: %#v %#v", stats, e)
	}
}

func TestFSM_DeleteDeployment(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)
	state := fsm.State()

	// Upsert a deployments
	d := mock.Deployment()
	if err := state.UpsertDeployment(1, d); err != nil {
		t.Fatalf("bad: %v", err)
	}

	req := structs.DeploymentDeleteRequest{
		Deployments: []string{d.ID},
	}
	buf, err := structs.Encode(structs.DeploymentDeleteRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are NOT registered
	ws := memdb.NewWatchSet()
	deployment, err := state.DeploymentByID(ws, d.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if deployment != nil {
		t.Fatalf("deployment found!")
	}
}

func TestFSM_UpsertACLPolicies(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	policy := mock.ACLPolicy()
	req := structs.ACLPolicyUpsertRequest{
		Policies: []*structs.ACLPolicy{policy},
	}
	buf, err := structs.Encode(structs.ACLPolicyUpsertRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().ACLPolicyByName(ws, policy.Name)
	assert.Nil(t, err)
	assert.NotNil(t, out)
}

func TestFSM_DeleteACLPolicies(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	policy := mock.ACLPolicy()
	err := fsm.State().UpsertACLPolicies(structs.MsgTypeTestSetup, 1000, []*structs.ACLPolicy{policy})
	assert.Nil(t, err)

	req := structs.ACLPolicyDeleteRequest{
		Names: []string{policy.Name},
	}
	buf, err := structs.Encode(structs.ACLPolicyDeleteRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are NOT registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().ACLPolicyByName(ws, policy.Name)
	assert.Nil(t, err)
	assert.Nil(t, out)
}

func TestFSM_BootstrapACLTokens(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	token := mock.ACLToken()
	req := structs.ACLTokenBootstrapRequest{
		Token: token,
	}
	buf, err := structs.Encode(structs.ACLTokenBootstrapRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	out, err := fsm.State().ACLTokenByAccessorID(nil, token.AccessorID)
	assert.Nil(t, err)
	assert.NotNil(t, out)

	// Test with reset
	token2 := mock.ACLToken()
	req = structs.ACLTokenBootstrapRequest{
		Token:      token2,
		ResetIndex: out.CreateIndex,
	}
	buf, err = structs.Encode(structs.ACLTokenBootstrapRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	out2, err := fsm.State().ACLTokenByAccessorID(nil, token2.AccessorID)
	assert.Nil(t, err)
	assert.NotNil(t, out2)
}

func TestFSM_UpsertACLTokens(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	token := mock.ACLToken()
	req := structs.ACLTokenUpsertRequest{
		Tokens: []*structs.ACLToken{token},
	}
	buf, err := structs.Encode(structs.ACLTokenUpsertRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().ACLTokenByAccessorID(ws, token.AccessorID)
	assert.Nil(t, err)
	assert.NotNil(t, out)
}

func TestFSM_DeleteACLTokens(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	token := mock.ACLToken()
	err := fsm.State().UpsertACLTokens(structs.MsgTypeTestSetup, 1000, []*structs.ACLToken{token})
	assert.Nil(t, err)

	req := structs.ACLTokenDeleteRequest{
		AccessorIDs: []string{token.AccessorID},
	}
	buf, err := structs.Encode(structs.ACLTokenDeleteRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are NOT registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().ACLTokenByAccessorID(ws, token.AccessorID)
	assert.Nil(t, err)
	assert.Nil(t, out)
}

func testSnapshotRestore(t *testing.T, fsm *nomadFSM) *nomadFSM {
	// Snapshot
	snap, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer snap.Release()

	// Persist
	buf := bytes.NewBuffer(nil)
	sink := &MockSink{buf, false}
	if err := snap.Persist(sink); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Try to restore on a new FSM
	fsm2 := testFSM(t)
	snap, err = fsm2.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer snap.Release()

	abandonCh := fsm2.State().AbandonCh()

	// Do a restore
	if err := fsm2.Restore(sink); err != nil {
		t.Fatalf("err: %v", err)
	}

	select {
	case <-abandonCh:
	default:
		t.Fatalf("bad")
	}

	return fsm2
}

func TestFSM_SnapshotRestore_Nodes(t *testing.T) {
	ci.Parallel(t)
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	node := mock.Node()
	state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	out, _ := state2.NodeByID(nil, node.ID)
	if !reflect.DeepEqual(node, out) {
		t.Fatalf("bad: \n%#v\n%#v", out, node)
	}
}

func TestFSM_SnapshotRestore_NodePools(t *testing.T) {
	ci.Parallel(t)

	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	pool := mock.NodePool()
	state.UpsertNodePools(structs.MsgTypeTestSetup, 1000, []*structs.NodePool{pool})

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	out, _ := state2.NodePoolByName(nil, pool.Name)
	must.Eq(t, pool, out)
}

func TestFSM_SnapshotRestore_Jobs(t *testing.T) {
	ci.Parallel(t)
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	job1 := mock.Job()
	state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job1)
	job2 := mock.Job()
	state.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, job2)

	// Verify the contents
	ws := memdb.NewWatchSet()
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	out1, _ := state2.JobByID(ws, job1.Namespace, job1.ID)
	out2, _ := state2.JobByID(ws, job2.Namespace, job2.ID)
	if !reflect.DeepEqual(job1, out1) {
		t.Fatalf("bad: \n%#v\n%#v", out1, job1)
	}
	if !reflect.DeepEqual(job2, out2) {
		t.Fatalf("bad: \n%#v\n%#v", out2, job2)
	}
}

func TestFSM_SnapshotRestore_Evals(t *testing.T) {
	ci.Parallel(t)
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	eval1 := mock.Eval()
	state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1})
	eval2 := mock.Eval()
	state.UpsertEvals(structs.MsgTypeTestSetup, 1001, []*structs.Evaluation{eval2})

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	ws := memdb.NewWatchSet()
	out1, _ := state2.EvalByID(ws, eval1.ID)
	out2, _ := state2.EvalByID(ws, eval2.ID)
	if !reflect.DeepEqual(eval1, out1) {
		t.Fatalf("bad: \n%#v\n%#v", out1, eval1)
	}
	if !reflect.DeepEqual(eval2, out2) {
		t.Fatalf("bad: \n%#v\n%#v", out2, eval2)
	}
}

func TestFSM_SnapshotRestore_Allocs(t *testing.T) {
	ci.Parallel(t)
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()
	state.UpsertJobSummary(998, mock.JobSummary(alloc1.JobID))
	state.UpsertJobSummary(999, mock.JobSummary(alloc2.JobID))
	state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc1})
	state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc2})

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	ws := memdb.NewWatchSet()
	out1, _ := state2.AllocByID(ws, alloc1.ID)
	out2, _ := state2.AllocByID(ws, alloc2.ID)
	if !reflect.DeepEqual(alloc1, out1) {
		t.Fatalf("bad: \n%#v\n%#v", out1, alloc1)
	}
	if !reflect.DeepEqual(alloc2, out2) {
		t.Fatalf("bad: \n%#v\n%#v", out2, alloc2)
	}
}

func TestFSM_SnapshotRestore_Allocs_Canonicalize(t *testing.T) {
	ci.Parallel(t)
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	alloc := mock.Alloc()

	// remove old versions to force migration path
	alloc.AllocatedResources = nil

	state.UpsertJobSummary(998, mock.JobSummary(alloc.JobID))
	state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc})

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	ws := memdb.NewWatchSet()
	out, err := state2.AllocByID(ws, alloc.ID)
	require.NoError(t, err)

	require.NotNil(t, out.AllocatedResources)
	require.Contains(t, out.AllocatedResources.Tasks, "web")

	alloc.Canonicalize()
	require.Equal(t, alloc, out)
}

func TestFSM_SnapshotRestore_Indexes(t *testing.T) {
	ci.Parallel(t)
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	node1 := mock.Node()
	state.UpsertNode(structs.MsgTypeTestSetup, 1000, node1)

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()

	index, err := state2.Index("nodes")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}
}

func TestFSM_SnapshotRestore_TimeTable(t *testing.T) {
	ci.Parallel(t)
	// Add some state
	fsm := testFSM(t)

	tt := fsm.TimeTable()
	start := time.Now().UTC()
	tt.Witness(1000, start)
	tt.Witness(2000, start.Add(10*time.Minute))

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)

	tt2 := fsm2.TimeTable()
	if tt2.NearestTime(1500) != start {
		t.Fatalf("bad")
	}
	if tt2.NearestIndex(start.Add(15*time.Minute)) != 2000 {
		t.Fatalf("bad")
	}
}

func TestFSM_SnapshotRestore_PeriodicLaunches(t *testing.T) {
	ci.Parallel(t)
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	job1 := mock.Job()
	launch1 := &structs.PeriodicLaunch{
		ID:        job1.ID,
		Namespace: job1.Namespace,
		Launch:    time.Now(),
	}
	state.UpsertPeriodicLaunch(1000, launch1)
	job2 := mock.Job()
	launch2 := &structs.PeriodicLaunch{
		ID:        job2.ID,
		Namespace: job2.Namespace,
		Launch:    time.Now(),
	}
	state.UpsertPeriodicLaunch(1001, launch2)

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	ws := memdb.NewWatchSet()
	out1, _ := state2.PeriodicLaunchByID(ws, launch1.Namespace, launch1.ID)
	out2, _ := state2.PeriodicLaunchByID(ws, launch2.Namespace, launch2.ID)

	if !cmp.Equal(launch1, out1) {
		t.Fatalf("bad: %v", cmp.Diff(launch1, out1))
	}
	if !cmp.Equal(launch2, out2) {
		t.Fatalf("bad: %v", cmp.Diff(launch2, out2))
	}
}

func TestFSM_SnapshotRestore_JobSummary(t *testing.T) {
	ci.Parallel(t)
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()

	job1 := mock.Job()
	state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job1)
	ws := memdb.NewWatchSet()
	js1, _ := state.JobSummaryByID(ws, job1.Namespace, job1.ID)

	job2 := mock.Job()
	state.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, job2)
	js2, _ := state.JobSummaryByID(ws, job2.Namespace, job2.ID)

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	out1, _ := state2.JobSummaryByID(ws, job1.Namespace, job1.ID)
	out2, _ := state2.JobSummaryByID(ws, job2.Namespace, job2.ID)
	if !reflect.DeepEqual(js1, out1) {
		t.Fatalf("bad: \n%#v\n%#v", js1, out1)
	}
	if !reflect.DeepEqual(js2, out2) {
		t.Fatalf("bad: \n%#v\n%#v", js2, out2)
	}
}

func TestFSM_SnapshotRestore_VaultAccessors(t *testing.T) {
	ci.Parallel(t)
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	a1 := mock.VaultAccessor()
	a2 := mock.VaultAccessor()
	state.UpsertVaultAccessor(1000, []*structs.VaultAccessor{a1, a2})

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	ws := memdb.NewWatchSet()
	out1, _ := state2.VaultAccessor(ws, a1.Accessor)
	out2, _ := state2.VaultAccessor(ws, a2.Accessor)
	if !reflect.DeepEqual(a1, out1) {
		t.Fatalf("bad: \n%#v\n%#v", out1, a1)
	}
	if !reflect.DeepEqual(a2, out2) {
		t.Fatalf("bad: \n%#v\n%#v", out2, a2)
	}
}

func TestFSM_SnapshotRestore_JobVersions(t *testing.T) {
	ci.Parallel(t)
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	job1 := mock.Job()
	state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job1)
	job2 := mock.Job()
	job2.ID = job1.ID
	state.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, job2)

	// Verify the contents
	ws := memdb.NewWatchSet()
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	out1, _ := state2.JobByIDAndVersion(ws, job1.Namespace, job1.ID, job1.Version)
	out2, _ := state2.JobByIDAndVersion(ws, job2.Namespace, job2.ID, job2.Version)
	if !reflect.DeepEqual(job1, out1) {
		t.Fatalf("bad: \n%#v\n%#v", out1, job1)
	}
	if !reflect.DeepEqual(job2, out2) {
		t.Fatalf("bad: \n%#v\n%#v", out2, job2)
	}
	if job2.Version != 1 {
		t.Fatalf("bad: \n%#v\n%#v", 1, job2)
	}
}

func TestFSM_SnapshotRestore_Deployments(t *testing.T) {
	ci.Parallel(t)
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	d1 := mock.Deployment()
	d2 := mock.Deployment()

	j := mock.Job()
	d1.JobID = j.ID
	d2.JobID = j.ID

	state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, j)
	state.UpsertDeployment(1000, d1)
	state.UpsertDeployment(1001, d2)

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	ws := memdb.NewWatchSet()
	out1, _ := state2.DeploymentByID(ws, d1.ID)
	out2, _ := state2.DeploymentByID(ws, d2.ID)
	if !reflect.DeepEqual(d1, out1) {
		t.Fatalf("bad: \n%#v\n%#v", out1, d1)
	}
	if !reflect.DeepEqual(d2, out2) {
		t.Fatalf("bad: \n%#v\n%#v", out2, d2)
	}
}

func TestFSM_SnapshotRestore_ACLPolicy(t *testing.T) {
	ci.Parallel(t)
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	p1 := mock.ACLPolicy()
	p2 := mock.ACLPolicy()
	state.UpsertACLPolicies(structs.MsgTypeTestSetup, 1000, []*structs.ACLPolicy{p1, p2})

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	ws := memdb.NewWatchSet()
	out1, _ := state2.ACLPolicyByName(ws, p1.Name)
	out2, _ := state2.ACLPolicyByName(ws, p2.Name)
	assert.Equal(t, p1, out1)
	assert.Equal(t, p2, out2)
}

func TestFSM_SnapshotRestore_ACLTokens(t *testing.T) {
	ci.Parallel(t)
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	tk1 := mock.ACLToken()
	tk2 := mock.ACLToken()
	state.UpsertACLTokens(structs.MsgTypeTestSetup, 1000, []*structs.ACLToken{tk1, tk2})

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	ws := memdb.NewWatchSet()
	out1, _ := state2.ACLTokenByAccessorID(ws, tk1.AccessorID)
	out2, _ := state2.ACLTokenByAccessorID(ws, tk2.AccessorID)
	assert.Equal(t, tk1, out1)
	assert.Equal(t, tk2, out2)
}

func TestFSM_SnapshotRestore_SchedulerConfiguration(t *testing.T) {
	ci.Parallel(t)
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	schedConfig := &structs.SchedulerConfiguration{
		SchedulerAlgorithm: "spread",
		PreemptionConfig: structs.PreemptionConfig{
			SystemSchedulerEnabled: true,
		},
	}
	state.SchedulerSetConfig(1000, schedConfig)

	// Verify the contents
	require := require.New(t)
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	index, out, err := state2.SchedulerConfig()
	require.Nil(err)
	require.EqualValues(1000, index)
	require.Equal(schedConfig, out)
}

func TestFSM_SnapshotRestore_ClusterMetadata(t *testing.T) {
	ci.Parallel(t)

	fsm := testFSM(t)
	state := fsm.State()
	clusterID := "12345678-1234-1234-1234-1234567890"
	now := time.Now().UnixNano()
	meta := &structs.ClusterMetadata{ClusterID: clusterID, CreateTime: now}
	state.ClusterSetMetadata(1000, meta)

	// Verify the contents
	require := require.New(t)
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	out, err := state2.ClusterMetadata(memdb.NewWatchSet())
	require.NoError(err)
	require.Equal(clusterID, out.ClusterID)
}

func TestFSM_SnapshotRestore_ServiceRegistrations(t *testing.T) {
	ci.Parallel(t)

	// Create our initial FSM which will be snapshotted.
	fsm := testFSM(t)
	testState := fsm.State()

	// Generate and upsert some service registrations.
	serviceRegs := mock.ServiceRegistrations()
	require.NoError(t, testState.UpsertServiceRegistrations(structs.MsgTypeTestSetup, 10, serviceRegs))

	// Perform a snapshot restore.
	restoredFSM := testSnapshotRestore(t, fsm)
	restoredState := restoredFSM.State()

	// List the service registrations from restored state and ensure everything
	// is as expected.
	iter, err := restoredState.GetServiceRegistrations(memdb.NewWatchSet())
	require.NoError(t, err)

	var restoredRegs []*structs.ServiceRegistration

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		restoredRegs = append(restoredRegs, raw.(*structs.ServiceRegistration))
	}
	require.ElementsMatch(t, restoredRegs, serviceRegs)
}

func TestFSM_SnapshotRestore_ACLRoles(t *testing.T) {
	ci.Parallel(t)

	// Create our initial FSM which will be snapshotted.
	fsm := testFSM(t)
	testState := fsm.State()

	// Create the policies our ACL roles wants to link to.
	policy1 := mock.ACLPolicy()
	policy1.Name = "mocked-test-policy-1"
	policy2 := mock.ACLPolicy()
	policy2.Name = "mocked-test-policy-2"

	require.NoError(t, testState.UpsertACLPolicies(
		structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2}))

	// Generate and upsert some ACL roles.
	aclRoles := []*structs.ACLRole{mock.ACLRole(), mock.ACLRole()}
	require.NoError(t, testState.UpsertACLRoles(structs.MsgTypeTestSetup, 10, aclRoles, false))

	// Perform a snapshot restore.
	restoredFSM := testSnapshotRestore(t, fsm)
	restoredState := restoredFSM.State()

	// List the ACL roles from restored state and ensure everything is as
	// expected.
	iter, err := restoredState.GetACLRoles(memdb.NewWatchSet())
	require.NoError(t, err)

	var restoredACLRoles []*structs.ACLRole

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		restoredACLRoles = append(restoredACLRoles, raw.(*structs.ACLRole))
	}
	require.ElementsMatch(t, restoredACLRoles, aclRoles)
}

func TestFSM_SnapshotRestore_ACLAuthMethods(t *testing.T) {
	ci.Parallel(t)

	// Create our initial FSM which will be snapshotted.
	fsm := testFSM(t)
	testState := fsm.State()

	// Generate and upsert some ACL auth methods.
	authMethods := []*structs.ACLAuthMethod{mock.ACLOIDCAuthMethod(), mock.ACLOIDCAuthMethod()}
	must.NoError(t, testState.UpsertACLAuthMethods(10, authMethods))

	// Perform a snapshot restore.
	restoredFSM := testSnapshotRestore(t, fsm)
	restoredState := restoredFSM.State()

	// List the ACL auth methods from restored state and ensure everything is as
	// expected.
	iter, err := restoredState.GetACLAuthMethods(memdb.NewWatchSet())
	must.NoError(t, err)

	var restoredACLAuthMethods []*structs.ACLAuthMethod
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		restoredACLAuthMethods = append(restoredACLAuthMethods, raw.(*structs.ACLAuthMethod))
	}
	must.SliceContainsAll(t, restoredACLAuthMethods, authMethods)
}

func TestFSM_SnapshotRestore_ACLBindingRules(t *testing.T) {
	ci.Parallel(t)

	// Create our initial FSM which will be snapshotted.
	fsm := testFSM(t)
	testState := fsm.State()

	// Generate a some mocked ACL binding rules for testing and upsert these
	// straight into state.
	mockedACLBindingRoles := []*structs.ACLBindingRule{mock.ACLBindingRule(), mock.ACLBindingRule()}
	must.NoError(t, testState.UpsertACLBindingRules(10, mockedACLBindingRoles, true))

	// Perform a snapshot restore.
	restoredFSM := testSnapshotRestore(t, fsm)
	restoredState := restoredFSM.State()

	// List the ACL binding rules from restored state and ensure everything is
	// as expected.
	iter, err := restoredState.GetACLBindingRules(memdb.NewWatchSet())
	must.NoError(t, err)

	var restoredACLBindingRules []*structs.ACLBindingRule

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		restoredACLBindingRules = append(restoredACLBindingRules, raw.(*structs.ACLBindingRule))
	}
	must.SliceContainsAll(t, restoredACLBindingRules, mockedACLBindingRoles)
}

func TestFSM_ReconcileSummaries(t *testing.T) {
	ci.Parallel(t)
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()

	// Add a node
	node := mock.Node()
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 800, node))

	// Make a job so that none of the tasks can be placed
	job1 := mock.Job()
	job1.TaskGroups[0].Tasks[0].Resources.CPU = 5000
	require.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job1))

	// make a job which can make partial progress
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	require.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1010, nil, alloc.Job))
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1011, []*structs.Allocation{alloc}))

	// Delete the summaries
	require.NoError(t, state.DeleteJobSummary(1030, job1.Namespace, job1.ID))
	require.NoError(t, state.DeleteJobSummary(1040, alloc.Namespace, alloc.Job.ID))

	req := structs.GenericRequest{}
	buf, err := structs.Encode(structs.ReconcileJobSummariesRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	ws := memdb.NewWatchSet()
	out1, err := state.JobSummaryByID(ws, job1.Namespace, job1.ID)
	require.NoError(t, err)

	expected := structs.JobSummary{
		JobID:     job1.ID,
		Namespace: job1.Namespace,
		Summary: map[string]structs.TaskGroupSummary{
			"web": {
				Queued: 10,
			},
		},
		CreateIndex: 1000,
		ModifyIndex: out1.ModifyIndex,
	}
	if !reflect.DeepEqual(&expected, out1) {
		t.Fatalf("expected: %#v, actual: %#v", &expected, out1)
	}

	// This exercises the code path which adds the allocations made by the
	// planner and the number of unplaced allocations in the reconcile summaries
	// codepath
	out2, err := state.JobSummaryByID(ws, alloc.Namespace, alloc.Job.ID)
	require.NoError(t, err)

	expected = structs.JobSummary{
		JobID:     alloc.Job.ID,
		Namespace: alloc.Job.Namespace,
		Summary: map[string]structs.TaskGroupSummary{
			"web": {
				Queued:   9,
				Starting: 1,
			},
		},
		CreateIndex: 1010,
		ModifyIndex: out2.ModifyIndex,
	}
	if !reflect.DeepEqual(&expected, out2) {
		t.Fatalf("Diff % #v", pretty.Diff(&expected, out2))
	}
}

// COMPAT: Remove in 0.11
func TestFSM_ReconcileParentJobSummary(t *testing.T) {
	// This test exercises code to handle https://github.com/hashicorp/nomad/issues/3886
	ci.Parallel(t)

	require := require.New(t)
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()

	// Add a node
	node := mock.Node()
	state.UpsertNode(structs.MsgTypeTestSetup, 800, node)

	// Make a parameterized job
	job1 := mock.BatchJob()
	job1.ID = "test"
	job1.ParameterizedJob = &structs.ParameterizedJobConfig{
		Payload: "random",
	}
	job1.TaskGroups[0].Count = 1
	state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job1)

	// Make a child job
	childJob := job1.Copy()
	childJob.ID = job1.ID + "dispatch-23423423"
	childJob.ParentID = job1.ID
	childJob.Dispatched = true
	childJob.Status = structs.JobStatusRunning

	// Create an alloc for child job
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	alloc.Job = childJob
	alloc.JobID = childJob.ID
	alloc.ClientStatus = structs.AllocClientStatusRunning

	state.UpsertJob(structs.MsgTypeTestSetup, 1010, nil, childJob)
	state.UpsertAllocs(structs.MsgTypeTestSetup, 1011, []*structs.Allocation{alloc})

	// Make the summary incorrect in the state store
	summary, err := state.JobSummaryByID(nil, job1.Namespace, job1.ID)
	require.Nil(err)

	summary.Children = nil
	summary.Summary = make(map[string]structs.TaskGroupSummary)
	summary.Summary["web"] = structs.TaskGroupSummary{
		Queued: 1,
	}

	req := structs.GenericRequest{}
	buf, err := structs.Encode(structs.ReconcileJobSummariesRequestType, req)
	require.Nil(err)

	resp := fsm.Apply(makeLog(buf))
	require.Nil(resp)

	ws := memdb.NewWatchSet()
	out1, _ := state.JobSummaryByID(ws, job1.Namespace, job1.ID)
	expected := structs.JobSummary{
		JobID:       job1.ID,
		Namespace:   job1.Namespace,
		Summary:     make(map[string]structs.TaskGroupSummary),
		CreateIndex: 1000,
		ModifyIndex: out1.ModifyIndex,
		Children: &structs.JobChildrenSummary{
			Running: 1,
		},
	}
	require.Equal(&expected, out1)
}

func TestFSM_LeakedDeployments(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	d := mock.Deployment()
	require.NoError(state.UpsertDeployment(1000, d))

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	out, _ := state2.DeploymentByID(nil, d.ID)
	require.NotNil(out)
	require.Equal(structs.DeploymentStatusCancelled, out.Status)
}

func TestFSM_Autopilot(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	// Set the autopilot config using a request.
	req := structs.AutopilotSetConfigRequest{
		Datacenter: "dc1",
		Config: structs.AutopilotConfig{
			CleanupDeadServers:   true,
			LastContactThreshold: 10 * time.Second,
			MaxTrailingLogs:      300,
			MinQuorum:            3,
		},
	}
	buf, err := structs.Encode(structs.AutopilotRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := fsm.Apply(makeLog(buf))
	if _, ok := resp.(error); ok {
		t.Fatalf("bad: %v", resp)
	}

	// Verify key is set directly in the state store.
	_, config, err := fsm.state.AutopilotConfig()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if config.CleanupDeadServers != req.Config.CleanupDeadServers {
		t.Fatalf("bad: %v", config.CleanupDeadServers)
	}
	if config.LastContactThreshold != req.Config.LastContactThreshold {
		t.Fatalf("bad: %v", config.LastContactThreshold)
	}
	if config.MaxTrailingLogs != req.Config.MaxTrailingLogs {
		t.Fatalf("bad: %v", config.MaxTrailingLogs)
	}
	if config.MinQuorum != req.Config.MinQuorum {
		t.Fatalf("bad: %v", config.MinQuorum)
	}

	// Now use CAS and provide an old index
	req.CAS = true
	req.Config.CleanupDeadServers = false
	req.Config.ModifyIndex = config.ModifyIndex - 1
	buf, err = structs.Encode(structs.AutopilotRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp = fsm.Apply(makeLog(buf))
	if _, ok := resp.(error); ok {
		t.Fatalf("bad: %v", resp)
	}

	_, config, err = fsm.state.AutopilotConfig()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !config.CleanupDeadServers {
		t.Fatalf("bad: %v", config.CleanupDeadServers)
	}
}

func TestFSM_SchedulerConfig(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	require := require.New(t)

	// Set the scheduler config using a request.
	req := structs.SchedulerSetConfigRequest{
		Config: structs.SchedulerConfiguration{
			PreemptionConfig: structs.PreemptionConfig{
				SystemSchedulerEnabled: true,
				BatchSchedulerEnabled:  true,
			},
		},
	}
	buf, err := structs.Encode(structs.SchedulerConfigRequestType, req)
	require.Nil(err)

	resp := fsm.Apply(makeLog(buf))
	if _, ok := resp.(error); ok {
		t.Fatalf("bad: %v", resp)
	}

	// Verify key is set directly in the state store.
	_, config, err := fsm.state.SchedulerConfig()
	require.Nil(err)

	require.Equal(config.PreemptionConfig.SystemSchedulerEnabled, req.Config.PreemptionConfig.SystemSchedulerEnabled)
	require.Equal(config.PreemptionConfig.BatchSchedulerEnabled, req.Config.PreemptionConfig.BatchSchedulerEnabled)

	// Now use CAS and provide an old index
	req.CAS = true
	req.Config.PreemptionConfig = structs.PreemptionConfig{SystemSchedulerEnabled: false, BatchSchedulerEnabled: false}
	req.Config.ModifyIndex = config.ModifyIndex - 1
	buf, err = structs.Encode(structs.SchedulerConfigRequestType, req)
	require.Nil(err)

	resp = fsm.Apply(makeLog(buf))
	if _, ok := resp.(error); ok {
		t.Fatalf("bad: %v", resp)
	}

	_, config, err = fsm.state.SchedulerConfig()
	require.Nil(err)
	// Verify that preemption is still enabled
	require.True(config.PreemptionConfig.SystemSchedulerEnabled)
	require.True(config.PreemptionConfig.BatchSchedulerEnabled)
}

func TestFSM_ClusterMetadata(t *testing.T) {
	ci.Parallel(t)
	r := require.New(t)

	fsm := testFSM(t)
	clusterID := "12345678-1234-1234-1234-1234567890"
	now := time.Now().UnixNano()
	meta := structs.ClusterMetadata{
		ClusterID:  clusterID,
		CreateTime: now,
	}
	buf, err := structs.Encode(structs.ClusterMetadataRequestType, meta)
	r.NoError(err)

	result := fsm.Apply(makeLog(buf))
	r.Nil(result)

	// Verify the clusterID is set directly in the state store
	ws := memdb.NewWatchSet()
	storedMetadata, err := fsm.state.ClusterMetadata(ws)
	r.NoError(err)
	r.Equal(clusterID, storedMetadata.ClusterID)

	// Assert cluster ID cannot be overwritten and is not regenerated
	erroneous := structs.ClusterMetadata{
		ClusterID: "99999999-9999-9999-9999-9999999999",
	}
	buf, err = structs.Encode(structs.ClusterMetadataRequestType, erroneous)
	r.NoError(err)

	result = fsm.Apply(makeLog(buf))
	r.Error(result.(error))

	storedMetadata, err = fsm.state.ClusterMetadata(ws)
	r.NoError(err)
	r.Equal(clusterID, storedMetadata.ClusterID)
	r.Equal(now, storedMetadata.CreateTime)
}

func TestFSM_UpsertNamespaces(t *testing.T) {
	assert := assert.New(t)
	ci.Parallel(t)
	fsm := testFSM(t)

	ns1 := mock.Namespace()
	ns2 := mock.Namespace()
	req := structs.NamespaceUpsertRequest{
		Namespaces: []*structs.Namespace{ns1, ns2},
	}
	buf, err := structs.Encode(structs.NamespaceUpsertRequestType, req)
	assert.Nil(err)
	assert.Nil(fsm.Apply(makeLog(buf)))

	// Verify we are registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().NamespaceByName(ws, ns1.Name)
	assert.Nil(err)
	assert.NotNil(out)

	out, err = fsm.State().NamespaceByName(ws, ns2.Name)
	assert.Nil(err)
	assert.NotNil(out)
}

func TestFSM_DeleteNamespaces(t *testing.T) {
	assert := assert.New(t)
	ci.Parallel(t)
	fsm := testFSM(t)

	ns1 := mock.Namespace()
	ns2 := mock.Namespace()
	assert.Nil(fsm.State().UpsertNamespaces(1000, []*structs.Namespace{ns1, ns2}))

	req := structs.NamespaceDeleteRequest{
		Namespaces: []string{ns1.Name, ns2.Name},
	}
	buf, err := structs.Encode(structs.NamespaceDeleteRequestType, req)
	assert.Nil(err)
	assert.Nil(fsm.Apply(makeLog(buf)))

	// Verify we are NOT registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().NamespaceByName(ws, ns1.Name)
	assert.Nil(err)
	assert.Nil(out)

	out, err = fsm.State().NamespaceByName(ws, ns2.Name)
	assert.Nil(err)
	assert.Nil(out)
}

func TestFSM_SnapshotRestore_Namespaces(t *testing.T) {
	ci.Parallel(t)
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	ns1 := mock.Namespace()
	ns2 := mock.Namespace()
	state.UpsertNamespaces(1000, []*structs.Namespace{ns1, ns2})

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	ws := memdb.NewWatchSet()
	out1, _ := state2.NamespaceByName(ws, ns1.Name)
	out2, _ := state2.NamespaceByName(ws, ns2.Name)
	if !reflect.DeepEqual(ns1, out1) {
		t.Fatalf("bad: \n%#v\n%#v", out1, ns1)
	}
	if !reflect.DeepEqual(ns2, out2) {
		t.Fatalf("bad: \n%#v\n%#v", out2, ns2)
	}
}

func TestFSM_UpsertServiceRegistrations(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	// Generate our test service registrations.
	services := mock.ServiceRegistrations()

	// Build and apply our message.
	req := structs.ServiceRegistrationUpsertRequest{Services: services}
	buf, err := structs.Encode(structs.ServiceRegistrationUpsertRequestType, req)
	assert.Nil(t, err)
	assert.Nil(t, fsm.Apply(makeLog(buf)))

	// Check that both services are found within state.
	ws := memdb.NewWatchSet()
	out, err := fsm.State().GetServiceRegistrationByID(ws, services[0].Namespace, services[0].ID)
	assert.Nil(t, err)
	assert.NotNil(t, out)

	out, err = fsm.State().GetServiceRegistrationByID(ws, services[1].Namespace, services[1].ID)
	assert.Nil(t, err)
	assert.NotNil(t, out)
}

func TestFSM_DeleteServiceRegistrationsByID(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	// Generate our test service registrations.
	services := mock.ServiceRegistrations()

	// Upsert the services.
	assert.NoError(t, fsm.State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, uint64(10), services))

	// Build and apply our message.
	req := structs.ServiceRegistrationDeleteByIDRequest{ID: services[0].ID}
	buf, err := structs.Encode(structs.ServiceRegistrationDeleteByIDRequestType, req)
	assert.Nil(t, err)
	assert.Nil(t, fsm.Apply(makeLog(buf)))

	// Check that the service has been deleted, whilst the other is still
	// available.
	ws := memdb.NewWatchSet()
	out, err := fsm.State().GetServiceRegistrationByID(ws, services[0].Namespace, services[0].ID)
	assert.Nil(t, err)
	assert.Nil(t, out)

	out, err = fsm.State().GetServiceRegistrationByID(ws, services[1].Namespace, services[1].ID)
	assert.Nil(t, err)
	assert.NotNil(t, out)
}

func TestFSM_DeleteServiceRegistrationsByNodeID(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	// Generate our test service registrations. Set them both to have the same
	// node ID.
	services := mock.ServiceRegistrations()
	services[1].NodeID = services[0].NodeID

	// Upsert the services.
	assert.NoError(t, fsm.State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, uint64(10), services))

	// Build and apply our message.
	req := structs.ServiceRegistrationDeleteByNodeIDRequest{NodeID: services[0].NodeID}
	buf, err := structs.Encode(structs.ServiceRegistrationDeleteByNodeIDRequestType, req)
	assert.Nil(t, err)
	assert.Nil(t, fsm.Apply(makeLog(buf)))

	// Check both services have been removed.
	ws := memdb.NewWatchSet()
	out, err := fsm.State().GetServiceRegistrationByID(ws, services[0].Namespace, services[0].ID)
	assert.Nil(t, err)
	assert.Nil(t, out)

	out, err = fsm.State().GetServiceRegistrationByID(ws, services[1].Namespace, services[1].ID)
	assert.Nil(t, err)
	assert.Nil(t, out)
}

func TestFSM_SnapshotRestore_Variables(t *testing.T) {
	ci.Parallel(t)

	// Create our initial FSM which will be snapshotted.
	fsm := testFSM(t)
	testState := fsm.State()

	// Generate and upsert some variables.
	msvs := mock.VariablesEncrypted(3, 3)
	svs := msvs.List()

	for _, sv := range svs {
		setResp := testState.VarSet(10, &structs.VarApplyStateRequest{
			Op:  structs.VarOpSet,
			Var: sv,
		})
		require.NoError(t, setResp.Error)
	}

	// Update the mock variables data with the actual create information
	iter, err := testState.Variables(memdb.NewWatchSet())
	require.NoError(t, err)

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		sv := raw.(*structs.VariableEncrypted)
		msvs[sv.Path].CreateIndex = sv.CreateIndex
		msvs[sv.Path].CreateTime = sv.CreateTime
		msvs[sv.Path].ModifyIndex = sv.ModifyIndex
		msvs[sv.Path].ModifyTime = sv.ModifyTime
	}
	svs = msvs.List()

	// List the variables from restored state and ensure everything
	// is as expected.

	// Perform a snapshot restore.
	restoredFSM := testSnapshotRestore(t, fsm)
	restoredState := restoredFSM.State()

	// List the variables from restored state and ensure everything
	// is as expected.
	iter, err = restoredState.Variables(memdb.NewWatchSet())
	require.NoError(t, err)

	var restoredSVs []*structs.VariableEncrypted

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		restoredSVs = append(restoredSVs, raw.(*structs.VariableEncrypted))
	}
	require.ElementsMatch(t, restoredSVs, svs)
}

func TestFSM_ApplyACLRolesUpsert(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	// Create the policies our ACL roles wants to link to.
	policy1 := mock.ACLPolicy()
	policy1.Name = "mocked-test-policy-1"
	policy2 := mock.ACLPolicy()
	policy2.Name = "mocked-test-policy-2"

	require.NoError(t, fsm.State().UpsertACLPolicies(
		structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2}))

	// Generate the upsert request and apply the change.
	req := structs.ACLRolesUpsertRequest{
		ACLRoles: []*structs.ACLRole{mock.ACLRole(), mock.ACLRole()},
	}
	buf, err := structs.Encode(structs.ACLRolesUpsertRequestType, req)
	require.NoError(t, err)
	require.Nil(t, fsm.Apply(makeLog(buf)))

	// Read out both ACL roles and perform an equality check using the hash.
	ws := memdb.NewWatchSet()
	out, err := fsm.State().GetACLRoleByName(ws, req.ACLRoles[0].Name)
	require.NoError(t, err)
	require.Equal(t, req.ACLRoles[0].Hash, out.Hash)

	out, err = fsm.State().GetACLRoleByName(ws, req.ACLRoles[1].Name)
	require.NoError(t, err)
	require.Equal(t, req.ACLRoles[1].Hash, out.Hash)
}

func TestFSM_ApplyACLRolesDeleteByID(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	// Create the policies our ACL roles wants to link to.
	policy1 := mock.ACLPolicy()
	policy1.Name = "mocked-test-policy-1"
	policy2 := mock.ACLPolicy()
	policy2.Name = "mocked-test-policy-2"

	require.NoError(t, fsm.State().UpsertACLPolicies(
		structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2}))

	// Generate and upsert two ACL roles.
	aclRoles := []*structs.ACLRole{mock.ACLRole(), mock.ACLRole()}
	require.NoError(t, fsm.State().UpsertACLRoles(structs.MsgTypeTestSetup, 10, aclRoles, false))

	// Build and apply our message.
	req := structs.ACLRolesDeleteByIDRequest{ACLRoleIDs: []string{aclRoles[0].ID, aclRoles[1].ID}}
	buf, err := structs.Encode(structs.ACLRolesDeleteByIDRequestType, req)
	require.NoError(t, err)
	require.Nil(t, fsm.Apply(makeLog(buf)))

	// List all ACL roles within state to ensure both have been removed.
	ws := memdb.NewWatchSet()
	iter, err := fsm.State().GetACLRoles(ws)
	require.NoError(t, err)

	var count int
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count++
	}
	require.Equal(t, 0, count)
}

func TestFSM_ACLEvents(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		desc     string
		setupfn  func(t *testing.T, fsm *nomadFSM)
		raftReq  func(t *testing.T) []byte
		reqTopic structs.Topic
		eventfn  func(t *testing.T, e []structs.Event)
	}{
		{
			desc: "ACLToken upserted",
			raftReq: func(t *testing.T) []byte {
				req := structs.ACLTokenUpsertRequest{
					Tokens: []*structs.ACLToken{mock.ACLToken()},
				}
				buf, err := structs.Encode(structs.ACLTokenUpsertRequestType, req)
				require.NoError(t, err)
				return buf
			},
			reqTopic: structs.TopicACLToken,
			eventfn: func(t *testing.T, e []structs.Event) {
				require.Len(t, e, 1)
				require.Equal(t, e[0].Topic, structs.TopicACLToken)
				require.Empty(t, e[0].Payload.(*structs.ACLTokenEvent).ACLToken.SecretID)
				require.Equal(t, e[0].Type, structs.TypeACLTokenUpserted)
			},
		},
		{
			desc: "ACLToken deleted",
			setupfn: func(t *testing.T, fsm *nomadFSM) {
				token := mock.ACLToken()
				token.SecretID = "26be01d3-df3a-45e9-9f49-4487a3dc3496"
				token.AccessorID = "b971acba-bbe5-4274-bdfa-8bb1f542a8c1"

				require.NoError(t,
					fsm.State().UpsertACLTokens(
						structs.MsgTypeTestSetup, 10, []*structs.ACLToken{token}))
			},
			raftReq: func(t *testing.T) []byte {
				req := structs.ACLTokenDeleteRequest{
					AccessorIDs: []string{"b971acba-bbe5-4274-bdfa-8bb1f542a8c1"},
				}
				buf, err := structs.Encode(structs.ACLTokenDeleteRequestType, req)
				require.NoError(t, err)
				return buf
			},
			reqTopic: structs.TopicACLToken,
			eventfn: func(t *testing.T, e []structs.Event) {
				require.Len(t, e, 1)
				require.Equal(t, e[0].Topic, structs.TopicACLToken)
				require.Empty(t, e[0].Payload.(*structs.ACLTokenEvent).ACLToken.SecretID)
				require.Equal(t, e[0].Type, structs.TypeACLTokenDeleted)
			},
		},
		{
			desc: "ACLPolicy upserted",
			raftReq: func(t *testing.T) []byte {
				req := structs.ACLPolicyUpsertRequest{
					Policies: []*structs.ACLPolicy{mock.ACLPolicy()},
				}
				buf, err := structs.Encode(structs.ACLPolicyUpsertRequestType, req)
				require.NoError(t, err)
				return buf
			},
			reqTopic: structs.TopicACLPolicy,
			eventfn: func(t *testing.T, e []structs.Event) {
				require.Len(t, e, 1)
				require.Equal(t, e[0].Topic, structs.TopicACLPolicy)
				require.Equal(t, e[0].Type, structs.TypeACLPolicyUpserted)
			},
		},
		{
			desc: "ACLPolicy deleted",
			setupfn: func(t *testing.T, fsm *nomadFSM) {
				policy := mock.ACLPolicy()
				policy.Name = "some-policy"

				require.NoError(t,
					fsm.State().UpsertACLPolicies(
						structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy}))
			},
			raftReq: func(t *testing.T) []byte {
				req := structs.ACLPolicyDeleteRequest{
					Names: []string{"some-policy"},
				}
				buf, err := structs.Encode(structs.ACLPolicyDeleteRequestType, req)
				require.NoError(t, err)
				return buf
			},
			reqTopic: structs.TopicACLPolicy,
			eventfn: func(t *testing.T, e []structs.Event) {
				require.Len(t, e, 1)
				require.Equal(t, e[0].Topic, structs.TopicACLPolicy)
				require.Equal(t, e[0].Type, structs.TypeACLPolicyDeleted)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			fsm := testFSM(t)

			// Setup any state necessary
			if tc.setupfn != nil {
				tc.setupfn(t, fsm)
			}

			// Apply the log
			resp := fsm.Apply(makeLog(tc.raftReq(t)))
			require.Nil(t, resp)

			broker, err := fsm.State().EventBroker()
			require.NoError(t, err)

			subReq := &stream.SubscribeRequest{
				Topics: map[structs.Topic][]string{
					tc.reqTopic: {"*"},
				},
				Namespace: "default",
			}

			sub, err := broker.Subscribe(subReq)
			require.NoError(t, err)

			var events []structs.Event

			testutil.WaitForResult(func() (bool, error) {
				out, err := sub.NextNoBlock()
				require.NoError(t, err)

				if out == nil {
					return false, fmt.Errorf("expected events got nil")
				}

				events = out
				return true, nil
			}, func(err error) {
				require.Fail(t, err.Error())
			})

			tc.eventfn(t, events)
		})
	}
}

// TestFSM_EventBroker_JobRegisterFSMEvents asserts that only a single job
// register event is emitted when registering a job
func TestFSM_EventBroker_JobRegisterFSMEvents(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	job := mock.Job()
	eval := mock.Eval()
	eval.JobID = job.ID

	req := structs.JobRegisterRequest{
		Job:  job,
		Eval: eval,
	}
	buf, err := structs.Encode(structs.JobRegisterRequestType, req)
	require.NoError(t, err)

	resp := fsm.Apply(makeLog(buf))
	require.Nil(t, resp)

	broker, err := fsm.State().EventBroker()
	require.NoError(t, err)

	subReq := &stream.SubscribeRequest{
		Topics: map[structs.Topic][]string{
			structs.TopicJob: {"*"},
		},
		Namespace: "default",
	}

	sub, err := broker.Subscribe(subReq)
	require.NoError(t, err)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(500*time.Millisecond))
	defer cancel()

	// consume the queue
	var events []structs.Event
	for {
		out, err := sub.Next(ctx)
		if len(out.Events) == 0 {
			break
		}

		// consume the queue until the deadline has exceeded or until we've
		// received more events than  expected
		if err == context.DeadlineExceeded || len(events) > 1 {
			break
		}

		events = append(events, out.Events...)
	}

	require.Len(t, events, 1)
	require.Equal(t, structs.TypeJobRegistered, events[0].Type)
}

func TestFSM_UpsertACLAuthMethods(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	am1 := mock.ACLOIDCAuthMethod()
	am2 := mock.ACLOIDCAuthMethod()
	req := structs.ACLAuthMethodUpsertRequest{
		AuthMethods: []*structs.ACLAuthMethod{am1, am2},
	}
	buf, err := structs.Encode(structs.ACLAuthMethodsUpsertRequestType, req)
	must.Nil(t, err)
	must.Nil(t, fsm.Apply(makeLog(buf)))

	// Verify we are registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().GetACLAuthMethodByName(ws, am1.Name)
	must.Nil(t, err)
	must.NotNil(t, out)

	out, err = fsm.State().GetACLAuthMethodByName(ws, am2.Name)
	must.Nil(t, err)
	must.NotNil(t, out)
}

func TestFSM_DeleteACLAuthMethods(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	am1 := mock.ACLOIDCAuthMethod()
	am2 := mock.ACLOIDCAuthMethod()
	must.Nil(t, fsm.State().UpsertACLAuthMethods(1000, []*structs.ACLAuthMethod{am1, am2}))

	req := structs.ACLAuthMethodDeleteRequest{
		Names: []string{am1.Name, am2.Name},
	}
	buf, err := structs.Encode(structs.ACLAuthMethodsDeleteRequestType, req)
	must.Nil(t, err)
	must.Nil(t, fsm.Apply(makeLog(buf)))

	// Verify we are NOT registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().GetACLAuthMethodByName(ws, am1.Name)
	must.Nil(t, err)
	must.Nil(t, out)

	out, err = fsm.State().GetACLAuthMethodByName(ws, am2.Name)
	must.Nil(t, err)
	must.Nil(t, out)
}

func TestFSM_UpsertACLBindingRules(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	// Create an auth method and upsert so the binding rules can link to this.
	authMethod := mock.ACLOIDCAuthMethod()
	must.NoError(t, fsm.state.UpsertACLAuthMethods(10, []*structs.ACLAuthMethod{authMethod}))

	aclBindingRule1 := mock.ACLBindingRule()
	aclBindingRule1.AuthMethod = authMethod.Name
	aclBindingRule2 := mock.ACLBindingRule()
	aclBindingRule2.AuthMethod = authMethod.Name

	req := structs.ACLBindingRulesUpsertRequest{
		ACLBindingRules: []*structs.ACLBindingRule{aclBindingRule1, aclBindingRule2},
	}
	buf, err := structs.Encode(structs.ACLBindingRulesUpsertRequestType, req)
	must.NoError(t, err)
	must.Nil(t, fsm.Apply(makeLog(buf)))

	// Ensure the ACL binding rules have been upserted correctly.
	ws := memdb.NewWatchSet()
	out, err := fsm.State().GetACLBindingRule(ws, aclBindingRule1.ID)
	must.Nil(t, err)
	must.Eq(t, aclBindingRule1, out)

	out, err = fsm.State().GetACLBindingRule(ws, aclBindingRule2.ID)
	must.Nil(t, err)
	must.Eq(t, aclBindingRule2, out)
}

func TestFSM_DeleteACLBindingRules(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	aclBindingRule1 := mock.ACLBindingRule()
	aclBindingRule2 := mock.ACLBindingRule()
	must.NoError(t, fsm.State().UpsertACLBindingRules(
		10, []*structs.ACLBindingRule{aclBindingRule1, aclBindingRule2}, true))

	req := structs.ACLBindingRulesDeleteRequest{
		ACLBindingRuleIDs: []string{aclBindingRule1.ID, aclBindingRule2.ID},
	}
	buf, err := structs.Encode(structs.ACLBindingRulesDeleteRequestType, req)
	must.NoError(t, err)
	must.Nil(t, fsm.Apply(makeLog(buf)))

	// Ensure neither ACL binding rule is now found.
	ws := memdb.NewWatchSet()
	out, err := fsm.State().GetACLBindingRule(ws, aclBindingRule1.ID)
	must.NoError(t, err)
	must.Nil(t, out)

	out, err = fsm.State().GetACLBindingRule(ws, aclBindingRule2.ID)
	must.NoError(t, err)
	must.Nil(t, out)
}
