// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"path"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestAuthenticate_mTLS(t *testing.T) {
	ci.Parallel(t)

	// Set up a cluster with mTLS and ACLs

	dir := t.TempDir()

	tlsCfg := &config.TLSConfig{
		EnableHTTP:           true,
		EnableRPC:            true,
		VerifyServerHostname: true,
		CAFile:               "../helper/tlsutil/testdata/nomad-agent-ca.pem",
		CertFile:             "../helper/tlsutil/testdata/regionFoo-server-nomad.pem",
		KeyFile:              "../helper/tlsutil/testdata/regionFoo-server-nomad-key.pem",
	}
	clientTLSCfg := tlsCfg.Copy()
	clientTLSCfg.CertFile = "../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
	clientTLSCfg.KeyFile = "../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"

	setCfg := func(name string, bootstrapExpect int) func(*Config) {
		return func(c *Config) {
			c.Region = "regionFoo"
			c.AuthoritativeRegion = "regionFoo"
			c.ACLEnabled = true
			c.BootstrapExpect = bootstrapExpect
			c.NumSchedulers = 0
			c.DevMode = false
			c.DataDir = path.Join(dir, name)
			c.TLSConfig = tlsCfg
		}
	}

	leader, cleanupLeader := TestServer(t, setCfg("node1", 1))
	defer cleanupLeader()
	testutil.WaitForLeader(t, leader.RPC)

	follower, cleanupFollower := TestServer(t, setCfg("node2", 0))
	defer cleanupFollower()

	TestJoin(t, leader, follower)
	testutil.WaitForLeader(t, leader.RPC)

	testutil.Wait(t, func() (bool, error) {
		keyset, err := follower.encrypter.activeKeySet()
		return keyset != nil, err
	})

	rootToken := uuid.Generate()
	var bootstrapResp *structs.ACLTokenUpsertResponse

	codec := rpcClientWithTLS(t, follower, tlsCfg)
	must.NoError(t, msgpackrpc.CallWithCodec(codec,
		"ACL.Bootstrap", &structs.ACLTokenBootstrapRequest{
			BootstrapSecret: rootToken,
			WriteRequest:    structs.WriteRequest{Region: "regionFoo"},
		}, &bootstrapResp))
	must.NotNil(t, bootstrapResp)
	must.Len(t, 1, bootstrapResp.Tokens)
	rootAccessor := bootstrapResp.Tokens[0].AccessorID

	// create some ACL tokens directly into raft so we can bypass RPC validation
	// around expiration times

	token1 := mock.ACLToken()
	token2 := mock.ACLToken()
	expireTime := time.Now().Add(time.Second * -10)
	token2.ExpirationTime = &expireTime

	_, _, err := leader.raftApply(structs.ACLTokenUpsertRequestType,
		&structs.ACLTokenUpsertRequest{Tokens: []*structs.ACLToken{token1, token2}})
	must.NoError(t, err)

	// create a node so we can test client RPCs

	node := mock.Node()
	nodeRegisterReq := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "regionFoo"},
	}
	var nodeRegisterResp structs.NodeUpdateResponse

	must.NoError(t, msgpackrpc.CallWithCodec(codec,
		"Node.Register", nodeRegisterReq, &nodeRegisterResp))
	must.NotNil(t, bootstrapResp)

	// create some allocations so we can test WorkloadIdentity claims. we'll
	// create directly into raft so we can bypass RPC validation and the whole
	// eval, plan, etc. workflow.
	job := mock.Job()

	_, _, err = leader.raftApply(structs.JobRegisterRequestType,
		&structs.JobRegisterRequest{Job: job})
	must.NoError(t, err)

	alloc1 := mock.Alloc()
	alloc1.NodeID = node.ID
	alloc1.ClientStatus = structs.AllocClientStatusFailed
	alloc1.Job = job
	alloc1.JobID = job.ID

	alloc2 := mock.Alloc()
	alloc2.NodeID = node.ID
	alloc2.Job = job
	alloc2.JobID = job.ID
	alloc2.ClientStatus = structs.AllocClientStatusRunning

	claims1 := structs.NewIdentityClaims(job, alloc1, "web", alloc1.LookupTask("web").Identity, time.Now())
	claims1Token, _, err := leader.encrypter.SignClaims(claims1)
	must.NoError(t, err, must.Sprint("could not sign claims"))

	claims2 := structs.NewIdentityClaims(job, alloc2, "web", alloc2.LookupTask("web").Identity, time.Now())
	claims2Token, _, err := leader.encrypter.SignClaims(claims2)
	must.NoError(t, err, must.Sprint("could not sign claims"))

	planReq := &structs.ApplyPlanResultsRequest{
		AllocUpdateRequest: structs.AllocUpdateRequest{
			Alloc: []*structs.Allocation{alloc1, alloc2},
			Job:   job,
		},
	}
	_, _, err = leader.raftApply(structs.ApplyPlanResultsRequestType, planReq)
	must.NoError(t, err)

	testutil.WaitForResult(func() (bool, error) {
		store := follower.fsm.State()
		alloc, err := store.AllocByID(nil, alloc1.ID)
		return alloc != nil, err
	}, func(err error) {
		t.Fatalf("alloc was not replicated via raft: %v", err) // should never happen
	})

	testCases := []struct {
		name           string
		tlsCfg         *config.TLSConfig
		stale          bool
		testToken      string
		expectAccessor string
		expectClientID string
		expectAllocID  string
		expectTLSName  string
		expectIP       string
		expectErr      string
		expectIDKey    string
		sendFromPeer   *Server
	}{
		{
			name:           "root token",
			tlsCfg:         clientTLSCfg, // TODO: this is a mixed use cert
			testToken:      rootToken,
			expectAccessor: rootAccessor,
			expectIDKey:    fmt.Sprintf("token:%s", rootAccessor),
		},
		{
			name:           "from peer to leader without token", // ex. Eval.Dequeue
			tlsCfg:         tlsCfg,
			expectTLSName:  "server.regionFoo.nomad",
			expectAccessor: "anonymous",
			expectIP:       follower.GetConfig().RPCAddr.IP.String(),
			sendFromPeer:   follower,
			expectIDKey:    "token:anonymous",
		},
		{
			// note: this test is somewhat bogus because under test all the
			// servers share the same IP address with the RPC client
			name:           "anonymous forwarded from peer to leader",
			tlsCfg:         tlsCfg,
			expectAccessor: "anonymous",
			expectTLSName:  "server.regionFoo.nomad",
			expectIP:       "127.0.0.1",
			expectIDKey:    "token:anonymous",
		},
		{
			name:          "invalid token",
			tlsCfg:        clientTLSCfg,
			testToken:     uuid.Generate(),
			expectTLSName: "server.regionFoo.nomad",
			expectIP:      follower.GetConfig().RPCAddr.IP.String(),
			expectIDKey:   "server.regionFoo.nomad:127.0.0.1",
			expectErr:     "rpc error: Permission denied",
		},
		{
			name:           "from peer to leader with leader ACL", // ex. core job GC
			tlsCfg:         tlsCfg,
			testToken:      leader.getLeaderAcl(),
			expectTLSName:  "server.regionFoo.nomad",
			expectAccessor: "leader",
			expectIP:       follower.GetConfig().RPCAddr.IP.String(),
			sendFromPeer:   follower,
			expectIDKey:    "token:leader",
		},
		{
			name:           "from client", // ex. Node.GetAllocs
			tlsCfg:         clientTLSCfg,
			testToken:      node.SecretID,
			expectClientID: node.ID,
			expectIDKey:    fmt.Sprintf("client:%s", node.ID),
		},
		{
			name:           "from client missing secret", // ex. Node.Register
			tlsCfg:         clientTLSCfg,
			expectAccessor: "anonymous",
			expectTLSName:  "server.regionFoo.nomad",
			expectIP:       follower.GetConfig().RPCAddr.IP.String(),
		},
		{
			name:      "from failed workload", // ex. Variables.List
			tlsCfg:    clientTLSCfg,
			testToken: claims1Token,
			expectErr: "rpc error: allocation is terminal",
		},
		{
			name:          "from running workload", // ex. Variables.List
			tlsCfg:        clientTLSCfg,
			testToken:     claims2Token,
			expectAllocID: alloc2.ID,
			expectIDKey:   fmt.Sprintf("alloc:%s", alloc2.ID),
		},
		{
			name:           "valid user token",
			tlsCfg:         clientTLSCfg,
			testToken:      token1.SecretID,
			expectAccessor: token1.AccessorID,
			expectIDKey:    fmt.Sprintf("token:%s", token1.AccessorID),
		},
		{
			name:      "expired user token",
			tlsCfg:    clientTLSCfg,
			testToken: token2.SecretID,
			expectErr: "rpc error: ACL token expired",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			req := &structs.GenericRequest{
				QueryOptions: structs.QueryOptions{
					Region:     "regionFoo",
					AllowStale: tc.stale,
					AuthToken:  tc.testToken,
				},
			}
			var resp structs.ACLWhoAmIResponse
			var err error

			if tc.sendFromPeer != nil {
				aclEndpoint := NewACLEndpoint(tc.sendFromPeer, nil)
				err = aclEndpoint.WhoAmI(req, &resp)
			} else {
				err = msgpackrpc.CallWithCodec(codec, "ACL.WhoAmI", req, &resp)
			}

			if tc.expectErr != "" {
				must.EqError(t, err, tc.expectErr)
				return
			}

			must.NoError(t, err)
			must.NotNil(t, resp)
			must.NotNil(t, resp.Identity)

			if tc.expectIDKey != "" {
				must.Eq(t, tc.expectIDKey, resp.Identity.String(),
					must.Sprintf("expected identity key for metrics to match"))
			}

			if tc.expectAccessor != "" {
				must.NotNil(t, resp.Identity.ACLToken, must.Sprint("expected ACL token"))
				test.Eq(t, tc.expectAccessor, resp.Identity.ACLToken.AccessorID,
					test.Sprint("expected ACL token accessor ID"))
			}

			test.Eq(t, tc.expectClientID, resp.Identity.ClientID,
				test.Sprint("expected client ID"))

			if tc.expectAllocID != "" {
				must.NotNil(t, resp.Identity.Claims, must.Sprint("expected claims"))
				test.Eq(t, tc.expectAllocID, resp.Identity.Claims.AllocationID,
					test.Sprint("expected workload identity"))
			}

			test.Eq(t, tc.expectTLSName, resp.Identity.TLSName, test.Sprint("expected TLS name"))

			if tc.expectIP == "" {
				test.Nil(t, resp.Identity.RemoteIP, test.Sprint("expected no remote IP"))
			} else {
				test.Eq(t, tc.expectIP, resp.Identity.RemoteIP.String())
			}

		})
	}
}

