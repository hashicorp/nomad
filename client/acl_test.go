// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

func Test_clientACLResolver_init(t *testing.T) {
	resolver := new(clientACLResolver)
	resolver.init()
	must.NotNil(t, resolver.aclCache)
	must.NotNil(t, resolver.policyCache)
	must.NotNil(t, resolver.tokenCache)
	must.NotNil(t, resolver.roleCache)
}

func TestClient_ACL_resolveTokenValue(t *testing.T) {
	ci.Parallel(t)

	s1, _, _, cleanupS1 := testACLServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.ACLEnabled = true
	})
	defer cleanup()

	// Create a policy / token
	policy := mock.ACLPolicy()
	policy2 := mock.ACLPolicy()
	token := mock.ACLToken()
	token.Policies = []string{policy.Name, policy2.Name}
	token2 := mock.ACLToken()
	token2.Type = structs.ACLManagementToken
	token2.Policies = nil
	err := s1.State().UpsertACLPolicies(structs.MsgTypeTestSetup, 100, []*structs.ACLPolicy{policy, policy2})
	must.NoError(t, err)
	err = s1.State().UpsertACLTokens(structs.MsgTypeTestSetup, 110, []*structs.ACLToken{token, token2})
	must.NoError(t, err)

	// Test the client resolution
	out0, err := c1.resolveTokenValue("")
	test.Nil(t, err)
	must.NotNil(t, out0)
	test.Eq(t, structs.AnonymousACLToken, out0.ACLToken)

	out1, err := c1.resolveTokenValue(token.SecretID)
	test.Nil(t, err)
	must.NotNil(t, out1)
	test.Eq(t, token, out1.ACLToken)

	out2, err := c1.resolveTokenValue(token2.SecretID)
	test.Nil(t, err)
	must.NotNil(t, out2)
	test.Eq(t, token2, out2.ACLToken)

	out3, err := c1.resolveTokenValue(token.SecretID)
	test.Nil(t, err)
	must.Eq(t, out1, out3, must.Sprintf("bad caching"))
}

func TestClient_ACL_resolvePolicies(t *testing.T) {
	ci.Parallel(t)

	s1, _, root, cleanupS1 := testACLServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.ACLEnabled = true
	})
	defer cleanup()

	// Create a policy / token
	policy := mock.ACLPolicy()
	policy2 := mock.ACLPolicy()
	token := mock.ACLToken()
	token.Policies = []string{policy.Name, policy2.Name}
	token2 := mock.ACLToken()
	token2.Type = structs.ACLManagementToken
	token2.Policies = nil
	err := s1.State().UpsertACLPolicies(structs.MsgTypeTestSetup, 100, []*structs.ACLPolicy{policy, policy2})
	must.NoError(t, err)
	err = s1.State().UpsertACLTokens(structs.MsgTypeTestSetup, 110, []*structs.ACLToken{token, token2})
	must.NoError(t, err)

	// Test the client resolution
	out, err := c1.resolvePolicies(root.SecretID, []string{policy.Name, policy2.Name})
	must.NoError(t, err)
	test.Len(t, 2, out)

	// Test caching
	out2, err := c1.resolvePolicies(root.SecretID, []string{policy.Name, policy2.Name})
	must.NoError(t, err)
	test.Len(t, 2, out2)

	// Check we get the same objects back (ignore ordering)
	if out[0] != out2[0] && out[0] != out2[1] {
		t.Fatalf("bad caching")
	}
}

func TestClient_resolveTokenACLRoles(t *testing.T) {
	ci.Parallel(t)

	testServer, _, rootACLToken, testServerCleanupS1 := testACLServer(t, nil)
	defer testServerCleanupS1()
	testutil.WaitForLeader(t, testServer.RPC)

	testClient, cleanup := TestClient(t, func(c *config.Config) {
		c.RPCHandler = testServer
		c.ACLEnabled = true
	})
	defer cleanup()

	// Create an ACL Role and a client token which is linked to this.
	mockACLRole := mock.ACLRole()

	mockACLToken := mock.ACLToken()
	mockACLToken.Policies = []string{}
	mockACLToken.Roles = []*structs.ACLTokenRoleLink{{ID: mockACLRole.ID}}

	err := testServer.State().UpsertACLRoles(structs.MsgTypeTestSetup, 10, []*structs.ACLRole{mockACLRole}, true)
	must.NoError(t, err)
	err = testServer.State().UpsertACLTokens(structs.MsgTypeTestSetup, 20, []*structs.ACLToken{mockACLToken})
	must.NoError(t, err)

	// Resolve the ACL policies linked via the role.
	resolvedRoles1, err := testClient.resolveTokenACLRoles(rootACLToken.SecretID, mockACLToken.Roles)
	must.NoError(t, err)
	must.Len(t, 2, resolvedRoles1)

	// Test the cache directly and check that the ACL role previously queried
	// is now cached.
	must.Eq(t, 1, testClient.roleCache.Len())
	must.True(t, testClient.roleCache.Contains(mockACLRole.ID))

	// Resolve the roles again to check we get the same results.
	resolvedRoles2, err := testClient.resolveTokenACLRoles(rootACLToken.SecretID, mockACLToken.Roles)
	must.NoError(t, err)
	must.SliceContainsAll(t, resolvedRoles1, resolvedRoles2)
}

func TestClient_ACL_ResolveToken_Disabled(t *testing.T) {
	ci.Parallel(t)

	s1, _, cleanupS1 := testServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
	})
	defer cleanup()

	// Should always get nil when disabled
	aclObj, err := c1.ResolveToken("blah")
	must.NoError(t, err)
	must.Nil(t, aclObj)
}

