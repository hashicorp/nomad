// +build ent

package nomad

import (
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
)

func TestQuotaEndpoint_GetQuotaSpec(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the quota spec
	qs := mock.QuotaSpec()
	s1.fsm.State().UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs})

	// Lookup the QuotaSpec
	get := &structs.QuotaSpecSpecificRequest{
		Name:         qs.Name,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp structs.SingleQuotaSpecResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.GetQuotaSpec", get, &resp))
	assert.EqualValues(1000, resp.Index)
	assert.Equal(qs, resp.Quota)

	// Lookup non-existing quota
	get.Name = uuid.Generate()
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.GetQuotaSpec", get, &resp))
	assert.EqualValues(1000, resp.Index)
	assert.Nil(resp.Quota)
}

func TestQuotaEndpoint_GetQuotaSpec_ACL(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	qs1 := mock.QuotaSpec()
	state := s1.fsm.State()
	s1.fsm.State().UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs1})

	// Create the policy and tokens
	validToken := mock.CreatePolicyAndToken(t, state, 1002, "test-valid",
		mock.QuotaPolicy(acl.PolicyRead))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NodePolicy(acl.PolicyRead))

	// Lookup the QuotaSpec
	get := &structs.QuotaSpecSpecificRequest{
		Name:         qs1.Name,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Lookup the quota without a token and expect failure
	{
		var resp structs.SingleQuotaSpecResponse
		err := msgpackrpc.CallWithCodec(codec, "Quota.GetQuotaSpec", get, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token
	get.SecretID = invalidToken.SecretID
	{
		var resp structs.SingleQuotaSpecResponse
		err := msgpackrpc.CallWithCodec(codec, "Quota.GetQuotaSpec", get, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a valid token
	get.SecretID = validToken.SecretID
	{
		var resp structs.SingleQuotaSpecResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.GetQuotaSpec", get, &resp))
		assert.EqualValues(1000, resp.Index)
		assert.Equal(qs1, resp.Quota)
	}

	// Try with a root token
	get.SecretID = root.SecretID
	{
		var resp structs.SingleQuotaSpecResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.GetQuotaSpec", get, &resp))
		assert.EqualValues(1000, resp.Index)
		assert.Equal(qs1, resp.Quota)
	}
}

func TestQuotaEndpoint_GetQuotaSpec_Blocking(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the quota specs
	qs1 := mock.QuotaSpec()
	qs2 := mock.QuotaSpec()

	// First create a quota spec
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.UpsertQuotaSpecs(100, []*structs.QuotaSpec{qs1}))
	})

	// Upsert the quota spec we are watching later
	time.AfterFunc(200*time.Millisecond, func() {
		assert.Nil(state.UpsertQuotaSpecs(200, []*structs.QuotaSpec{qs2}))
	})

	// Lookup the spec
	req := &structs.QuotaSpecSpecificRequest{
		Name: qs2.Name,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
		},
	}
	var resp structs.SingleQuotaSpecResponse
	start := time.Now()
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.GetQuotaSpec", req, &resp))
	assert.EqualValues(200, resp.Index)
	assert.NotNil(resp.Quota)
	assert.Equal(qs2.Name, resp.Quota.Name)

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}

	// Quota spec delete triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.DeleteQuotaSpecs(300, []string{qs2.Name}))
	})

	req.QueryOptions.MinQueryIndex = 250
	var resp2 structs.SingleQuotaSpecResponse
	start = time.Now()
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.GetQuotaSpec", req, &resp2))
	assert.EqualValues(300, resp2.Index)
	assert.Nil(resp2.Quota)

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
}

