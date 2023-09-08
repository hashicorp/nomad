// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
)

func TestNamespaceEndpoint_GetNamespace(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	ns := mock.Namespace()
	s1.fsm.State().UpsertNamespaces(1000, []*structs.Namespace{ns})

	// Lookup the namespace
	get := &structs.NamespaceSpecificRequest{
		Name:         ns.Name,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp structs.SingleNamespaceResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Namespace.GetNamespace", get, &resp))
	assert.EqualValues(1000, resp.Index)
	assert.Equal(ns, resp.Namespace)

	// Lookup non-existing namespace
	get.Name = uuid.Generate()
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Namespace.GetNamespace", get, &resp))
	assert.EqualValues(1000, resp.Index)
	assert.Nil(resp.Namespace)
}

func TestNamespaceEndpoint_GetNamespace_ACL(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	ns1 := mock.Namespace()
	ns2 := mock.Namespace()
	state := s1.fsm.State()
	s1.fsm.State().UpsertNamespaces(1000, []*structs.Namespace{ns1, ns2})

	// Create the policy and tokens
	validToken := mock.CreatePolicyAndToken(t, state, 1002, "test-valid",
		mock.NamespacePolicy(ns1.Name, "", []string{acl.NamespaceCapabilityReadJob}))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(ns2.Name, "", []string{acl.NamespaceCapabilityReadJob}))

	get := &structs.NamespaceSpecificRequest{
		Name:         ns1.Name,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Lookup the namespace without a token and expect failure
	{
		var resp structs.SingleNamespaceResponse
		err := msgpackrpc.CallWithCodec(codec, "Namespace.GetNamespace", get, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token
	get.AuthToken = invalidToken.SecretID
	{
		var resp structs.SingleNamespaceResponse
		err := msgpackrpc.CallWithCodec(codec, "Namespace.GetNamespace", get, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a valid token
	get.AuthToken = validToken.SecretID
	{
		var resp structs.SingleNamespaceResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Namespace.GetNamespace", get, &resp))
		assert.EqualValues(1000, resp.Index)
		assert.Equal(ns1, resp.Namespace)
	}

	// Try with a root token
	get.AuthToken = root.SecretID
	{
		var resp structs.SingleNamespaceResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Namespace.GetNamespace", get, &resp))
		assert.EqualValues(1000, resp.Index)
		assert.Equal(ns1, resp.Namespace)
	}
}

func TestNamespaceEndpoint_GetNamespace_Blocking(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the namespaces
	ns1 := mock.Namespace()
	ns2 := mock.Namespace()

	// First create an namespace
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.UpsertNamespaces(100, []*structs.Namespace{ns1}))
	})

	// Upsert the namespace we are watching later
	time.AfterFunc(200*time.Millisecond, func() {
		assert.Nil(state.UpsertNamespaces(200, []*structs.Namespace{ns2}))
	})

	// Lookup the namespace
	req := &structs.NamespaceSpecificRequest{
		Name: ns2.Name,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
		},
	}
	var resp structs.SingleNamespaceResponse
	start := time.Now()
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Namespace.GetNamespace", req, &resp))
	assert.EqualValues(200, resp.Index)
	assert.NotNil(resp.Namespace)
	assert.Equal(ns2.Name, resp.Namespace.Name)

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}

	// Namespace delete triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.DeleteNamespaces(300, []string{ns2.Name}))
	})

	req.QueryOptions.MinQueryIndex = 250
	var resp2 structs.SingleNamespaceResponse
	start = time.Now()
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Namespace.GetNamespace", req, &resp2))
	assert.EqualValues(300, resp2.Index)
	assert.Nil(resp2.Namespace)

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
}