func TestClient_ACL_ResolveToken(t *testing.T) {
	ci.Parallel(t)

	s1, _, _, cleanupS1 := testACLServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.ACLEnabled = true
	})
	defer cleanup()

	// Create a policy / token
	policy := mock.ACLPolicy()
	policy2 := mock.ACLPolicy()
	token := mock.ACLToken()
	token.Policies = []string{policy.Name, policy2.Name}
	token2 := mock.ACLToken()
	token2.Type = structs.ACLManagementToken
	token2.Policies = nil
	err := s1.State().UpsertACLPolicies(structs.MsgTypeTestSetup, 100, []*structs.ACLPolicy{policy, policy2})
	must.NoError(t, err)
	err = s1.State().UpsertACLTokens(structs.MsgTypeTestSetup, 110, []*structs.ACLToken{token, token2})
	must.NoError(t, err)

	// Test the client resolution
	out, err := c1.ResolveToken(token.SecretID)
	must.NoError(t, err)
	test.NotNil(t, out)

	// Test caching
	out2, err := c1.ResolveToken(token.SecretID)
	must.NoError(t, err)
	must.Eq(t, out, out2, must.Sprintf("should be cached"))

	// Test management token
	out3, err := c1.ResolveToken(token2.SecretID)
	must.NoError(t, err)
	must.Eq(t, acl.ManagementACL, out3)

	// Test bad token
	out4, err := c1.ResolveToken(uuid.Generate())
	test.EqError(t, err, structs.ErrPermissionDenied.Error())
	test.Nil(t, out4)
}

func TestClient_ACL_ResolveToken_Expired(t *testing.T) {
	ci.Parallel(t)

	s1, _, _, cleanupS1 := testACLServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.ACLEnabled = true
	})
	defer cleanup()

	// Create and upsert a token which has just expired.
	mockExpiredToken := mock.ACLToken()
	mockExpiredToken.ExpirationTime = pointer.Of(time.Now().Add(-5 * time.Minute))

	err := s1.State().UpsertACLTokens(structs.MsgTypeTestSetup, 120, []*structs.ACLToken{mockExpiredToken})
	must.NoError(t, err)

	expiredTokenResp, err := c1.ResolveToken(mockExpiredToken.SecretID)
	must.Nil(t, expiredTokenResp)
	must.ErrorContains(t, err, "ACL token expired")
}

// TestClient_ACL_ResolveToken_Claims asserts that ResolveToken
// properly resolves valid workload identity claims.
func TestClient_ACL_ResolveToken_Claims(t *testing.T) {
	ci.Parallel(t)

	s1, _, rootToken, cleanupS1 := testACLServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.ACLEnabled = true
	})
	defer cleanup()

	// Create a minimal job
	job := mock.MinJob()

	// Add a job policy
	polArgs := structs.ACLPolicyUpsertRequest{
		Policies: []*structs.ACLPolicy{
			{
				Name:        "nw",
				Description: "test job can write to nodes",
				Rules:       `node { policy = "write" }`,
				JobACL: &structs.JobACL{
					Namespace: job.Namespace,
					JobID:     job.ID,
				},
			},
		},
		WriteRequest: structs.WriteRequest{
			Region:    job.Region,
			AuthToken: rootToken.SecretID,
			Namespace: job.Namespace,
		},
	}
	polReply := structs.GenericResponse{}
	must.NoError(t, s1.RPC("ACL.UpsertPolicies", &polArgs, &polReply))
	must.NonZero(t, polReply.WriteMeta.Index)

	allocs := testutil.WaitForRunningWithToken(t, s1.RPC, job, rootToken.SecretID)
	must.Len(t, 1, allocs)

	alloc, err := s1.State().AllocByID(nil, allocs[0].ID)
	must.NoError(t, err)
	must.MapContainsKey(t, alloc.SignedIdentities, "t")
	wid := alloc.SignedIdentities["t"]

	aclObj, err := c1.ResolveToken(wid)
	must.NoError(t, err)
	must.True(t, aclObj.AllowNodeWrite(), must.Sprintf("expected workload id to allow node write"))
}

// TestClient_ACL_ResolveToken_InvalidClaims asserts that ResolveToken properly
// rejects invalid workload identity claims.
func TestClient_ACL_ResolveToken_InvalidClaims(t *testing.T) {
	ci.Parallel(t)

	s1, _, rootToken, cleanupS1 := testACLServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.ACLEnabled = true
	})
	defer cleanup()

	// Create a minimal job
	job := mock.MinJob()
	allocs := testutil.WaitForRunningWithToken(t, s1.RPC, job, rootToken.SecretID)
	must.Len(t, 1, allocs)

	// Get wid while it's still running
	alloc, err := s1.State().AllocByID(nil, allocs[0].ID)
	must.NoError(t, err)
	must.MapContainsKey(t, alloc.SignedIdentities, "t")
	wid := alloc.SignedIdentities["t"]

	// Stop job
	deregArgs := structs.JobDeregisterRequest{
		JobID: job.ID,
		WriteRequest: structs.WriteRequest{
			Region:    job.Region,
			Namespace: job.Namespace,
			AuthToken: rootToken.SecretID,
		},
	}
	deregReply := structs.JobDeregisterResponse{}
	must.NoError(t, s1.RPC("Job.Deregister", &deregArgs, &deregReply))

	cond := map[string]int{
		structs.AllocClientStatusComplete: 1,
	}
	allocs = testutil.WaitForJobAllocStatusWithToken(t, s1.RPC, job, cond, rootToken.SecretID)
	must.Len(t, 1, allocs)

	// ResolveToken should error now that alloc is dead
	aclObj, err := c1.ResolveToken(wid)
	must.ErrorContains(t, err, "allocation is terminal")
	must.Nil(t, aclObj)
}