func TestQuotaEndpoint_GetQuotaSpecs(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the quota specs
	qs1 := mock.QuotaSpec()
	qs2 := mock.QuotaSpec()
	state := s1.fsm.State()
	state.UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs1, qs2})

	// Lookup the QuotaSpec
	get := &structs.QuotaSpecSetRequest{
		Names:        []string{qs1.Name, qs2.Name},
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Create the policy and tokens
	invalidToken := mock.CreatePolicyAndToken(t, state, 1002, "test-invalid",
		mock.NamespacePolicy("foo", "", []string{acl.NamespaceCapabilityReadJob}))

	// Lookup with a random token
	get.SecretID = invalidToken.SecretID
	{
		var resp structs.QuotaSpecSetResponse
		err := msgpackrpc.CallWithCodec(codec, "Quota.GetQuotaSpecs", get, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Lookup with management token
	get.SecretID = root.SecretID
	{
		var resp structs.QuotaSpecSetResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.GetQuotaSpecs", get, &resp))
		assert.EqualValues(1000, resp.Index)
		assert.Len(resp.Quotas, 2)
		assert.Contains(resp.Quotas, qs1.Name)
		assert.Contains(resp.Quotas, qs2.Name)
	}
}

func TestQuotaEndpoint_GetQuotaSpecs_Blocking(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the quota specs
	qs1 := mock.QuotaSpec()
	qs2 := mock.QuotaSpec()

	// First create a quota spec
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.UpsertQuotaSpecs(100, []*structs.QuotaSpec{qs1}))
	})

	// Upsert the quota spec we are watching later
	time.AfterFunc(200*time.Millisecond, func() {
		assert.Nil(state.UpsertQuotaSpecs(200, []*structs.QuotaSpec{qs2}))
	})

	// Lookup the spec
	req := &structs.QuotaSpecSetRequest{
		Names: []string{qs2.Name},
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
			SecretID:      root.SecretID,
		},
	}
	var resp structs.QuotaSpecSetResponse
	start := time.Now()
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.GetQuotaSpecs", req, &resp))
	assert.EqualValues(200, resp.Index)
	assert.Len(resp.Quotas, 1)
	assert.Contains(resp.Quotas, qs2.Name)

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}

	// Quota spec delete triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.DeleteQuotaSpecs(300, []string{qs2.Name}))
	})

	req.QueryOptions.MinQueryIndex = 250
	var resp2 structs.QuotaSpecSetResponse
	start = time.Now()
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.GetQuotaSpecs", req, &resp2))
	assert.EqualValues(300, resp2.Index)
	assert.Empty(resp2.Quotas)

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
}

func TestQuotaEndpoint_ListQuotaSpecs(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	qs1 := mock.QuotaSpec()
	qs2 := mock.QuotaSpec()

	qs1.Name = "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9"
	qs2.Name = "aaaabbbb-3350-4b4b-d185-0e1992ed43e9"
	assert.Nil(s1.fsm.State().UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs1, qs2}))

	// Lookup the quota specs
	get := &structs.QuotaSpecListRequest{
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp structs.QuotaSpecListResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.ListQuotaSpecs", get, &resp))
	assert.EqualValues(1000, resp.Index)
	assert.Len(resp.Quotas, 2)

	// Lookup the quota specs by prefix
	get = &structs.QuotaSpecListRequest{
		QueryOptions: structs.QueryOptions{
			Region: "global",
			Prefix: "aaaabb",
		},
	}
	var resp2 structs.QuotaSpecListResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.ListQuotaSpecs", get, &resp2))
	assert.EqualValues(1000, resp2.Index)
	assert.Len(resp2.Quotas, 1)
}

