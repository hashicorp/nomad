package nomad

import (
	"strconv"
	"strings"
	"testing"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
)

const jobIndex = 1000

func registerAndVerifyJob(s *Server, t *testing.T, prefix string, counter int) *structs.Job {
	job := mock.Job()
	job.ID = prefix + strconv.Itoa(counter)
	state := s.fsm.State()
	if err := state.UpsertJob(jobIndex, job); err != nil {
		t.Fatalf("err: %v", err)
	}

	return job
}

func TestSearch_PrefixSearch_Job(t *testing.T) {
	assert := assert.New(t)
	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	t.Parallel()
	s := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	job := registerAndVerifyJob(s, t, prefix, 0)

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Jobs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Jobs]))
	assert.Equal(job.ID, resp.Matches[structs.Jobs][0])
	assert.Equal(uint64(jobIndex), resp.Index)
}

func TestSearch_PrefixSearch_ACL(t *testing.T) {
	assert := assert.New(t)
	jobID := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	t.Parallel()
	s, root := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)
	state := s.fsm.State()

	job := registerAndVerifyJob(s, t, jobID, 0)
	assert.Nil(state.UpsertNode(1001, mock.Node()))

	req := &structs.SearchRequest{
		Prefix:  "",
		Context: structs.Jobs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Try without a token and expect failure
	{
		var resp structs.SearchResponse
		err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token and expect failure
	{
		invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
			mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))
		req.AuthToken = invalidToken.SecretID
		var resp structs.SearchResponse
		err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a node:read token and expect failure due to Jobs being the context
	{
		validToken := mock.CreatePolicyAndToken(t, state, 1005, "test-invalid2", mock.NodePolicy(acl.PolicyRead))
		req.AuthToken = validToken.SecretID
		var resp structs.SearchResponse
		err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a node:read token and expect success due to All context
	{
		validToken := mock.CreatePolicyAndToken(t, state, 1007, "test-valid", mock.NodePolicy(acl.PolicyRead))
		req.Context = structs.All
		req.AuthToken = validToken.SecretID
		var resp structs.SearchResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))
		assert.Equal(uint64(1001), resp.Index)
		assert.Len(resp.Matches[structs.Nodes], 1)

		// Jobs filtered out since token only has access to node:read
		assert.Len(resp.Matches[structs.Jobs], 0)
	}

	// Try with a valid token for namespace:read-job
	{
		validToken := mock.CreatePolicyAndToken(t, state, 1009, "test-valid2",
			mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))
		req.AuthToken = validToken.SecretID
		var resp structs.SearchResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))
		assert.Len(resp.Matches[structs.Jobs], 1)
		assert.Equal(job.ID, resp.Matches[structs.Jobs][0])

		// Index of job - not node - because node context is filtered out
		assert.Equal(uint64(1000), resp.Index)

		// Nodes filtered out since token only has access to namespace:read-job
		assert.Len(resp.Matches[structs.Nodes], 0)
	}

	// Try with a valid token for node:read and namespace:read-job
	{
		validToken := mock.CreatePolicyAndToken(t, state, 1011, "test-valid3", strings.Join([]string{
			mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}),
			mock.NodePolicy(acl.PolicyRead),
		}, "\n"))
		req.AuthToken = validToken.SecretID
		var resp structs.SearchResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))
		assert.Len(resp.Matches[structs.Jobs], 1)
		assert.Equal(job.ID, resp.Matches[structs.Jobs][0])
		assert.Len(resp.Matches[structs.Nodes], 1)
		assert.Equal(uint64(1001), resp.Index)
	}

	// Try with a management token
	{
		req.AuthToken = root.SecretID
		var resp structs.SearchResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))
		assert.Equal(uint64(1001), resp.Index)
		assert.Len(resp.Matches[structs.Jobs], 1)
		assert.Equal(job.ID, resp.Matches[structs.Jobs][0])
		assert.Len(resp.Matches[structs.Nodes], 1)
	}
}

