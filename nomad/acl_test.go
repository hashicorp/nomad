package nomad

import (
	"testing"

	lru "github.com/hashicorp/golang-lru"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
)

func TestResolveACLToken(t *testing.T) {
	t.Parallel()

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

	// Check that the ACL object is sane
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
	t.Parallel()
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
	t.Parallel()

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
