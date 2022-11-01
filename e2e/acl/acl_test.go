package acl

import (
	"testing"

	"github.com/hashicorp/go-set"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestACL(t *testing.T) {

	// Wait until we have a usable cluster before running the tests. While the
	// test does not run client workload, some do perform listings of nodes. It
	// is therefore better to wait until we have a node, so these tests can
	// check for a non-empty node list response object.
	nomadClient := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomadClient)
	e2eutil.WaitForNodesReady(t, nomadClient, 1)

	// Run our test cases.
	t.Run("TestACL_Role", testACLRole)
	t.Run("TestACL_TokenExpiration", testACLTokenExpiration)
	t.Run("TestACL_TokenRolePolicyAssignment", testACLTokenRolePolicyAssignment)
}

// testResourceType indicates what the resource is so the cleanup process can
// use the correct API.
type testResourceType int

const (
	namespaceTestResourceType testResourceType = iota
	aclPolicyTestResourceType
	aclRoleTestResourceType
	aclTokenTestResourceType
)

// cleanup stores Nomad resources that have been created by a test which will
// need to be deleted once the test exits. This ensures other tests can run in
// a clean environment and reduces the potential for conflicts.
type cleanup struct {
	namespaces  *set.Set[string]
	aclPolicies *set.Set[string]
	aclRoles    *set.Set[string]
	aclTokens   *set.Set[string]
}

// newCleanup generates an initialized cleanup object for immediate use.
func newCleanup() *cleanup {
	return &cleanup{
		namespaces:  set.New[string](0),
		aclPolicies: set.New[string](0),
		aclRoles:    set.New[string](0),
		aclTokens:   set.New[string](0),
	}
}

// run triggers a cleanup of all the stored resources. This should typically be
// called via defer, so it will always run no matter if the test fails or not.
// Any failure will ultimately fail the test, but will not stop the attempts to
// delete all the resources.
func (c *cleanup) run(t *testing.T, nomadClient *api.Client) {

	for _, namespace := range c.namespaces.List() {
		_, err := nomadClient.Namespaces().Delete(namespace, nil)
		assert.NoError(t, err)
	}

	for _, policy := range c.aclPolicies.List() {
		_, err := nomadClient.ACLPolicies().Delete(policy, nil)
		assert.NoError(t, err)
	}

	for _, role := range c.aclRoles.List() {
		_, err := nomadClient.ACLRoles().Delete(role, nil)
		assert.NoError(t, err)
	}

	for _, token := range c.aclTokens.List() {
		_, err := nomadClient.ACLTokens().Delete(token, nil)
		assert.NoError(t, err)
	}

	require.NoError(t, nomadClient.System().GarbageCollect())
}

// add the resource identifier to the resource tracker. It will be removed by
// the cleanup function once it is triggered.
func (c *cleanup) add(id string, resourceType testResourceType) {
	switch resourceType {
	case namespaceTestResourceType:
		c.namespaces.Insert(id)
	case aclPolicyTestResourceType:
		c.aclPolicies.Insert(id)
	case aclRoleTestResourceType:
		c.aclRoles.Insert(id)
	case aclTokenTestResourceType:
		c.aclTokens.Insert(id)
	}
}

// remove the resource identifier from the resource tracker, indicating it is
// no longer existing on the cluster and does not need to be cleaned.
func (c *cleanup) remove(id string, resourceType testResourceType) {
	switch resourceType {
	case namespaceTestResourceType:
		c.namespaces.Remove(id)
	case aclPolicyTestResourceType:
		c.aclPolicies.Remove(id)
	case aclRoleTestResourceType:
		c.aclRoles.Remove(id)
	case aclTokenTestResourceType:
		c.aclTokens.Remove(id)
	}
}
