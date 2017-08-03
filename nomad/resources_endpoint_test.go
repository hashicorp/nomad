package nomad

import (
	"fmt"
	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
)

func registerAndVerifyJob(s *Server, t *testing.T, prefix string, counter int) string {
	job := mock.Job()

	job.ID = prefix + strconv.Itoa(counter)
	state := s.fsm.State()
	err := state.UpsertJob(1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	return job.ID
}

func TestResourcesEndpoint_List(t *testing.T) {
	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	jobID := registerAndVerifyJob(s, t, prefix, 0)

	req := &structs.ResourcesRequest{
		Prefix:  prefix,
		Context: "job",
	}

	var resp structs.ResourcesResponse
	if err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	num_matches := len(resp.Matches["job"])
	if num_matches != 1 {
		t.Fatalf(fmt.Sprintf("err: the number of jobs expected %d does not match the number of jobs registered %d", 1, num_matches))
	}

	assert.Equal(t, jobID, resp.Matches["job"][0])
}

func TestResourcesEndpoint_List_ShouldTruncateResultsToUnder20(t *testing.T) {
	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	for counter := 0; counter < 25; counter++ {
		registerAndVerifyJob(s, t, prefix, counter)
	}

	req := &structs.ResourcesRequest{
		Prefix:  prefix,
		Context: "job",
	}

	var resp structs.ResourcesResponse
	if err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	num_matches := len(resp.Matches["job"])
	if num_matches != 20 {
		t.Fatalf(fmt.Sprintf("err: the number of jobs expected %d does not match the number of jobs returned %d", 20, num_matches))
	}

	assert.Equal(t, resp.Truncations["job"], true)
}

func TestResourcesEndpoint_List_ShouldReturnEvals(t *testing.T) {
	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	eval1 := mock.Eval()
	s.fsm.State().UpsertEvals(1000, []*structs.Evaluation{eval1})

	prefix := eval1.ID[:len(eval1.ID)-2]

	req := &structs.ResourcesRequest{
		Prefix:  prefix,
		Context: "eval",
	}

	var resp structs.ResourcesResponse
	if err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	numMatches := len(resp.Matches["eval"])
	if numMatches != 1 {
		t.Fatalf(fmt.Sprintf("err: the number of evaluations expected %d does not match the number expected %d", 1, numMatches))
	}

	recEval := resp.Matches["eval"][0]
	if recEval != eval1.ID {
		t.Fatalf(fmt.Sprintf("err: expected %s evaluation but received %s", eval1.ID, recEval))
	}

	assert.Equal(t, resp.Truncations["job"], false)
}

func TestResourcesEndpoint_List_Allocation(t *testing.T) {
	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	alloc := mock.Alloc()
	summary := mock.JobSummary(alloc.JobID)
	state := s.fsm.State()

	if err := state.UpsertJobSummary(999, summary); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := state.UpsertAllocs(1000, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	prefix := alloc.ID[:len(alloc.ID)-2]

	req := &structs.ResourcesRequest{
		Prefix:  prefix,
		Context: "alloc",
	}

	var resp structs.ResourcesResponse
	if err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	numMatches := len(resp.Matches["alloc"])
	if numMatches != 1 {
		t.Fatalf(fmt.Sprintf("err: the number of allocations expected %d does not match the number expected %d", 1, numMatches))
	}

	recAlloc := resp.Matches["alloc"][0]
	if recAlloc != alloc.ID {
		t.Fatalf(fmt.Sprintf("err: expected %s allocation but received %s", alloc.ID, recAlloc))
	}

	assert.Equal(t, resp.Truncations["alloc"], false)
}

func TestResourcesEndpoint_List_Node(t *testing.T) {
	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	state := s.fsm.State()
	node := mock.Node()

	if err := state.UpsertNode(100, node); err != nil {
		t.Fatalf("err: %v", err)
	}

	prefix := node.ID[:len(node.ID)-2]

	req := &structs.ResourcesRequest{
		Prefix:  prefix,
		Context: "node",
	}

	var resp structs.ResourcesResponse
	if err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	numMatches := len(resp.Matches["node"])
	if numMatches != 1 {
		t.Fatalf(fmt.Sprintf("err: the number of nodes expected %d does not match the number expected %d", 1, numMatches))
	}

	recNode := resp.Matches["node"][0]
	if recNode != node.ID {
		t.Fatalf(fmt.Sprintf("err: expected %s node but received %s", node.ID, recNode))
	}

	assert.Equal(t, resp.Truncations["node"], false)
}
