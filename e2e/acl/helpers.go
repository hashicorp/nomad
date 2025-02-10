// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"testing"

	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/api"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

// TestResourceType indicates what the resource is so the Cleanup process can
// use the correct API.
type TestResourceType int

const (
	NamespaceTestResourceType TestResourceType = iota
	ACLPolicyTestResourceType
	ACLRoleTestResourceType
	ACLTokenTestResourceType
)

// Cleanup stores Nomad resources that have been created by a test which will
// need to be deleted once the test exits. This ensures other tests can run in
// a clean environment and reduces the potential for conflicts.
type Cleanup struct {
	namespaces  *set.Set[string]
	aclPolicies *set.Set[string]
	aclRoles    *set.Set[string]
	aclTokens   *set.Set[string]
}

// NewCleanup generates an initialized Cleanup object for immediate use.
func NewCleanup() *Cleanup {
	return &Cleanup{
		namespaces:  set.New[string](0),
		aclPolicies: set.New[string](0),
		aclRoles:    set.New[string](0),
		aclTokens:   set.New[string](0),
	}
}

// Run triggers a Cleanup of all the stored resources. This should typically be
// called via defer, so it will always Run no matter if the test fails or not.
// Any failure will ultimately fail the test, but will not stop the attempts to
// delete all the resources.
func (c *Cleanup) Run(t *testing.T, nomadClient *api.Client) {
	for namespace := range c.namespaces.Items() {
		_, err := nomadClient.Namespaces().Delete(namespace, nil)
		test.NoError(t, err)
	}

	for policy := range c.aclPolicies.Items() {
		_, err := nomadClient.ACLPolicies().Delete(policy, nil)
		test.NoError(t, err)
	}

	for role := range c.aclRoles.Items() {
		_, err := nomadClient.ACLRoles().Delete(role, nil)
		test.NoError(t, err)
	}

	for token := range c.aclTokens.Items() {
		_, err := nomadClient.ACLTokens().Delete(token, nil)
		test.NoError(t, err)
	}

	must.NoError(t, nomadClient.System().GarbageCollect())
}

// Add the resource identifier to the resource tracker. It will be removed by
// the Cleanup function once it is triggered.
func (c *Cleanup) Add(id string, resourceType TestResourceType) {
	switch resourceType {
	case NamespaceTestResourceType:
		c.namespaces.Insert(id)
	case ACLPolicyTestResourceType:
		c.aclPolicies.Insert(id)
	case ACLRoleTestResourceType:
		c.aclRoles.Insert(id)
	case ACLTokenTestResourceType:
		c.aclTokens.Insert(id)
	}
}

// Remove the resource identifier from the resource tracker, indicating it is
// no longer existing on the cluster and does not need to be cleaned.
func (c *Cleanup) Remove(id string, resourceType TestResourceType) {
	switch resourceType {
	case NamespaceTestResourceType:
		c.namespaces.Remove(id)
	case ACLPolicyTestResourceType:
		c.aclPolicies.Remove(id)
	case ACLRoleTestResourceType:
		c.aclRoles.Remove(id)
	case ACLTokenTestResourceType:
		c.aclTokens.Remove(id)
	}
}