func TestNamespaceEndpoint_GetNamespaces(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	ns1 := mock.Namespace()
	ns2 := mock.Namespace()
	s1.fsm.State().UpsertNamespaces(1000, []*structs.Namespace{ns1, ns2})

	// Lookup the namespace
	get := &structs.NamespaceSetRequest{
		Namespaces:   []string{ns1.Name, ns2.Name},
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp structs.NamespaceSetResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Namespace.GetNamespaces", get, &resp))
	assert.EqualValues(1000, resp.Index)
	assert.Len(resp.Namespaces, 2)
	assert.Contains(resp.Namespaces, ns1.Name)
	assert.Contains(resp.Namespaces, ns2.Name)
}

func TestNamespaceEndpoint_GetNamespaces_ACL(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	ns1 := mock.Namespace()
	ns2 := mock.Namespace()
	state := s1.fsm.State()
	state.UpsertNamespaces(1000, []*structs.Namespace{ns1, ns2})

	// Create the policy and tokens
	validToken := mock.CreatePolicyAndToken(t, state, 1002, "test-valid",
		mock.NamespacePolicy(ns1.Name, "", []string{acl.NamespaceCapabilityReadJob}))

	// Lookup the namespace
	get := &structs.NamespaceSetRequest{
		Namespaces:   []string{ns1.Name, ns2.Name},
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Lookup the namespaces without a token and expect a failure
	{
		var resp structs.NamespaceSetResponse
		assert.NotNil(msgpackrpc.CallWithCodec(codec, "Namespace.GetNamespaces", get, &resp))
	}

	// Try with an non-management token
	get.AuthToken = validToken.SecretID
	{
		var resp structs.NamespaceSetResponse
		assert.NotNil(msgpackrpc.CallWithCodec(codec, "Namespace.GetNamespaces", get, &resp))
	}

	// Try with a root token
	get.AuthToken = root.SecretID
	{
		var resp structs.NamespaceSetResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Namespace.GetNamespaces", get, &resp))
		assert.EqualValues(1000, resp.Index)
		assert.Len(resp.Namespaces, 2)
		assert.Contains(resp.Namespaces, ns1.Name)
		assert.Contains(resp.Namespaces, ns2.Name)
	}
}

func TestNamespaceEndpoint_GetNamespaces_Blocking(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the namespaces
	ns1 := mock.Namespace()
	ns2 := mock.Namespace()

	// First create an namespace
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.UpsertNamespaces(100, []*structs.Namespace{ns1}))
	})

	// Upsert the namespace we are watching later
	time.AfterFunc(200*time.Millisecond, func() {
		assert.Nil(state.UpsertNamespaces(200, []*structs.Namespace{ns2}))
	})

	// Lookup the namespace
	req := &structs.NamespaceSetRequest{
		Namespaces: []string{ns2.Name},
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
		},
	}
	var resp structs.NamespaceSetResponse
	start := time.Now()
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Namespace.GetNamespaces", req, &resp))
	assert.EqualValues(200, resp.Index)
	assert.Len(resp.Namespaces, 1)
	assert.Contains(resp.Namespaces, ns2.Name)

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}

	// Namespace delete triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.DeleteNamespaces(300, []string{ns2.Name}))
	})

	req.QueryOptions.MinQueryIndex = 250
	var resp2 structs.NamespaceSetResponse
	start = time.Now()
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Namespace.GetNamespaces", req, &resp2))
	assert.EqualValues(300, resp2.Index)
	assert.Empty(resp2.Namespaces)

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
}