func TestResolveACLToken(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name   string
		testFn func()
	}{
		{
			name: "leader token",
			testFn: func() {

				testServer, _, testServerCleanup := TestACLServer(t, nil)
				defer testServerCleanup()
				testutil.WaitForLeader(t, testServer.RPC)

				// Check the leader ACL token is correctly set.
				leaderACL := testServer.getLeaderAcl()
				require.NotEmpty(t, leaderACL)

				// Resolve the token and ensure it's a management token.
				aclResp, err := testServer.ResolveToken(leaderACL)
				require.NoError(t, err)
				require.NotNil(t, aclResp)
				require.True(t, aclResp.IsManagement())
			},
		},
		{
			name: "anonymous token",
			testFn: func() {

				testServer, _, testServerCleanup := TestACLServer(t, nil)
				defer testServerCleanup()
				testutil.WaitForLeader(t, testServer.RPC)

				// Call the function with an empty input secret ID which is
				// classed as representing anonymous access in clusters with
				// ACLs enabled.
				aclResp, err := testServer.ResolveToken("")
				require.NoError(t, err)
				require.NotNil(t, aclResp)
				require.False(t, aclResp.IsManagement())
			},
		},
		{
			name: "token not found",
			testFn: func() {

				testServer, _, testServerCleanup := TestACLServer(t, nil)
				defer testServerCleanup()
				testutil.WaitForLeader(t, testServer.RPC)

				// Call the function with randomly generated secret ID which
				// does not exist within state.
				aclResp, err := testServer.ResolveToken(uuid.Generate())
				require.Equal(t, structs.ErrTokenNotFound, err)
				require.Nil(t, aclResp)
			},
		},
		{
			name: "token expired",
			testFn: func() {

				testServer, _, testServerCleanup := TestACLServer(t, nil)
				defer testServerCleanup()
				testutil.WaitForLeader(t, testServer.RPC)

				// Create a mock token with an expiration time long in the
				// past, and upsert.
				token := mock.ACLToken()
				token.ExpirationTime = pointer.Of(time.Date(
					1970, time.January, 1, 0, 0, 0, 0, time.UTC))

				err := testServer.State().UpsertACLTokens(
					structs.MsgTypeTestSetup, 10, []*structs.ACLToken{token})
				require.NoError(t, err)

				// Perform the function call which should result in finding the
				// token has expired.
				aclResp, err := testServer.ResolveToken(uuid.Generate())
				require.Equal(t, structs.ErrTokenNotFound, err)
				require.Nil(t, aclResp)
			},
		},
		{
			name: "management token",
			testFn: func() {

				testServer, _, testServerCleanup := TestACLServer(t, nil)
				defer testServerCleanup()
				testutil.WaitForLeader(t, testServer.RPC)

				// Generate a management token and upsert this.
				managementToken := mock.ACLToken()
				managementToken.Type = structs.ACLManagementToken
				managementToken.Policies = nil

				err := testServer.State().UpsertACLTokens(
					structs.MsgTypeTestSetup, 10, []*structs.ACLToken{managementToken})
				require.NoError(t, err)

				// Resolve the token and check that we received a management
				// ACL.
				aclResp, err := testServer.ResolveToken(managementToken.SecretID)
				require.Nil(t, err)
				require.NotNil(t, aclResp)
				require.True(t, aclResp.IsManagement())
				require.Equal(t, acl.ManagementACL, aclResp)
			},
		},
		{
			name: "client token with policies only",
			testFn: func() {

				testServer, _, testServerCleanup := TestACLServer(t, nil)
				defer testServerCleanup()
				testutil.WaitForLeader(t, testServer.RPC)

				// Generate a client token with associated policies and upsert
				// these.
				policy1 := mock.ACLPolicy()
				policy2 := mock.ACLPolicy()
				err := testServer.State().UpsertACLPolicies(
					structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2})

				clientToken := mock.ACLToken()
				clientToken.Policies = []string{policy1.Name, policy2.Name}
				err = testServer.State().UpsertACLTokens(
					structs.MsgTypeTestSetup, 20, []*structs.ACLToken{clientToken})
				require.NoError(t, err)

				// Resolve the token and check that we received a client
				// ACL with appropriate permissions.
				aclResp, err := testServer.ResolveToken(clientToken.SecretID)
				require.Nil(t, err)
				require.NotNil(t, aclResp)
				require.False(t, aclResp.IsManagement())

				allowed := aclResp.AllowNamespaceOperation("default", acl.NamespaceCapabilityListJobs)
				require.True(t, allowed)
				allowed = aclResp.AllowNamespaceOperation("other", acl.NamespaceCapabilityListJobs)
				require.False(t, allowed)

				// Resolve the same token again and ensure we get the same
				// result.
				aclResp2, err := testServer.ResolveToken(clientToken.SecretID)
				require.Nil(t, err)
				require.NotNil(t, aclResp2)
				require.Equal(t, aclResp, aclResp2)

				// Bust the cache by upserting the policy
				err = testServer.State().UpsertACLPolicies(
					structs.MsgTypeTestSetup, 30, []*structs.ACLPolicy{policy1})
				require.Nil(t, err)

				// Resolve the same token again, should get different value
				aclResp3, err := testServer.ResolveToken(clientToken.SecretID)
				require.Nil(t, err)
				require.NotNil(t, aclResp3)
				require.NotEqual(t, aclResp2, aclResp3)
			},
		},
		{
			name: "client token with roles only",
			testFn: func() {

				testServer, _, testServerCleanup := TestACLServer(t, nil)
				defer testServerCleanup()
				testutil.WaitForLeader(t, testServer.RPC)

				// Create a client token that only has a link to a role.
				policy1 := mock.ACLPolicy()
				policy2 := mock.ACLPolicy()
				err := testServer.State().UpsertACLPolicies(
					structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2})

				aclRole := mock.ACLRole()
				aclRole.Policies = []*structs.ACLRolePolicyLink{
					{Name: policy1.Name},
					{Name: policy2.Name},
				}
				err = testServer.State().UpsertACLRoles(
					structs.MsgTypeTestSetup, 30, []*structs.ACLRole{aclRole}, false)
				require.NoError(t, err)

				clientToken := mock.ACLToken()
				clientToken.Policies = []string{}
				clientToken.Roles = []*structs.ACLTokenRoleLink{{ID: aclRole.ID}}
				err = testServer.State().UpsertACLTokens(
					structs.MsgTypeTestSetup, 30, []*structs.ACLToken{clientToken})
				require.NoError(t, err)

				// Resolve the token and check that we received a client
				// ACL with appropriate permissions.
				aclResp, err := testServer.ResolveToken(clientToken.SecretID)
				require.Nil(t, err)
				require.NotNil(t, aclResp)
				require.False(t, aclResp.IsManagement())

				allowed := aclResp.AllowNamespaceOperation("default", acl.NamespaceCapabilityListJobs)
				require.True(t, allowed)
				allowed = aclResp.AllowNamespaceOperation("other", acl.NamespaceCapabilityListJobs)
				require.False(t, allowed)

				// Remove the policies from the ACL role and ensure the resolution
				// permissions are updated.
				aclRole.Policies = []*structs.ACLRolePolicyLink{}
				err = testServer.State().UpsertACLRoles(
					structs.MsgTypeTestSetup, 40, []*structs.ACLRole{aclRole}, false)
				require.NoError(t, err)

				aclResp, err = testServer.ResolveToken(clientToken.SecretID)
				require.Nil(t, err)
				require.NotNil(t, aclResp)
				require.False(t, aclResp.IsManagement())
				require.False(t, aclResp.AllowNamespaceOperation("default", acl.NamespaceCapabilityListJobs))
			},
		},
		{
			name: "client with roles and policies",
			testFn: func() {

				testServer, _, testServerCleanup := TestACLServer(t, nil)
				defer testServerCleanup()
				testutil.WaitForLeader(t, testServer.RPC)

				// Generate two policies, each with a different namespace
				// permission set.
				policy1 := &structs.ACLPolicy{
					Name:        "policy-" + uuid.Generate(),
					Rules:       `namespace "platform" { policy = "write"}`,
					CreateIndex: 10,
					ModifyIndex: 10,
				}
				policy1.SetHash()
				policy2 := &structs.ACLPolicy{
					Name:        "policy-" + uuid.Generate(),
					Rules:       `namespace "web" { policy = "write"}`,
					CreateIndex: 10,
					ModifyIndex: 10,
				}
				policy2.SetHash()

				err := testServer.State().UpsertACLPolicies(
					structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2})
				require.NoError(t, err)

				// Create a role which references the policy that has access to
				// the web namespace.
				aclRole := mock.ACLRole()
				aclRole.Policies = []*structs.ACLRolePolicyLink{{Name: policy2.Name}}
				err = testServer.State().UpsertACLRoles(
					structs.MsgTypeTestSetup, 20, []*structs.ACLRole{aclRole}, false)
				require.NoError(t, err)

				// Create a token which references the policy and role.
				clientToken := mock.ACLToken()
				clientToken.Policies = []string{policy1.Name}
				clientToken.Roles = []*structs.ACLTokenRoleLink{{ID: aclRole.ID}}
				err = testServer.State().UpsertACLTokens(
					structs.MsgTypeTestSetup, 30, []*structs.ACLToken{clientToken})
				require.NoError(t, err)

				// Resolve the token and check that we received a client
				// ACL with appropriate permissions.
				aclResp, err := testServer.ResolveToken(clientToken.SecretID)
				require.Nil(t, err)
				require.NotNil(t, aclResp)
				require.False(t, aclResp.IsManagement())

				allowed := aclResp.AllowNamespaceOperation("platform", acl.NamespaceCapabilityListJobs)
				require.True(t, allowed)
				allowed = aclResp.AllowNamespaceOperation("web", acl.NamespaceCapabilityListJobs)
				require.True(t, allowed)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.testFn()
		})
	}
}