func TestSearch_PrefixSearch_All_JobWithHyphen(t *testing.T) {
	assert := assert.New(t)
	prefix := "example-test-------" // Assert that a job with more than 4 hyphens works

	t.Parallel()
	s := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Register a job and an allocation
	job := registerAndVerifyJob(s, t, prefix, 0)
	alloc := mock.Alloc()
	alloc.JobID = job.ID
	alloc.Namespace = job.Namespace
	summary := mock.JobSummary(alloc.JobID)
	state := s.fsm.State()

	if err := state.UpsertJobSummary(999, summary); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := state.UpsertAllocs(1000, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	req := &structs.SearchRequest{
		Context: structs.All,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// req.Prefix = "example-te": 9
	for i := 1; i < len(prefix); i++ {
		req.Prefix = prefix[:i]
		var resp structs.SearchResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))
		assert.Equal(1, len(resp.Matches[structs.Jobs]))
		assert.Equal(job.ID, resp.Matches[structs.Jobs][0])
		assert.EqualValues(jobIndex, resp.Index)
	}
}

func TestSearch_PrefixSearch_All_LongJob(t *testing.T) {
	assert := assert.New(t)
	prefix := strings.Repeat("a", 100)

	t.Parallel()
	s := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Register a job and an allocation
	job := registerAndVerifyJob(s, t, prefix, 0)
	alloc := mock.Alloc()
	alloc.JobID = job.ID
	summary := mock.JobSummary(alloc.JobID)
	state := s.fsm.State()

	if err := state.UpsertJobSummary(999, summary); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := state.UpsertAllocs(1000, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.All,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Jobs]))
	assert.Equal(job.ID, resp.Matches[structs.Jobs][0])
	assert.EqualValues(jobIndex, resp.Index)
}

// truncate should limit results to 20
func TestSearch_PrefixSearch_Truncate(t *testing.T) {
	assert := assert.New(t)
	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	t.Parallel()
	s := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	var job *structs.Job
	for counter := 0; counter < 25; counter++ {
		job = registerAndVerifyJob(s, t, prefix, counter)
	}

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Jobs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(20, len(resp.Matches[structs.Jobs]))
	assert.Equal(resp.Truncations[structs.Jobs], true)
	assert.Equal(uint64(jobIndex), resp.Index)
}

func TestSearch_PrefixSearch_AllWithJob(t *testing.T) {
	assert := assert.New(t)
	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	t.Parallel()
	s := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	job := registerAndVerifyJob(s, t, prefix, 0)

	eval1 := mock.Eval()
	eval1.ID = job.ID
	s.fsm.State().UpsertEvals(2000, []*structs.Evaluation{eval1})

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.All,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Jobs]))
	assert.Equal(job.ID, resp.Matches[structs.Jobs][0])

	assert.Equal(1, len(resp.Matches[structs.Evals]))
	assert.Equal(eval1.ID, resp.Matches[structs.Evals][0])
}

func TestSearch_PrefixSearch_Evals(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	eval1 := mock.Eval()
	s.fsm.State().UpsertEvals(2000, []*structs.Evaluation{eval1})

	prefix := eval1.ID[:len(eval1.ID)-2]

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Evals,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: eval1.Namespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Evals]))
	assert.Equal(eval1.ID, resp.Matches[structs.Evals][0])
	assert.Equal(resp.Truncations[structs.Evals], false)

	assert.Equal(uint64(2000), resp.Index)
}

func TestSearch_PrefixSearch_Allocation(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s := TestServer(t, func(c *Config) {
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

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Allocs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: alloc.Namespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Allocs]))
	assert.Equal(alloc.ID, resp.Matches[structs.Allocs][0])
	assert.Equal(resp.Truncations[structs.Allocs], false)

	assert.Equal(uint64(90), resp.Index)
}

