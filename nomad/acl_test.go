package nomad

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

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
