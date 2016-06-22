package nomad

import (
	"fmt"
	"testing"

	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func TestSystemEndpoint_GarbageCollect(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Insert a job that can be GC'd
	state := s1.fsm.State()
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	if err := state.UpsertJob(1000, job); err != nil {
		t.Fatalf("UpsertAllocs() failed: %v", err)
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
		exist, err := state.JobByID(job.ID)
		if err != nil {
			return false, err
		}
		if exist != nil {
			return false, fmt.Errorf("job %q wasn't garbage collected", job.ID)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})
}