func TestNamespaceEndpoint_List(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	ns1 := mock.Namespace()
	ns2 := mock.Namespace()

	ns1.Name = "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9"
	ns2.Name = "aaaabbbb-3350-4b4b-d185-0e1992ed43e9"
	assert.Nil(s1.fsm.State().UpsertNamespaces(1000, []*structs.Namespace{ns1, ns2}))

	// Lookup the namespaces
	get := &structs.NamespaceListRequest{
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp structs.NamespaceListResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Namespace.ListNamespaces", get, &resp))
	assert.EqualValues(1000, resp.Index)
	assert.Len(resp.Namespaces, 3)

	// Lookup the namespaces by prefix
	get = &structs.NamespaceListRequest{
		QueryOptions: structs.QueryOptions{
			Region: "global",
			Prefix: "aaaabb",
		},
	}
	var resp2 structs.NamespaceListResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Namespace.ListNamespaces", get, &resp2))
	assert.EqualValues(1000, resp2.Index)
	assert.Len(resp2.Namespaces, 1)
}

func TestNamespaceEndpoint_List_ACL(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	ns1 := mock.Namespace()
	ns2 := mock.Namespace()
	state := s1.fsm.State()

	ns1.Name = "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9"
	ns2.Name = "bbbbbbbb-3350-4b4b-d185-0e1992ed43e9"
	assert.Nil(s1.fsm.State().UpsertNamespaces(1000, []*structs.Namespace{ns1, ns2}))

	validDefToken := mock.CreatePolicyAndToken(t, state, 1001, "test-def-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadFS}))
	validMultiToken := mock.CreatePolicyAndToken(t, state, 1002, "test-multi-valid", fmt.Sprintf("%s\n%s",
		mock.NamespacePolicy(ns1.Name, "", []string{acl.NamespaceCapabilityReadJob}),
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob})))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy("invalid-namespace", "", []string{acl.NamespaceCapabilityReadJob}))

	get := &structs.NamespaceListRequest{
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Lookup the namespaces without a token and expect a failure
	{
		var resp structs.NamespaceListResponse
		err := msgpackrpc.CallWithCodec(codec, "Namespace.ListNamespaces", get, &resp)
		assert.Nil(err)
		assert.Len(resp.Namespaces, 0)
	}

	// Try with an invalid token
	get.AuthToken = invalidToken.SecretID
	{
		var resp structs.NamespaceListResponse
		err := msgpackrpc.CallWithCodec(codec, "Namespace.ListNamespaces", get, &resp)
		assert.Nil(err)
		assert.Len(resp.Namespaces, 0)
	}

	// Try with a valid token for one
	get.AuthToken = validDefToken.SecretID
	{
		var resp structs.NamespaceListResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Namespace.ListNamespaces", get, &resp))
		assert.EqualValues(1000, resp.Index)
		assert.Len(resp.Namespaces, 1)
	}

	// Try with a valid token for two
	get.AuthToken = validMultiToken.SecretID
	{
		var resp structs.NamespaceListResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Namespace.ListNamespaces", get, &resp))
		assert.EqualValues(1000, resp.Index)
		assert.Len(resp.Namespaces, 2)
	}

	// Try with a root token
	get.AuthToken = root.SecretID
	{
		var resp structs.NamespaceListResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Namespace.ListNamespaces", get, &resp))
		assert.EqualValues(1000, resp.Index)
		assert.Len(resp.Namespaces, 3)
	}
}

func TestNamespaceEndpoint_List_Blocking(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the namespace
	ns := mock.Namespace()

	// Upsert namespace triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.UpsertNamespaces(200, []*structs.Namespace{ns}))
	})

	req := &structs.NamespaceListRequest{
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
		},
	}
	start := time.Now()
	var resp structs.NamespaceListResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Namespace.ListNamespaces", req, &resp))

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	assert.EqualValues(200, resp.Index)
	assert.Len(resp.Namespaces, 2)

	// Namespace deletion triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.DeleteNamespaces(300, []string{ns.Name}))
	})

	req.MinQueryIndex = 200
	start = time.Now()
	var resp2 structs.NamespaceListResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Namespace.ListNamespaces", req, &resp2))

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	assert.EqualValues(300, resp2.Index)
	assert.Len(resp2.Namespaces, 1)
}

