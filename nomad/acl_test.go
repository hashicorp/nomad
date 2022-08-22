package nomad

import (
	"testing"

	lru "github.com/hashicorp/golang-lru"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func TestResolveACLToken(t *testing.T) {
	ci.Parallel(t)

	// Create mock state store and cache
	state := state.TestStateStore(t)
	cache, err := lru.New2Q(16)
	assert.Nil(t, err)

	// Create a policy / token
	policy := mock.ACLPolicy()
	policy2 := mock.ACLPolicy()
	token := mock.ACLToken()
	token.Policies = []string{policy.Name, policy2.Name}
	token2 := mock.ACLToken()
	token2.Type = structs.ACLManagementToken
	token2.Policies = nil
	err = state.UpsertACLPolicies(structs.MsgTypeTestSetup, 100, []*structs.ACLPolicy{policy, policy2})
	assert.Nil(t, err)
	err = state.UpsertACLTokens(structs.MsgTypeTestSetup, 110, []*structs.ACLToken{token, token2})
	assert.Nil(t, err)

	snap, err := state.Snapshot()
	assert.Nil(t, err)

	// Attempt resolution of blank token. Should return anonymous policy
	aclObj, err := resolveTokenFromSnapshotCache(snap, cache, "")
	assert.Nil(t, err)
	assert.NotNil(t, aclObj)

	// Attempt resolution of unknown token. Should fail.
	randID := uuid.Generate()
	aclObj, err = resolveTokenFromSnapshotCache(snap, cache, randID)
	assert.Equal(t, structs.ErrTokenNotFound, err)
	assert.Nil(t, aclObj)

	// Attempt resolution of management token. Should get singleton.
	aclObj, err = resolveTokenFromSnapshotCache(snap, cache, token2.SecretID)
	assert.Nil(t, err)
	assert.NotNil(t, aclObj)
	assert.Equal(t, true, aclObj.IsManagement())
	if aclObj != acl.ManagementACL {
		t.Fatalf("expected singleton")
	}

	// Attempt resolution of client token
	aclObj, err = resolveTokenFromSnapshotCache(snap, cache, token.SecretID)
	assert.Nil(t, err)
	assert.NotNil(t, aclObj)

	// Check that the ACL object looks reasonable
	assert.Equal(t, false, aclObj.IsManagement())
	allowed := aclObj.AllowNamespaceOperation("default", acl.NamespaceCapabilityListJobs)
	assert.Equal(t, true, allowed)
	allowed = aclObj.AllowNamespaceOperation("other", acl.NamespaceCapabilityListJobs)
	assert.Equal(t, false, allowed)

	// Resolve the same token again, should get cache value
	aclObj2, err := resolveTokenFromSnapshotCache(snap, cache, token.SecretID)
	assert.Nil(t, err)
	assert.NotNil(t, aclObj2)
	if aclObj != aclObj2 {
		t.Fatalf("expected cached value")
	}

	// Bust the cache by upserting the policy
	err = state.UpsertACLPolicies(structs.MsgTypeTestSetup, 120, []*structs.ACLPolicy{policy})
	assert.Nil(t, err)
	snap, err = state.Snapshot()
	assert.Nil(t, err)

	// Resolve the same token again, should get different value
	aclObj3, err := resolveTokenFromSnapshotCache(snap, cache, token.SecretID)
	assert.Nil(t, err)
	assert.NotNil(t, aclObj3)
	if aclObj == aclObj3 {
		t.Fatalf("unexpected cached value")
	}
}

func TestResolveACLToken_LeaderToken(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	s1, _, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	leaderAcl := s1.getLeaderAcl()
	assert.NotEmpty(leaderAcl)
	token, err := s1.ResolveToken(leaderAcl)
	assert.Nil(err)
	if assert.NotNil(token) {
		assert.True(token.IsManagement())
	}
}

func TestResolveSecretToken(t *testing.T) {
	ci.Parallel(t)

	s1, _, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	state := s1.State()
	leaderToken := s1.getLeaderAcl()
	assert.NotEmpty(t, leaderToken)

	token := mock.ACLToken()

	err := state.UpsertACLTokens(structs.MsgTypeTestSetup, 110, []*structs.ACLToken{token})
	assert.Nil(t, err)

	respToken, err := s1.ResolveSecretToken(token.SecretID)
	assert.Nil(t, err)
	if assert.NotNil(t, respToken) {
		assert.NotEmpty(t, respToken.AccessorID)
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

	index++
	err := store.UpsertACLPolicies(structs.MsgTypeTestSetup, index, []*structs.ACLPolicy{
		policy0, policy1, policy2, policy3, policy4, policy5, policy6, policy7})
	must.NoError(t, err)

	aclObj, err := srv.ResolveClaims(claims)
	must.Nil(t, aclObj)
	must.EqError(t, err, "allocation does not exist")

	// upsert the allocation
	index++
	err = store.UpsertAllocs(structs.MsgTypeTestSetup, index, []*structs.Allocation{alloc})
	must.NoError(t, err)

	aclObj, err = srv.ResolveClaims(claims)
	must.NoError(t, err)
	must.NotNil(t, aclObj)

	// Check that the ACL object looks reasonable
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
	must.Contains(t, policies, policy1)
	must.Contains(t, policies, policy2)
	must.Contains(t, policies, policy3)
}
