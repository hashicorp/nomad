// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc/v2"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
	"github.com/stretchr/testify/assert"
)

func TestSystemEndpoint_GarbageCollect(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Insert a job that can be GC'd
	state := s1.fsm.State()
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.Stop = true
	// set submit time older than now but still newer than default GC threshold
	job.SubmitTime = time.Now().Add(-10 * time.Millisecond).UnixNano()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job))

	eval := mock.Eval()
	eval.Status = structs.EvalStatusComplete
	eval.JobID = job.ID
	eval.JobModifyIndex = job.ModifyIndex
	// set modify time older than now but still newer than default GC threshold
	eval.ModifyTime = time.Now().Add(-10 * time.Millisecond).UnixNano()
	must.NoError(t, state.UpsertEvals(structs.MsgTypeTestSetup, 1001, []*structs.Evaluation{eval}))

	// Make the GC request
	req := &structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	var resp structs.GenericResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "System.GarbageCollect", req, &resp))

	must.Wait(t, wait.InitialSuccess(wait.TestFunc(func() (bool, error) {
		// Check if the job has been GC'd
		ws := memdb.NewWatchSet()
		exist, err := state.JobByID(ws, job.Namespace, job.ID)
		if err != nil {
			return false, err
		}
		if exist != nil {
			return false, fmt.Errorf("job %+v wasn't garbage collected", job)
		}
		return true, nil
	}), wait.Timeout(3*time.Second)))
}

func TestSystemEndpoint_GarbageCollect_ACL(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	assert := assert.New(t)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create ACL tokens
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid", mock.NodePolicy(acl.PolicyWrite))

	// Make the GC request
	req := &structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}

	// Try without a token and expect failure
	{
		var resp structs.GenericResponse
		err := msgpackrpc.CallWithCodec(codec, "System.GarbageCollect", req, &resp)
		assert.NotNil(err)
		assert.Contains(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token and expect failure
	{
		req.AuthToken = invalidToken.SecretID
		var resp structs.GenericResponse
		err := msgpackrpc.CallWithCodec(codec, "System.GarbageCollect", req, &resp)
		assert.NotNil(err)
		assert.Contains(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a management token
	{
		req.AuthToken = root.SecretID
		var resp structs.GenericResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "System.GarbageCollect", req, &resp))
	}
}

func TestSystemEndpoint_ReconcileSummaries(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Insert a job that can be GC'd
	state := s1.fsm.State()
	s1.fsm.State()
	job := mock.Job()
	if err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job); err != nil {
		t.Fatalf("UpsertJob() failed: %v", err)
	}

	// Delete the job summary
	state.DeleteJobSummary(1001, job.Namespace, job.ID)

	// Make the GC request
	req := &structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "System.ReconcileJobSummaries", req, &resp); err != nil {
		t.Fatalf("expect err: %v", err)
	}

	testutil.WaitForResult(func() (bool, error) {
		// Check if Nomad has reconciled the summary for the job
		ws := memdb.NewWatchSet()
		summary, err := state.JobSummaryByID(ws, job.Namespace, job.ID)
		if err != nil {
			return false, err
		}
		if summary.CreateIndex == 0 || summary.ModifyIndex == 0 {
			t.Fatalf("create index: %v, modify index: %v", summary.CreateIndex, summary.ModifyIndex)
		}

		// setting the modifyindex and createindex of the expected summary to
		// the output so that we can do deep equal
		expectedSummary := structs.JobSummary{
			JobID:     job.ID,
			Namespace: job.Namespace,
			Summary: map[string]structs.TaskGroupSummary{
				"web": {
					Queued: 10,
				},
			},
			ModifyIndex: summary.ModifyIndex,
			CreateIndex: summary.CreateIndex,
		}
		if !reflect.DeepEqual(&expectedSummary, summary) {
			return false, fmt.Errorf("expected: %v, actual: %v", expectedSummary, summary)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})
}

func TestSystemEndpoint_ReconcileJobSummaries_ACL(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	assert := assert.New(t)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create ACL tokens
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid", mock.NodePolicy(acl.PolicyWrite))

	// Make the request
	req := &structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}

	// Try without a token and expect failure
	{
		var resp structs.GenericResponse
		err := msgpackrpc.CallWithCodec(codec, "System.ReconcileJobSummaries", req, &resp)
		assert.NotNil(err)
		assert.Contains(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token and expect failure
	{
		req.AuthToken = invalidToken.SecretID
		var resp structs.GenericResponse
		err := msgpackrpc.CallWithCodec(codec, "System.ReconcileJobSummaries", req, &resp)
		assert.NotNil(err)
		assert.Contains(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a management token
	{
		req.AuthToken = root.SecretID
		var resp structs.GenericResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "System.ReconcileJobSummaries", req, &resp))
	}
}