func TestNamespaceEndpoint_DeleteNamespaces(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	ns1 := mock.Namespace()
	ns2 := mock.Namespace()
	s1.fsm.State().UpsertNamespaces(1000, []*structs.Namespace{ns1, ns2})

	// Lookup the namespaces
	req := &structs.NamespaceDeleteRequest{
		Namespaces:   []string{ns1.Name, ns2.Name},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.GenericResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Namespace.DeleteNamespaces", req, &resp))
	assert.NotEqual(uint64(0), resp.Index)
}

func TestNamespaceEndpoint_DeleteNamespaces_NonTerminal_Local(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	ns1 := mock.Namespace()
	ns2 := mock.Namespace()
	s1.fsm.State().UpsertNamespaces(1000, []*structs.Namespace{ns1, ns2})

	// Create a job in one
	j := mock.Job()
	j.Namespace = ns1.Name
	assert.Nil(s1.fsm.State().UpsertJob(structs.MsgTypeTestSetup, 1001, nil, j))

	// Lookup the namespaces
	req := &structs.NamespaceDeleteRequest{
		Namespaces:   []string{ns1.Name, ns2.Name},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "Namespace.DeleteNamespaces", req, &resp)
	if assert.NotNil(err) {
		assert.Contains(err.Error(), "has non-terminal jobs")
	}
}

func TestNamespaceEndpoint_DeleteNamespaces_NonTerminal_Federated_ACL(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.Region = "region1"
		c.AuthoritativeRegion = "region1"
		c.ACLEnabled = true
	})
	defer cleanupS1()
	s2, _, cleanupS2 := TestACLServer(t, func(c *Config) {
		c.Region = "region2"
		c.AuthoritativeRegion = "region1"
		c.ACLEnabled = true
		c.ReplicationBackoff = 20 * time.Millisecond
		c.ReplicationToken = root.SecretID
	})
	defer cleanupS2()
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)
	codec := rpcClient(t, s1)

	// Create the register request
	ns1 := mock.Namespace()
	s1.fsm.State().UpsertNamespaces(1000, []*structs.Namespace{ns1})

	testutil.WaitForResult(func() (bool, error) {
		state := s2.State()
		out, err := state.NamespaceByName(nil, ns1.Name)
		return out != nil, err
	}, func(err error) {
		t.Fatalf("should replicate namespace")
	})

	// Create a job in the namespace on the non-authority
	j := mock.Job()
	j.Namespace = ns1.Name
	assert.Nil(s2.fsm.State().UpsertJob(structs.MsgTypeTestSetup, 1001, nil, j))

	// Delete the namespaces without the correct permissions
	req := &structs.NamespaceDeleteRequest{
		Namespaces: []string{ns1.Name},
		WriteRequest: structs.WriteRequest{
			Region: "global",
		},
	}
	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "Namespace.DeleteNamespaces", req, &resp)
	if assert.NotNil(err) {
		assert.EqualError(err, structs.ErrPermissionDenied.Error())
	}

	// Try with a auth token
	req.AuthToken = root.SecretID
	var resp2 structs.GenericResponse
	err = msgpackrpc.CallWithCodec(codec, "Namespace.DeleteNamespaces", req, &resp2)
	if assert.NotNil(err) {
		assert.Contains(err.Error(), "has non-terminal jobs")
	}
}