func TestQuotaEndpoint_ListQuotaSpecs_ACL(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	qs1 := mock.QuotaSpec()
	qs2 := mock.QuotaSpec()
	state := s1.fsm.State()

	qs1.Name = "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9"
	qs2.Name = "bbbbbbbb-3350-4b4b-d185-0e1992ed43e9"
	assert.Nil(state.UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs1, qs2}))

	// Create a namespace that attaches to one of the quotas
	ns1 := mock.Namespace()
	ns1.Quota = qs1.Name
	assert.Nil(state.UpsertNamespaces(1001, []*structs.Namespace{ns1}))

	validQuotaToken := mock.CreatePolicyAndToken(t, state, 1002, "test-quota-valid", mock.QuotaPolicy(acl.PolicyRead))
	validNamespaceToken := mock.CreatePolicyAndToken(t, state, 1004, "test-namespace-valid",
		mock.NamespacePolicy(ns1.Name, "", []string{acl.NamespaceCapabilityReadJob}))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1006, "test-invalid",
		mock.NamespacePolicy("invalid-namespace", "", []string{acl.NamespaceCapabilityReadJob}))

	get := &structs.QuotaSpecListRequest{
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Lookup the quota specs without a token and expect nothing
	{
		var resp structs.QuotaSpecListResponse
		err := msgpackrpc.CallWithCodec(codec, "Quota.ListQuotaSpecs", get, &resp)
		assert.Nil(err)
		assert.Len(resp.Quotas, 0)
	}

	// Try with an invalid token
	get.SecretID = invalidToken.SecretID
	{
		var resp structs.QuotaSpecListResponse
		err := msgpackrpc.CallWithCodec(codec, "Quota.ListQuotaSpecs", get, &resp)
		assert.Nil(err)
		assert.Len(resp.Quotas, 0)
	}

	// Try with a valid token for one
	get.SecretID = validNamespaceToken.SecretID
	{
		var resp structs.QuotaSpecListResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.ListQuotaSpecs", get, &resp))
		assert.EqualValues(1000, resp.Index)
		assert.Len(resp.Quotas, 1)
		assert.Equal(qs1.Name, resp.Quotas[0].Name)
	}

	// Try with a valid token for all
	get.SecretID = validQuotaToken.SecretID
	{
		var resp structs.QuotaSpecListResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.ListQuotaSpecs", get, &resp))
		assert.EqualValues(1000, resp.Index)
		assert.Len(resp.Quotas, 2)
	}

	// Try with a root token
	get.SecretID = root.SecretID
	{
		var resp structs.QuotaSpecListResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.ListQuotaSpecs", get, &resp))
		assert.EqualValues(1000, resp.Index)
		assert.Len(resp.Quotas, 2)
	}
}

func TestQuotaEndpoint_ListQuotaSpecs_Blocking(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the quotaspec
	qs := mock.QuotaSpec()

	// Upsert quotaspec triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.UpsertQuotaSpecs(200, []*structs.QuotaSpec{qs}))
	})

	req := &structs.QuotaSpecListRequest{
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
		},
	}
	start := time.Now()
	var resp structs.QuotaSpecListResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.ListQuotaSpecs", req, &resp))

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	assert.EqualValues(200, resp.Index)
	assert.Len(resp.Quotas, 1)

	// Quota spec deletion triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.DeleteQuotaSpecs(300, []string{qs.Name}))
	})

	req.MinQueryIndex = 200
	start = time.Now()
	var resp2 structs.QuotaSpecListResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.ListQuotaSpecs", req, &resp2))

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	assert.EqualValues(300, resp2.Index)
	assert.Len(resp2.Quotas, 0)
}

func TestQuotaEndpoint_DeleteQuotaSpecs(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	qs1 := mock.QuotaSpec()
	qs2 := mock.QuotaSpec()
	s1.fsm.State().UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs1, qs2})

	// Lookup the quotas
	req := &structs.QuotaSpecDeleteRequest{
		Names:        []string{qs1.Name, qs2.Name},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.GenericResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.DeleteQuotaSpecs", req, &resp))
	assert.NotEqual(uint64(0), resp.Index)
}