func TestSearch_PrefixSearch_All_UUID(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s := TestServer(t, func(c *Config) {
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

	node := mock.Node()
	if err := state.UpsertNode(1001, node); err != nil {
		t.Fatalf("err: %v", err)
	}

	eval1 := mock.Eval()
	eval1.ID = node.ID
	if err := state.UpsertEvals(1002, []*structs.Evaluation{eval1}); err != nil {
		t.Fatalf("err: %v", err)
	}

	req := &structs.SearchRequest{
		Context: structs.All,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: eval1.Namespace,
		},
	}

	for i := 1; i < len(alloc.ID); i++ {
		req.Prefix = alloc.ID[:i]
		var resp structs.SearchResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))
		assert.Equal(1, len(resp.Matches[structs.Allocs]))
		assert.Equal(alloc.ID, resp.Matches[structs.Allocs][0])
		assert.Equal(resp.Truncations[structs.Allocs], false)
		assert.EqualValues(1002, resp.Index)
	}
}

func TestSearch_PrefixSearch_Node(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s := TestServer(t, func(c *Config) {
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

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Nodes,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Nodes]))
	assert.Equal(node.ID, resp.Matches[structs.Nodes][0])
	assert.Equal(false, resp.Truncations[structs.Nodes])

	assert.Equal(uint64(100), resp.Index)
}

func TestSearch_PrefixSearch_Deployment(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	deployment := mock.Deployment()
	s.fsm.State().UpsertDeployment(2000, deployment)

	prefix := deployment.ID[:len(deployment.ID)-2]

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Deployments,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: deployment.Namespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Deployments]))
	assert.Equal(deployment.ID, resp.Matches[structs.Deployments][0])
	assert.Equal(resp.Truncations[structs.Deployments], false)

	assert.Equal(uint64(2000), resp.Index)
}

func TestSearch_PrefixSearch_AllContext(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s := TestServer(t, func(c *Config) {
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

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.All,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: eval1.Namespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Nodes]))
	assert.Equal(1, len(resp.Matches[structs.Evals]))

	assert.Equal(node.ID, resp.Matches[structs.Nodes][0])
	assert.Equal(eval1.ID, resp.Matches[structs.Evals][0])

	assert.Equal(uint64(1000), resp.Index)
}

// Tests that the top 20 matches are returned when no prefix is set
func TestSearch_PrefixSearch_NoPrefix(t *testing.T) {
	assert := assert.New(t)

	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	t.Parallel()
	s := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	job := registerAndVerifyJob(s, t, prefix, 0)

	req := &structs.SearchRequest{
		Prefix:  "",
		Context: structs.Jobs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Jobs]))
	assert.Equal(job.ID, resp.Matches[structs.Jobs][0])
	assert.Equal(uint64(jobIndex), resp.Index)
}

// Tests that the zero matches are returned when a prefix has no matching
// results
func TestSearch_PrefixSearch_NoMatches(t *testing.T) {
	assert := assert.New(t)

	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	t.Parallel()
	s := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Jobs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(0, len(resp.Matches[structs.Jobs]))
	assert.Equal(uint64(0), resp.Index)
}

// Prefixes can only be looked up if their length is a power of two. For
// prefixes which are an odd length, use the length-1 characters.
func TestSearch_PrefixSearch_RoundDownToEven(t *testing.T) {
	assert := assert.New(t)
	id1 := "aaafaaaa-e8f7-fd38-c855-ab94ceb89"
	id2 := "aaafeaaa-e8f7-fd38-c855-ab94ceb89"
	prefix := "aaafa"

	t.Parallel()
	s := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	job := registerAndVerifyJob(s, t, id1, 0)
	registerAndVerifyJob(s, t, id2, 50)

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Jobs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Jobs]))
	assert.Equal(job.ID, resp.Matches[structs.Jobs][0])
}

func TestSearch_PrefixSearch_MultiRegion(t *testing.T) {
	assert := assert.New(t)

	jobName := "exampleexample"

	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.Region = "foo"
	})
	defer s1.Shutdown()

	s2 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.Region = "bar"
	})
	defer s2.Shutdown()

	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)

	job := registerAndVerifyJob(s1, t, jobName, 0)

	req := &structs.SearchRequest{
		Prefix:  "",
		Context: structs.Jobs,
		QueryOptions: structs.QueryOptions{
			Region:    "foo",
			Namespace: job.Namespace,
		},
	}

	codec := rpcClient(t, s2)

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Jobs]))
	assert.Equal(job.ID, resp.Matches[structs.Jobs][0])
	assert.Equal(uint64(jobIndex), resp.Index)
}
