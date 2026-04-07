// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

func Test_BatchTimeoutWatcher_StopsExpiredBatchAlloc(t *testing.T) {
	t.Parallel()

	srv, cleanup := TestServer(t, nil)
	t.Cleanup(cleanup)

	testutil.WaitForLeader(t, srv.RPC)

	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.TaskGroups[0].MaxRunDuration = pointer.Of(50 * time.Millisecond)
	job.Canonicalize()

	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.Namespace = job.Namespace
	alloc.TaskGroup = job.TaskGroups[0].Name
	alloc.DesiredStatus = structs.AllocDesiredStatusRun
	alloc.ClientStatus = structs.AllocClientStatusRunning
	alloc.TaskStates = map[string]*structs.TaskState{
		job.TaskGroups[0].Tasks[0].Name: {
			State:     structs.TaskStateRunning,
			StartedAt: time.Now().UTC().Add(-1 * time.Second),
		},
	}

	state := srv.fsm.State()
	must.NoError(t, state.UpsertJobSummary(998, mock.JobSummary(job.ID)))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, job))
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc}))

	testutil.WaitForResult(func() (bool, error) {
		out, err := state.AllocByID(nil, alloc.ID)
		if err != nil {
			return false, err
		}
		if out == nil {
			return false, nil
		}

		return out.DesiredStatus == structs.AllocDesiredStatusStop &&
			out.DesiredDescription == structs.AllocTimeoutReasonMaxRunDuration &&
			out.ClientStatus == structs.AllocClientStatusFailed, nil
	}, func(err error) {
		out, lookupErr := state.AllocByID(nil, alloc.ID)
		if lookupErr != nil {
			t.Fatalf("failed to lookup alloc after waiting: %v", lookupErr)
		}
		t.Fatalf("timed out waiting for batch timeout watcher to stop alloc: %v; alloc=%#v", err, out)
	})
}

func Test_BatchTimeoutWatcher_StopsExpiredSysBatchAlloc(t *testing.T) {
	t.Parallel()

	srv, cleanup := TestServer(t, nil)
	t.Cleanup(cleanup)

	testutil.WaitForLeader(t, srv.RPC)

	job := mock.SystemBatchJob()
	job.TaskGroups[0].MaxRunDuration = pointer.Of(50 * time.Millisecond)
	job.Canonicalize()

	alloc := mock.SysBatchAlloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.Namespace = job.Namespace
	alloc.TaskGroup = job.TaskGroups[0].Name
	alloc.DesiredStatus = structs.AllocDesiredStatusRun
	alloc.ClientStatus = structs.AllocClientStatusRunning
	alloc.TaskStates = map[string]*structs.TaskState{
		job.TaskGroups[0].Tasks[0].Name: {
			State:     structs.TaskStateRunning,
			StartedAt: time.Now().UTC().Add(-1 * time.Second),
		},
	}

	state := srv.fsm.State()
	must.NoError(t, state.UpsertJobSummary(998, mock.JobSummary(job.ID)))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, job))
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc}))

	testutil.WaitForResult(func() (bool, error) {
		out, err := state.AllocByID(nil, alloc.ID)
		if err != nil {
			return false, err
		}
		if out == nil {
			return false, nil
		}

		return out.DesiredStatus == structs.AllocDesiredStatusStop &&
			out.DesiredDescription == structs.AllocTimeoutReasonMaxRunDuration &&
			out.ClientStatus == structs.AllocClientStatusFailed, nil
	}, func(err error) {
		out, lookupErr := state.AllocByID(nil, alloc.ID)
		if lookupErr != nil {
			t.Fatalf("failed to lookup alloc after waiting: %v", lookupErr)
		}
		t.Fatalf("timed out waiting for batch timeout watcher to stop sysbatch alloc: %v; alloc=%#v", err, out)
	})
}
