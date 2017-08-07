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
	if err := state.UpsertJob(jobIndex, job); err != nil {
		t.Fatalf("err: %v", err)
	}

	return job.ID
}

func TestResourcesEndpoint_List(t *testing.T) {
	assert := assert.New(t)
	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	jobID := registerAndVerifyJob(s, t, prefix, 0)

	req := &structs.ResourceListRequest{
		Prefix:  prefix,
		Context: "jobs",
	}

	var resp structs.ResourceListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches["jobs"]))
	assert.Equal(jobID, resp.Matches["jobs"][0])
	assert.Equal(uint64(jobIndex), resp.Index)
}

// truncate should limit results to 20
func TestResourcesEndpoint_List_Truncate(t *testing.T) {
	assert := assert.New(t)
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

	req := &structs.ResourceListRequest{
		Prefix:  prefix,
		Context: "jobs",
	}

	var resp structs.ResourceListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(20, len(resp.Matches["jobs"]))
	assert.Equal(resp.Truncations["jobs"], true)
	assert.Equal(uint64(jobIndex), resp.Index)
}

func TestResourcesEndpoint_List_Evals(t *testing.T) {
	assert := assert.New(t)
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

	req := &structs.ResourceListRequest{
		Prefix:  prefix,
		Context: "evals",
	}

	var resp structs.ResourceListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches["evals"]))
	assert.Equal(eval1.ID, resp.Matches["evals"][0])
	assert.Equal(resp.Truncations["evals"], false)

	assert.Equal(uint64(2000), resp.Index)
}

func TestResourcesEndpoint_List_Allocation(t *testing.T) {
	assert := assert.New(t)
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

	req := &structs.ResourceListRequest{
		Prefix:  prefix,
		Context: "allocs",
	}

	var resp structs.ResourceListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches["allocs"]))
	assert.Equal(alloc.ID, resp.Matches["allocs"][0])
	assert.Equal(resp.Truncations["allocs"], false)

	assert.Equal(uint64(90), resp.Index)
}

func TestResourcesEndpoint_List_Node(t *testing.T) {
	assert := assert.New(t)
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

	req := &structs.ResourceListRequest{
		Prefix:  prefix,
		Context: "nodes",
	}

	var resp structs.ResourceListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches["nodes"]))
	assert.Equal(node.ID, resp.Matches["nodes"][0])
	assert.Equal(false, resp.Truncations["nodes"])

	assert.Equal(uint64(100), resp.Index)
}

func TestResourcesEndpoint_List_InvalidContext(t *testing.T) {
	assert := assert.New(t)

	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	req := &structs.ResourceListRequest{
		Prefix:  "anyPrefix",
		Context: "invalid",
	}

	var resp structs.ResourceListResponse
	err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp)
	assert.Equal(err.Error(), "context must be one of [allocs nodes jobs evals]; got \"invalid\"")

	assert.Equal(uint64(0), resp.Index)
}

func TestResourcesEndpoint_List_NoContext(t *testing.T) {
	assert := assert.New(t)
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

	req := &structs.ResourceListRequest{
		Prefix:  prefix,
		Context: "",
	}

	var resp structs.ResourceListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches["nodes"]))
	assert.Equal(1, len(resp.Matches["evals"]))

	assert.Equal(node.ID, resp.Matches["nodes"][0])
	assert.Equal(eval1.ID, resp.Matches["evals"][0])

	assert.Equal(uint64(1000), resp.Index)
}

// Tests that the top 20 matches are returned when no prefix is set
func TestResourcesEndpoint_List_NoPrefix(t *testing.T) {
	assert := assert.New(t)

	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	jobID := registerAndVerifyJob(s, t, prefix, 0)

	req := &structs.ResourceListRequest{
		Prefix:  "",
		Context: "jobs",
	}

	var resp structs.ResourceListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches["jobs"]))
	assert.Equal(jobID, resp.Matches["jobs"][0])
	assert.Equal(uint64(jobIndex), resp.Index)
}

//// Tests that the zero matches are returned when a prefix has no matching
//// results
func TestResourcesEndpoint_List_NoMatches(t *testing.T) {
	assert := assert.New(t)

	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	req := &structs.ResourceListRequest{
		Prefix:  prefix,
		Context: "jobs",
	}

	var resp structs.ResourceListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Resources.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(0, len(resp.Matches["jobs"]))
	assert.Equal(uint64(0), resp.Index)
}