func TestNamespaceEndpoint_DeleteNamespaces_ACL(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	ns1 := mock.Namespace()
	ns2 := mock.Namespace()
	state := s1.fsm.State()
	s1.fsm.State().UpsertNamespaces(1000, []*structs.Namespace{ns1, ns2})

	// Create the policy and tokens
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	req := &structs.NamespaceDeleteRequest{
		Namespaces:   []string{ns1.Name, ns2.Name},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Delete namespaces without a token and expect failure
	{
		var resp structs.GenericResponse
		err := msgpackrpc.CallWithCodec(codec, "Namespace.DeleteNamespaces", req, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())

		// Check we did not delete the namespaces
		out, err := s1.fsm.State().NamespaceByName(nil, ns1.Name)
		assert.Nil(err)
		assert.NotNil(out)

		out, err = s1.fsm.State().NamespaceByName(nil, ns2.Name)
		assert.Nil(err)
		assert.NotNil(out)
	}

	// Try with an invalid token
	req.AuthToken = invalidToken.SecretID
	{
		var resp structs.GenericResponse
		err := msgpackrpc.CallWithCodec(codec, "Namespace.DeleteNamespaces", req, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())

		// Check we did not delete the namespaces
		out, err := s1.fsm.State().NamespaceByName(nil, ns1.Name)
		assert.Nil(err)
		assert.NotNil(out)

		out, err = s1.fsm.State().NamespaceByName(nil, ns2.Name)
		assert.Nil(err)
		assert.NotNil(out)
	}

	// Try with a root token
	req.AuthToken = root.SecretID
	{
		var resp structs.GenericResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Namespace.DeleteNamespaces", req, &resp))
		assert.NotEqual(uint64(0), resp.Index)

		// Check we deleted the namespaces
		out, err := s1.fsm.State().NamespaceByName(nil, ns1.Name)
		assert.Nil(err)
		assert.Nil(out)

		out, err = s1.fsm.State().NamespaceByName(nil, ns2.Name)
		assert.Nil(err)
		assert.Nil(out)
	}
}

func TestNamespaceEndpoint_DeleteNamespaces_Default(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Delete the default namespace
	req := &structs.NamespaceDeleteRequest{
		Namespaces:   []string{structs.DefaultNamespace},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.GenericResponse
	assert.NotNil(msgpackrpc.CallWithCodec(codec, "Namespace.DeleteNamespaces", req, &resp))
}

func TestNamespaceEndpoint_UpsertNamespaces(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	ns1 := mock.Namespace()
	ns2 := mock.Namespace()

	// Lookup the namespaces
	req := &structs.NamespaceUpsertRequest{
		Namespaces:   []*structs.Namespace{ns1, ns2},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.GenericResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Namespace.UpsertNamespaces", req, &resp))
	assert.NotEqual(uint64(0), resp.Index)

	// Check we created the namespaces
	out, err := s1.fsm.State().NamespaceByName(nil, ns1.Name)
	assert.Nil(err)
	assert.NotNil(out)

	out, err = s1.fsm.State().NamespaceByName(nil, ns2.Name)
	assert.Nil(err)
	assert.NotNil(out)
}

func TestNamespaceEndpoint_UpsertNamespaces_ACL(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	ns1 := mock.Namespace()
	ns2 := mock.Namespace()
	state := s1.fsm.State()

	// Create the policy and tokens
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	// Create the register request
	req := &structs.NamespaceUpsertRequest{
		Namespaces:   []*structs.Namespace{ns1, ns2},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Upsert the namespace without a token and expect failure
	{
		var resp structs.GenericResponse
		err := msgpackrpc.CallWithCodec(codec, "Namespace.UpsertNamespaces", req, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())

		// Check we did not create the namespaces
		out, err := s1.fsm.State().NamespaceByName(nil, ns1.Name)
		assert.Nil(err)
		assert.Nil(out)

		out, err = s1.fsm.State().NamespaceByName(nil, ns2.Name)
		assert.Nil(err)
		assert.Nil(out)
	}

	// Try with an invalid token
	req.AuthToken = invalidToken.SecretID
	{
		var resp structs.GenericResponse
		err := msgpackrpc.CallWithCodec(codec, "Namespace.UpsertNamespaces", req, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())

		// Check we did not create the namespaces
		out, err := s1.fsm.State().NamespaceByName(nil, ns1.Name)
		assert.Nil(err)
		assert.Nil(out)

		out, err = s1.fsm.State().NamespaceByName(nil, ns2.Name)
		assert.Nil(err)
		assert.Nil(out)
	}

	// Try with a bogus token
	req.AuthToken = uuid.Generate()
	{
		var resp structs.GenericResponse
		err := msgpackrpc.CallWithCodec(codec, "Namespace.UpsertNamespaces", req, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())

		// Check we did not create the namespaces
		out, err := s1.fsm.State().NamespaceByName(nil, ns1.Name)
		assert.Nil(err)
		assert.Nil(out)

		out, err = s1.fsm.State().NamespaceByName(nil, ns2.Name)
		assert.Nil(err)
		assert.Nil(out)
	}

	// Try with a root token
	req.AuthToken = root.SecretID
	{
		var resp structs.GenericResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Namespace.UpsertNamespaces", req, &resp))
		assert.NotEqual(uint64(0), resp.Index)

		// Check we created the namespaces
		out, err := s1.fsm.State().NamespaceByName(nil, ns1.Name)
		assert.Nil(err)
		assert.NotNil(out)

		out, err = s1.fsm.State().NamespaceByName(nil, ns2.Name)
		assert.Nil(err)
		assert.NotNil(out)
	}
}
