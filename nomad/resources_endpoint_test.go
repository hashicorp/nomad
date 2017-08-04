package nomad

import (
	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
)

const jobIndex = 1000

func registerAndVerifyJob(s *Server, t *testing.T, prefix string, counter int) string {
	job := mock.Job()

	job.ID = prefix + strconv.Itoa(counter)
	state := s.fsm.State()
	err := state.UpsertJob(jobIndex, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	return job.ID
}

func TestResourcesEndpoint_List(t *testing.T) {
	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	jobID := registerAndVerifyJob(s, t, prefix, 0)

	req := &structs.ResourcesRequest{
		Prefix:  prefix,
		Context: "jobs",
	}

	var resp structs.ResourcesResponse
	if err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(t, 1, len(resp.Matches["jobs"]))
	assert.Equal(t, jobID, resp.Matches["jobs"][0])
	assert.Equal(t, uint64(jobIndex), resp.Index)
}

// truncate should limit results to 20
func TestResourcesEndpoint_List_Truncate(t *testing.T) {
	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	for counter := 0; counter < 25; counter++ {
		registerAndVerifyJob(s, t, prefix, counter)
	}

	req := &structs.ResourcesRequest{
		Prefix:  prefix,
		Context: "jobs",
	}

	var resp structs.ResourcesResponse
	if err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(t, 20, len(resp.Matches["jobs"]))
	assert.Equal(t, resp.Truncations["jobs"], true)
	assert.Equal(t, uint64(jobIndex), resp.Index)
}

func TestResourcesEndpoint_List_Evals(t *testing.T) {
	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	eval1 := mock.Eval()
	s.fsm.State().UpsertEvals(2000, []*structs.Evaluation{eval1})

	prefix := eval1.ID[:len(eval1.ID)-2]

	req := &structs.ResourcesRequest{
		Prefix:  prefix,
		Context: "evals",
	}

	var resp structs.ResourcesResponse
	if err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(t, 1, len(resp.Matches["evals"]))
	assert.Equal(t, eval1.ID, resp.Matches["evals"][0])
	assert.Equal(t, resp.Truncations["evals"], false)

	assert.Equal(t, uint64(2000), resp.Index)
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
	if err := state.UpsertAllocs(90, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	prefix := alloc.ID[:len(alloc.ID)-2]

	req := &structs.ResourcesRequest{
		Prefix:  prefix,
		Context: "allocs",
	}

	var resp structs.ResourcesResponse
	if err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(t, 1, len(resp.Matches["allocs"]))
	assert.Equal(t, alloc.ID, resp.Matches["allocs"][0])
	assert.Equal(t, resp.Truncations["allocs"], false)

	assert.Equal(t, uint64(90), resp.Index)
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
		Context: "nodes",
	}

	var resp structs.ResourcesResponse
	if err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(t, 1, len(resp.Matches["nodes"]))
	assert.Equal(t, node.ID, resp.Matches["nodes"][0])
	assert.Equal(t, false, resp.Truncations["nodes"])

	assert.Equal(t, uint64(100), resp.Index)
}

func TestResourcesEndpoint_List_InvalidContext(t *testing.T) {
	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	req := &structs.ResourcesRequest{
		Prefix:  "anyPrefix",
		Context: "invalid",
	}

	var resp structs.ResourcesResponse
	err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp)
	assert.Equal(t, err.Error(), "invalid context")

	assert.Equal(t, uint64(0), resp.Index)
}

func TestResourcesEndpoint_List_NoContext(t *testing.T) {
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

	eval1 := mock.Eval()
	eval1.ID = node.ID
	if err := state.UpsertEvals(1000, []*structs.Evaluation{eval1}); err != nil {
		t.Fatalf("err: %v", err)
	}

	prefix := node.ID[:len(node.ID)-2]

	req := &structs.ResourcesRequest{
		Prefix:  prefix,
		Context: "",
	}

	var resp structs.ResourcesResponse
	if err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(t, 1, len(resp.Matches["nodes"]))
	assert.Equal(t, 1, len(resp.Matches["evals"]))

	assert.Equal(t, node.ID, resp.Matches["nodes"][0])
	assert.Equal(t, eval1.ID, resp.Matches["evals"][0])

	assert.NotEqual(t, uint64(0), resp.Index)
}

// Tests that the top 20 matches are returned when no prefix is set
func TestResourcesEndpoint_List_NoPrefix(t *testing.T) {
	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	jobID := registerAndVerifyJob(s, t, prefix, 0)

	req := &structs.ResourcesRequest{
		Prefix:  "",
		Context: "jobs",
	}

	var resp structs.ResourcesResponse
	if err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(t, 1, len(resp.Matches["jobs"]))
	assert.Equal(t, jobID, resp.Matches["jobs"][0])
	assert.Equal(t, uint64(jobIndex), resp.Index)
}

// Tests that the zero matches are returned when a prefix has no matching
// results
func TestResourcesEndpoint_List_NoMatches(t *testing.T) {
	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	req := &structs.ResourcesRequest{
		Prefix:  prefix,
		Context: "jobs",
	}

	var resp structs.ResourcesResponse
	if err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(t, 0, len(resp.Matches["jobs"]))
	assert.Equal(t, uint64(0), resp.Index)
}