func TestResolveSecretToken(t *testing.T) {
	ci.Parallel(t)

	testServer, _, testServerCleanup := TestACLServer(t, nil)
	defer testServerCleanup()
	testutil.WaitForLeader(t, testServer.RPC)

	testCases := []struct {
		name   string
		testFn func(testServer *Server)
	}{
		{
			name: "valid token",
			testFn: func(testServer *Server) {

				// Generate and upsert a token.
				token := mock.ACLToken()
				err := testServer.State().UpsertACLTokens(
					structs.MsgTypeTestSetup, 10, []*structs.ACLToken{token})
				require.NoError(t, err)

				// Attempt to look up the token and perform checks.
				tokenResp, err := testServer.ResolveSecretToken(token.SecretID)
				require.NoError(t, err)
				require.NotNil(t, tokenResp)
				require.Equal(t, token, tokenResp)
			},
		},
		{
			name: "anonymous token",
			testFn: func(testServer *Server) {

				// Call the function with an empty input secret ID which is
				// classed as representing anonymous access in clusters with
				// ACLs enabled.
				tokenResp, err := testServer.ResolveSecretToken("")
				require.NoError(t, err)
				require.NotNil(t, tokenResp)
				require.Equal(t, structs.AnonymousACLToken, tokenResp)
			},
		},
		{
			name: "token not found",
			testFn: func(testServer *Server) {

				// Call the function with randomly generated secret ID which
				// does not exist within state.
				tokenResp, err := testServer.ResolveSecretToken(uuid.Generate())
				require.Equal(t, structs.ErrTokenNotFound, err)
				require.Nil(t, tokenResp)
			},
		},
		{
			name: "token expired",
			testFn: func(testServer *Server) {

				// Create a mock token with an expiration time long in the
				// past, and upsert.
				token := mock.ACLToken()
				token.ExpirationTime = pointer.Of(time.Date(
					1970, time.January, 1, 0, 0, 0, 0, time.UTC))

				err := testServer.State().UpsertACLTokens(
					structs.MsgTypeTestSetup, 10, []*structs.ACLToken{token})
				require.NoError(t, err)

				// Perform the function call which should result in finding the
				// token has expired.
				tokenResp, err := testServer.ResolveSecretToken(uuid.Generate())
				require.Equal(t, structs.ErrTokenNotFound, err)
				require.Nil(t, tokenResp)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.testFn(testServer)
		})
	}
}

