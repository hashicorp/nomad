package nomad

import (
	"fmt"
	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"net/rpc"
	"testing"
)

func registerAndVerifyJob(codec rpc.ClientCodec, s *Server, t *testing.T) string {
	// Create the register request
	job := mock.Job()
	state := s.fsm.State()
	err := state.UpsertJob(1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	return job.ID
}

func TestResourcesEndpoint_List(t *testing.T) {
	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	jobID := registerAndVerifyJob(codec, s, t)

	req := &structs.ResourcesRequest{
		QueryOptions: structs.QueryOptions{Region: "global", Prefix: jobID},
	}

	var resp structs.ResourcesResponse
	if err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	num_matches := len(resp.Resources.Matches["jobs"])
	if num_matches != 1 {
		t.Fatalf(fmt.Sprintf("err: the number of jobs expected %d does not match the number of jobs registered %d", 1, num_matches))
	}

	assert.Equal(t, jobID, resp.Resources.Matches["jobs"][0])
}