func TestQuotaEndpoint_DeleteQuotaSpecs_ACL(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	qs1 := mock.QuotaSpec()
	qs2 := mock.QuotaSpec()
	state := s1.fsm.State()
	s1.fsm.State().UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs1, qs2})

	// Create the policy and tokens
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))
	validToken := mock.CreatePolicyAndToken(t, state, 1003, "test-valid", mock.QuotaPolicy(acl.PolicyWrite))

	req := &structs.QuotaSpecDeleteRequest{
		Names:        []string{qs1.Name},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Delete quotaspec without a token and expect failure
	{
		var resp structs.GenericResponse
		err := msgpackrpc.CallWithCodec(codec, "Quota.DeleteQuotaSpecs", req, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())

		// Check we did not delete the quota specs
		out, err := state.QuotaSpecByName(nil, qs1.Name)
		assert.Nil(err)
		assert.NotNil(out)
	}

	// Try with an invalid token
	req.SecretID = invalidToken.SecretID
	{
		var resp structs.GenericResponse
		err := msgpackrpc.CallWithCodec(codec, "Quota.DeleteQuotaSpecs", req, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a valid token
	req.SecretID = validToken.SecretID
	{
		var resp structs.GenericResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.DeleteQuotaSpecs", req, &resp))
		assert.NotEqual(uint64(0), resp.Index)

		// Check we deleted the quota specs
		out, err := state.QuotaSpecByName(nil, qs1.Name)
		assert.Nil(err)
		assert.Nil(out)
	}

	// Try with a root token
	req.SecretID = root.SecretID
	{
		req.Names = []string{qs2.Name}
		var resp structs.GenericResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.DeleteQuotaSpecs", req, &resp))
		assert.NotEqual(uint64(0), resp.Index)

		// Check we deleted the quota specs
		out, err := state.QuotaSpecByName(nil, qs2.Name)
		assert.Nil(err)
		assert.Nil(out)
	}
}

func TestQuotaEndpoint_UpsertQuotaSpecs(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	qs1 := mock.QuotaSpec()
	qs2 := mock.QuotaSpec()

	req := &structs.QuotaSpecUpsertRequest{
		Quotas:       []*structs.QuotaSpec{qs1, qs2},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.GenericResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.UpsertQuotaSpecs", req, &resp))
	assert.NotEqual(uint64(0), resp.Index)

	// Check we created the quota specs
	out, err := s1.fsm.State().QuotaSpecByName(nil, qs1.Name)
	assert.Nil(err)
	assert.NotNil(out)

	out, err = s1.fsm.State().QuotaSpecByName(nil, qs2.Name)
	assert.Nil(err)
	assert.NotNil(out)
}

func TestQuotaEndpoint_UpsertQuotaSpecs_ACL(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	qs1 := mock.QuotaSpec()
	qs2 := mock.QuotaSpec()
	state := s1.fsm.State()

	// Create the policy and tokens
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))
	validToken := mock.CreatePolicyAndToken(t, state, 1003, "test-valid", mock.QuotaPolicy(acl.PolicyWrite))

	// Create the register request
	req := &structs.QuotaSpecUpsertRequest{
		Quotas:       []*structs.QuotaSpec{qs1},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Upsert the quotaspec without a token and expect failure
	{
		var resp structs.GenericResponse
		err := msgpackrpc.CallWithCodec(codec, "Quota.UpsertQuotaSpecs", req, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())

		// Check we did not create the quota specs
		out, err := state.QuotaSpecByName(nil, qs1.Name)
		assert.Nil(err)
		assert.Nil(out)
	}

	// Try with an invalid token
	req.SecretID = invalidToken.SecretID
	{
		var resp structs.GenericResponse
		err := msgpackrpc.CallWithCodec(codec, "Quota.UpsertQuotaSpecs", req, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())

		// Check we did not create the quota spec
		out, err := state.QuotaSpecByName(nil, qs1.Name)
		assert.Nil(err)
		assert.Nil(out)
	}

	// Try with a valid token
	req.SecretID = validToken.SecretID
	{
		var resp structs.GenericResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.UpsertQuotaSpecs", req, &resp))
		assert.NotEqual(uint64(0), resp.Index)

		// Check we created the quota specs
		out, err := state.QuotaSpecByName(nil, qs1.Name)
		assert.Nil(err)
		assert.NotNil(out)
	}

	// Try with a root token
	req.SecretID = root.SecretID
	{
		req.Quotas = []*structs.QuotaSpec{qs2}
		var resp structs.GenericResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.UpsertQuotaSpecs", req, &resp))
		assert.NotEqual(uint64(0), resp.Index)

		// Check we created the quota specs
		out, err := state.QuotaSpecByName(nil, qs2.Name)
		assert.Nil(err)
		assert.NotNil(out)
	}
}

func TestQuotaEndpoint_ListQuotaUsages(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	qs1 := mock.QuotaSpec()
	qs2 := mock.QuotaSpec()

	qs1.Name = "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9"
	qs2.Name = "aaaabbbb-3350-4b4b-d185-0e1992ed43e9"
	assert.Nil(s1.fsm.State().UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs1, qs2}))

	// Lookup the quota usages
	get := &structs.QuotaUsageListRequest{
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp structs.QuotaUsageListResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.ListQuotaUsages", get, &resp))
	assert.EqualValues(1000, resp.Index)
	assert.Len(resp.Usages, 2)

	// Lookup the quota usages by prefix
	get = &structs.QuotaUsageListRequest{
		QueryOptions: structs.QueryOptions{
			Region: "global",
			Prefix: "aaaabb",
		},
	}
	var resp2 structs.QuotaUsageListResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.ListQuotaUsages", get, &resp2))
	assert.EqualValues(1000, resp2.Index)
	assert.Len(resp2.Usages, 1)
}

func TestQuotaEndpoint_ListQuotaUsages_ACL(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	qs1 := mock.QuotaSpec()
	qs2 := mock.QuotaSpec()
	state := s1.fsm.State()

	qs1.Name = "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9"
	qs2.Name = "bbbbbbbb-3350-4b4b-d185-0e1992ed43e9"
	assert.Nil(state.UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs1, qs2}))

	// Create a namespace that attaches to one of the quotas
	ns1 := mock.Namespace()
	ns1.Quota = qs1.Name
	assert.Nil(state.UpsertNamespaces(1001, []*structs.Namespace{ns1}))

	validQuotaToken := mock.CreatePolicyAndToken(t, state, 1002, "test-quota-valid", mock.QuotaPolicy(acl.PolicyRead))
	validNamespaceToken := mock.CreatePolicyAndToken(t, state, 1004, "test-namespace-valid",
		mock.NamespacePolicy(ns1.Name, "", []string{acl.NamespaceCapabilityReadJob}))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1006, "test-invalid",
		mock.NamespacePolicy("invalid-namespace", "", []string{acl.NamespaceCapabilityReadJob}))

	get := &structs.QuotaUsageListRequest{
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Lookup the quota usages without a token and expect nothing
	{
		var resp structs.QuotaUsageListResponse
		err := msgpackrpc.CallWithCodec(codec, "Quota.ListQuotaUsages", get, &resp)
		assert.Nil(err)
		assert.Len(resp.Usages, 0)
	}

	// Try with an invalid token
	get.SecretID = invalidToken.SecretID
	{
		var resp structs.QuotaUsageListResponse
		err := msgpackrpc.CallWithCodec(codec, "Quota.ListQuotaUsages", get, &resp)
		assert.Nil(err)
		assert.Len(resp.Usages, 0)
	}

	// Try with a valid token for one
	get.SecretID = validNamespaceToken.SecretID
	{
		var resp structs.QuotaUsageListResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.ListQuotaUsages", get, &resp))
		assert.EqualValues(1000, resp.Index)
		assert.Len(resp.Usages, 1)
		assert.Equal(qs1.Name, resp.Usages[0].Name)
	}

	// Try with a valid token for all
	get.SecretID = validQuotaToken.SecretID
	{
		var resp structs.QuotaUsageListResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.ListQuotaUsages", get, &resp))
		assert.EqualValues(1000, resp.Index)
		assert.Len(resp.Usages, 2)
	}

	// Try with a root token
	get.SecretID = root.SecretID
	{
		var resp structs.QuotaUsageListResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.ListQuotaUsages", get, &resp))
		assert.EqualValues(1000, resp.Index)
		assert.Len(resp.Usages, 2)
	}
}

