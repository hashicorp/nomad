// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/require"
)

// testACLRole tests basic functionality of ACL roles when used for
// authorization. It also performs some basic token and policy tests due to the
// coupling between the ACL objects.
func testACLRole(t *testing.T) {

	nomadClient := e2eutil.NomadClient(t)

	// Create and defer the Cleanup process. This is used to remove all
	// resources created by this test and covers situations where the test
	// fails or during normal running.
	cleanUpProcess := NewCleanup()
	defer cleanUpProcess.Run(t, nomadClient)

	// An ACL role must reference an ACL policy that is stored in state. Ensure
	// this behaviour by attempting to create a role that links to a policy
	// that does not exist.
	invalidRole := api.ACLRole{
		Name:        "e2e-acl-" + uuid.Short(),
		Description: "E2E ACL Role Testing",
		Policies:    []*api.ACLRolePolicyLink{{Name: "404-not-found"}},
	}
	aclRoleCreateResp, _, err := nomadClient.ACLRoles().Create(&invalidRole, nil)
	require.ErrorContains(t, err, "cannot find policy 404-not-found")
	require.Nil(t, aclRoleCreateResp)

	// Create a custom namespace to test along with the default.
	ns := api.Namespace{
		Name:        "e2e-acl-" + uuid.Short(),
		Description: "E2E ACL Role Testing",
	}
	_, err = nomadClient.Namespaces().Register(&ns, nil)
	require.NoError(t, err)

	cleanUpProcess.Add(ns.Name, NamespaceTestResourceType)

	// Create an ACL policy which will be used to link from the role. This
	// policy grants read access to our custom namespace.
	customNamespacePolicy := api.ACLPolicy{
		Name:        "e2e-acl-" + uuid.Short(),
		Description: "E2E ACL Role Testing",
		Rules:       fmt.Sprintf(`namespace %q {policy = "read"}`, ns.Name),
	}
	_, err = nomadClient.ACLPolicies().Upsert(&customNamespacePolicy, nil)
	require.NoError(t, err)

	cleanUpProcess.Add(customNamespacePolicy.Name, ACLPolicyTestResourceType)

	// Create a valid role with a link to the previously created policy.
	validRole := api.ACLRole{
		Name:        "e2e-acl-" + uuid.Short(),
		Description: "E2E ACL Role Testing",
		Policies:    []*api.ACLRolePolicyLink{{Name: customNamespacePolicy.Name}},
	}
	aclRoleCreateResp, _, err = nomadClient.ACLRoles().Create(&validRole, nil)
	require.NoError(t, err)
	require.NotNil(t, aclRoleCreateResp)
	require.NotEmpty(t, aclRoleCreateResp.ID)
	require.Equal(t, validRole.Name, aclRoleCreateResp.Name)

	cleanUpProcess.Add(aclRoleCreateResp.ID, ACLRoleTestResourceType)

	// Perform a role listing and check we have the expected entries.
	aclRoleListResp, _, err := nomadClient.ACLRoles().List(nil)
	require.NoError(t, err)
	require.Len(t, aclRoleListResp, 1)
	require.Equal(t, aclRoleCreateResp.ID, aclRoleListResp[0].ID)

	// Create our ACL token which is linked to the created ACL role.
	token := api.ACLToken{
		Name:  "e2e-acl-" + uuid.Short(),
		Type:  "client",
		Roles: []*api.ACLTokenRoleLink{{ID: aclRoleCreateResp.ID}},
	}
	aclTokenCreateResp, _, err := nomadClient.ACLTokens().Create(&token, nil)
	require.NoError(t, err)
	require.NotNil(t, aclTokenCreateResp)

	cleanUpProcess.Add(aclTokenCreateResp.AccessorID, ACLTokenTestResourceType)

	// Attempt two job listings against the two available namespaces. The token
	// only has access to the custom namespace, so the default should return an
	// error.
	customNSQueryMeta := api.QueryOptions{Namespace: ns.Name, AuthToken: aclTokenCreateResp.SecretID}
	defaultNSQueryMeta := api.QueryOptions{Namespace: "default", AuthToken: aclTokenCreateResp.SecretID}

	jobListResp, _, err := nomadClient.Jobs().List(&customNSQueryMeta)
	require.NoError(t, err)
	require.Empty(t, jobListResp)

	jobListResp, _, err = nomadClient.Jobs().List(&defaultNSQueryMeta)
	require.ErrorContains(t, err, "Permission denied")

	// Create an ACL policy which grants read access to the default namespace.
	defaultNamespacePolicy := api.ACLPolicy{
		Name:        "e2e-acl-" + uuid.Short(),
		Description: "E2E ACL Role Testing",
		Rules:       `namespace "default" {policy = "read"}`,
	}
	_, err = nomadClient.ACLPolicies().Upsert(&defaultNamespacePolicy, nil)
	require.NoError(t, err)

	cleanUpProcess.Add(defaultNamespacePolicy.Name, ACLPolicyTestResourceType)

	// Update the ACL role to include the new ACL policy that allows read
	// access to the default namespace.
	aclRoleCreateResp.Policies = append(aclRoleCreateResp.Policies, &api.ACLRolePolicyLink{
		Name: defaultNamespacePolicy.Name,
	})
	aclRoleUpdateResp, _, err := nomadClient.ACLRoles().Update(aclRoleCreateResp, nil)
	require.NoError(t, err)
	require.Equal(t, aclRoleCreateResp.ID, aclRoleUpdateResp.ID)
	require.Len(t, aclRoleUpdateResp.Policies, 2)

	// Try listing the jobs in the default namespace again to ensure we now
	// have permission due to the updated role.
	jobListResp, _, err = nomadClient.Jobs().List(&defaultNSQueryMeta)
	require.NoError(t, err)
	require.Empty(t, jobListResp)

	// Delete a policy from under the role.
	_, err = nomadClient.ACLPolicies().Delete(defaultNamespacePolicy.Name, nil)
	require.NoError(t, err)

	cleanUpProcess.Remove(defaultNamespacePolicy.Name, ACLPolicyTestResourceType)

	// The permission to list the job in the default namespace should now be
	// revoked.
	jobListResp, _, err = nomadClient.Jobs().List(&defaultNSQueryMeta)
	require.ErrorContains(t, err, "Permission denied")

	// Delete the ACL role.
	_, err = nomadClient.ACLRoles().Delete(aclRoleUpdateResp.ID, nil)
	require.NoError(t, err)

	cleanUpProcess.Remove(aclRoleUpdateResp.ID, ACLRoleTestResourceType)

	// We should now not be able to list jobs in the custom namespace either as
	// the token does not have any permissions.
	jobListResp, _, err = nomadClient.Jobs().List(&customNSQueryMeta)
	require.ErrorContains(t, err, "Permission denied")
}