func TestResolveClaims(t *testing.T) {
	ci.Parallel(t)

	srv, _, cleanup := TestACLServer(t, nil)
	defer cleanup()

	store := srv.fsm.State()
	index := uint64(100)

	alloc := mock.Alloc()

	claims := &structs.IdentityClaims{
		Namespace:    alloc.Namespace,
		JobID:        alloc.Job.ID,
		AllocationID: alloc.ID,
		TaskName:     alloc.Job.TaskGroups[0].Tasks[0].Name,
	}

	// unrelated policy
	policy0 := mock.ACLPolicy()

	// policy for job
	policy1 := mock.ACLPolicy()
	policy1.JobACL = &structs.JobACL{
		Namespace: claims.Namespace,
		JobID:     claims.JobID,
	}

	// policy for job and group
	policy2 := mock.ACLPolicy()
	policy2.JobACL = &structs.JobACL{
		Namespace: claims.Namespace,
		JobID:     claims.JobID,
		Group:     alloc.Job.TaskGroups[0].Name,
	}

	// policy for job and group	and task
	policy3 := mock.ACLPolicy()
	policy3.JobACL = &structs.JobACL{
		Namespace: claims.Namespace,
		JobID:     claims.JobID,
		Group:     alloc.Job.TaskGroups[0].Name,
		Task:      claims.TaskName,
	}

	// policy for job and group	but different task
	policy4 := mock.ACLPolicy()
	policy4.JobACL = &structs.JobACL{
		Namespace: claims.Namespace,
		JobID:     claims.JobID,
		Group:     alloc.Job.TaskGroups[0].Name,
		Task:      "another",
	}

	// policy for job but different group
	policy5 := mock.ACLPolicy()
	policy5.JobACL = &structs.JobACL{
		Namespace: claims.Namespace,
		JobID:     claims.JobID,
		Group:     "another",
	}

	// policy for same namespace but different job
	policy6 := mock.ACLPolicy()
	policy6.JobACL = &structs.JobACL{
		Namespace: claims.Namespace,
		JobID:     "another",
	}

	// policy for same job in different namespace
	policy7 := mock.ACLPolicy()
	policy7.JobACL = &structs.JobACL{
		Namespace: "another",
		JobID:     claims.JobID,
	}

	aclObj, err := srv.ResolveClaims(claims)
	must.Nil(t, aclObj)
	must.EqError(t, err, "allocation does not exist")

	// upsert the allocation
	index++
	err = store.UpsertAllocs(structs.MsgTypeTestSetup, index, []*structs.Allocation{alloc})
	must.NoError(t, err)

	// Resolve claims and check we that the ACL object without policies provides no access
	aclObj, err = srv.ResolveClaims(claims)
	must.NoError(t, err)
	must.NotNil(t, aclObj)
	must.False(t, aclObj.AllowNamespaceOperation("default", acl.NamespaceCapabilityListJobs))

	// Add the policies
	index++
	err = store.UpsertACLPolicies(structs.MsgTypeTestSetup, index, []*structs.ACLPolicy{
		policy0, policy1, policy2, policy3, policy4, policy5, policy6, policy7})
	must.NoError(t, err)

	// Re-resolve and check that the resulting ACL looks reasonable
	aclObj, err = srv.ResolveClaims(claims)
	must.NoError(t, err)
	must.NotNil(t, aclObj)
	must.False(t, aclObj.IsManagement())
	must.True(t, aclObj.AllowNamespaceOperation("default", acl.NamespaceCapabilityListJobs))
	must.False(t, aclObj.AllowNamespaceOperation("other", acl.NamespaceCapabilityListJobs))

	// Resolve the same claim again, should get cache value
	aclObj2, err := srv.ResolveClaims(claims)
	must.NoError(t, err)
	must.NotNil(t, aclObj)
	must.Eq(t, aclObj, aclObj2, must.Sprintf("expected cached value"))

	policies, err := srv.resolvePoliciesForClaims(claims)
	must.NoError(t, err)
	must.Len(t, 3, policies)
	must.SliceContainsAll(t, policies, []*structs.ACLPolicy{policy1, policy2, policy3})
}