func TestQuotaEndpoint_ListQuotaUsages_Blocking(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the quotaspec
	qs := mock.QuotaSpec()

	// Upsert quotaspec triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.UpsertQuotaSpecs(200, []*structs.QuotaSpec{qs}))
	})

	req := &structs.QuotaUsageListRequest{
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
		},
	}
	start := time.Now()
	var resp structs.QuotaUsageListResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.ListQuotaUsages", req, &resp))

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	assert.EqualValues(200, resp.Index)
	assert.Len(resp.Usages, 1)

	// Quota spec deletion triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.DeleteQuotaSpecs(300, []string{qs.Name}))
	})

	req.MinQueryIndex = 200
	start = time.Now()
	var resp2 structs.QuotaUsageListResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.ListQuotaUsages", req, &resp2))

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	assert.EqualValues(300, resp2.Index)
	assert.Len(resp2.Usages, 0)
}

func TestQuotaEndpoint_GetQuotaUsage(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the quota spec
	qs := mock.QuotaSpec()
	s1.fsm.State().UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs})

	// Lookup the QuotaUsage
	get := &structs.QuotaUsageSpecificRequest{
		Name:         qs.Name,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp structs.SingleQuotaUsageResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.GetQuotaUsage", get, &resp))
	assert.EqualValues(1000, resp.Index)
	assert.Equal(qs.Name, resp.Usage.Name)

	// Lookup non-existing quota
	get.Name = uuid.Generate()
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.GetQuotaUsage", get, &resp))
	assert.EqualValues(1000, resp.Index)
	assert.Nil(resp.Usage)
}

