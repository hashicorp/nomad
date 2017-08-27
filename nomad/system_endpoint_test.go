package nomad

import (
	"fmt"
	"reflect"
	"testing"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func TestSystemEndpoint_GarbageCollect(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Insert a job that can be GC'd
	state := s1.fsm.State()
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.Stop = true
	if err := state.UpsertJob(1000, job); err != nil {
		t.Fatalf("UpsertJob() failed: %v", err)
	}

	eval := mock.Eval()
	eval.Status = structs.EvalStatusComplete
	eval.JobID = job.ID
	if err := state.UpsertEvals(1001, []*structs.Evaluation{eval}); err != nil {
		t.Fatalf("UpsertEvals() failed: %v", err)
	}

	// Make the GC request
	req := &structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "System.GarbageCollect", req, &resp); err != nil {
		t.Fatalf("expect err")
	}

	testutil.WaitForResult(func() (bool, error) {
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
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})
}

func TestSystemEndpoint_ReconcileSummaries(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Insert a job that can be GC'd
	state := s1.fsm.State()
	s1.fsm.State()
	job := mock.Job()
	if err := state.UpsertJob(1000, job); err != nil {
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
				"web": structs.TaskGroupSummary{
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