func TestQuotaEndpoint_GetQuotaUsage_ACL(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	qs1 := mock.QuotaSpec()
	state := s1.fsm.State()
	s1.fsm.State().UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs1})

	// Create the policy and tokens
	validToken := mock.CreatePolicyAndToken(t, state, 1002, "test-valid",
		mock.QuotaPolicy(acl.PolicyRead))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NodePolicy(acl.PolicyRead))

	// Lookup the QuotaUsage
	get := &structs.QuotaUsageSpecificRequest{
		Name:         qs1.Name,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Lookup the quota without a token and expect failure
	{
		var resp structs.SingleQuotaUsageResponse
		err := msgpackrpc.CallWithCodec(codec, "Quota.GetQuotaUsage", get, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token
	get.SecretID = invalidToken.SecretID
	{
		var resp structs.SingleQuotaUsageResponse
		err := msgpackrpc.CallWithCodec(codec, "Quota.GetQuotaUsage", get, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a valid token
	get.SecretID = validToken.SecretID
	{
		var resp structs.SingleQuotaUsageResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.GetQuotaUsage", get, &resp))
		assert.EqualValues(1000, resp.Index)
		assert.Equal(qs1.Name, resp.Usage.Name)
	}

	// Try with a root token
	get.SecretID = root.SecretID
	{
		var resp structs.SingleQuotaUsageResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.GetQuotaUsage", get, &resp))
		assert.EqualValues(1000, resp.Index)
		assert.Equal(qs1.Name, resp.Usage.Name)
	}
}

func TestQuotaEndpoint_GetQuotaUsage_Blocking(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the quota specs
	qs1 := mock.QuotaSpec()
	qs2 := mock.QuotaSpec()

	// First create a quota spec
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.UpsertQuotaSpecs(100, []*structs.QuotaSpec{qs1}))
	})

	// Upsert the quota spec we are watching later
	time.AfterFunc(200*time.Millisecond, func() {
		assert.Nil(state.UpsertQuotaSpecs(200, []*structs.QuotaSpec{qs2}))
	})

	// Lookup the spec
	req := &structs.QuotaUsageSpecificRequest{
		Name: qs2.Name,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
		},
	}
	var resp structs.SingleQuotaUsageResponse
	start := time.Now()
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.GetQuotaUsage", req, &resp))
	assert.EqualValues(200, resp.Index)
	assert.NotNil(resp.Usage)
	assert.Equal(qs2.Name, resp.Usage.Name)

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}

	// Quota spec delete triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.DeleteQuotaSpecs(300, []string{qs2.Name}))
	})

	req.QueryOptions.MinQueryIndex = 250
	var resp2 structs.SingleQuotaUsageResponse
	start = time.Now()
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Quota.GetQuotaUsage", req, &resp2))
	assert.EqualValues(300, resp2.Index)
	assert.Nil(resp2.Usage)

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
}
