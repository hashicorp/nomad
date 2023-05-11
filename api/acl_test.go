// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestACLPolicies_ListUpsert(t *testing.T) {
	testutil.Parallel(t)

	c, s, _ := makeACLClient(t, nil, nil)
	defer s.Stop()
	ap := c.ACLPolicies()

	// Listing when nothing exists returns empty
	result, qm, err := ap.List(nil)
	must.NoError(t, err)
	must.One(t, qm.LastIndex)
	must.Len(t, 0, result)

	// Register a policy
	policy := &ACLPolicy{
		Name:        "test",
		Description: "test",
		Rules: `namespace "default" {
			policy = "read"
		}
		`,
	}
	wm, err := ap.Upsert(policy, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Check the list again
	result, qm, err = ap.List(nil)
	must.NoError(t, err)

	assertQueryMeta(t, qm)
	must.Len(t, 1, result)
}

func TestACLPolicies_Delete(t *testing.T) {
	testutil.Parallel(t)

	c, s, _ := makeACLClient(t, nil, nil)
	defer s.Stop()
	ap := c.ACLPolicies()

	// Register a policy
	policy := &ACLPolicy{
		Name:        "test",
		Description: "test",
		Rules: `namespace "default" {
			policy = "read"
		}
		`,
	}
	wm, err := ap.Upsert(policy, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Delete the policy
	wm, err = ap.Delete(policy.Name, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Check the list again
	result, qm, err := ap.List(nil)
	must.NoError(t, err)

	assertQueryMeta(t, qm)
	must.Len(t, 0, result)
}

func TestACLPolicies_Info(t *testing.T) {
	testutil.Parallel(t)

	c, s, _ := makeACLClient(t, nil, nil)
	defer s.Stop()
	ap := c.ACLPolicies()

	// Register a policy
	policy := &ACLPolicy{
		Name:        "test",
		Description: "test",
		Rules: `namespace "default" {
			policy = "read"
		}
		`,
	}
	wm, err := ap.Upsert(policy, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Query the policy
	out, qm, err := ap.Info(policy.Name, nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.Eq(t, policy.Name, out.Name)
}

func TestACLTokens_List(t *testing.T) {
	testutil.Parallel(t)

	c, s, _ := makeACLClient(t, nil, nil)
	defer s.Stop()
	at := c.ACLTokens()

	// Expect the bootstrap token.
	result, qm, err := at.List(nil)
	must.NoError(t, err)
	must.NonZero(t, qm.LastIndex)
	must.Len(t, 1, result)
	must.Nil(t, result[0].ExpirationTime)

	// Create a token with an expiry.
	token := &ACLToken{
		Name:          "token-with-expiry",
		Type:          "client",
		Policies:      []string{"foo1"},
		ExpirationTTL: 1 * time.Hour,
	}
	createExpirationResp, _, err := at.Create(token, nil)
	must.NoError(t, err)

	// Perform the listing again and ensure we have two entries along with the
	// expiration correctly set and available.
	listResp, qm, err := at.List(nil)
	must.Nil(t, err)
	assertQueryMeta(t, qm)
	must.Len(t, 2, listResp)

	for _, tokenStub := range listResp {
		if tokenStub.AccessorID == createExpirationResp.AccessorID {
			must.NotNil(t, tokenStub.ExpirationTime)
		}
	}
}

func TestACLTokens_CreateUpdate(t *testing.T) {
	testutil.Parallel(t)

	c, s, _ := makeACLClient(t, nil, nil)
	defer s.Stop()
	at := c.ACLTokens()

	token := &ACLToken{
		Name:     "foo",
		Type:     "client",
		Policies: []string{"foo1"},
	}

	// Create the token
	out, wm, err := at.Create(token, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)
	must.NotNil(t, out)

	// Update the token
	out.Name = "other"
	out2, wm, err := at.Update(out, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)
	must.NotNil(t, out2)

	// Verify the change took hold
	must.Eq(t, out.Name, out2.Name)

	// Try updating the token to include a TTL which is not allowed.
	out2.ExpirationTTL = 10 * time.Minute
	out3, _, err := at.Update(out2, nil)
	must.Error(t, err)
	must.Nil(t, out3)

	// Try adding a role link to our token, which should be possible. For this
	// we need to create a policy and link to this from a role.
	aclPolicy := ACLPolicy{
		Name:  "acl-role-api-test",
		Rules: `namespace "default" { policy = "read" }`,
	}
	writeMeta, err := c.ACLPolicies().Upsert(&aclPolicy, nil)
	must.NoError(t, err)
	assertWriteMeta(t, writeMeta)

	// Create an ACL role referencing the previously created
	// policy.
	role := ACLRole{
		Name:     "acl-role-api-test",
		Policies: []*ACLRolePolicyLink{{Name: aclPolicy.Name}},
	}
	aclRoleCreateResp, writeMeta, err := c.ACLRoles().Create(&role, nil)
	must.NoError(t, err)
	assertWriteMeta(t, writeMeta)
	must.UUIDv4(t, aclRoleCreateResp.ID)
	must.Eq(t, role.Name, aclRoleCreateResp.Name)

	out2.Roles = []*ACLTokenRoleLink{{Name: aclRoleCreateResp.Name}}
	out2.ExpirationTTL = 0

	out3, _, err = at.Update(out2, nil)
	must.NoError(t, err)
	must.NotNil(t, out3)
	must.Len(t, 1, out3.Policies)
	must.Eq(t, "foo1", out3.Policies[0])
	must.Len(t, 1, out3.Roles)
	must.Eq(t, role.Name, out3.Roles[0].Name)
}

func TestACLTokens_Info(t *testing.T) {
	testutil.Parallel(t)

	testClient, testServer, _ := makeACLClient(t, nil, nil)
	defer testServer.Stop()

	testCases := []struct {
		name   string
		testFn func(client *Client)
	}{
		{
			name: "token without expiry",
			testFn: func(client *Client) {

				token := &ACLToken{
					Name:     "foo",
					Type:     "client",
					Policies: []string{"foo1"},
				}

				// Create the token
				out, wm, err := client.ACLTokens().Create(token, nil)
				must.NoError(t, err)
				assertWriteMeta(t, wm)
				must.NotNil(t, out)

				// Query the token
				out2, qm, err := client.ACLTokens().Info(out.AccessorID, nil)
				must.NoError(t, err)
				assertQueryMeta(t, qm)
				must.Eq(t, out, out2)
			},
		},
		{
			name: "token with expiry",
			testFn: func(client *Client) {

				token := &ACLToken{
					Name:          "token-with-expiry",
					Type:          "client",
					Policies:      []string{"foo1"},
					ExpirationTTL: 10 * time.Minute,
				}

				// Create the token
				out, wm, err := client.ACLTokens().Create(token, nil)
				must.NoError(t, err)
				assertWriteMeta(t, wm)
				must.NotNil(t, out)

				// Query the token and ensure it matches what was returned
				// during the creation as well as ensuring the expiration time
				// is set.
				out2, qm, err := client.ACLTokens().Info(out.AccessorID, nil)
				must.NoError(t, err)
				assertQueryMeta(t, qm)
				must.Eq(t, out, out2)
				must.NotNil(t, out2.ExpirationTime)
			},
		},
		{
			name: "token with role link",
			testFn: func(client *Client) {

				// Create an ACL policy that can be referenced within the ACL
				// role.
				aclPolicy := ACLPolicy{
					Name:  "acl-role-api-test",
					Rules: `namespace "default" { policy = "read" }`,
				}
				writeMeta, err := testClient.ACLPolicies().Upsert(&aclPolicy, nil)
				must.NoError(t, err)
				assertWriteMeta(t, writeMeta)

				// Create an ACL role referencing the previously created
				// policy.
				role := ACLRole{
					Name:     "acl-role-api-test",
					Policies: []*ACLRolePolicyLink{{Name: aclPolicy.Name}},
				}
				aclRoleCreateResp, writeMeta, err := testClient.ACLRoles().Create(&role, nil)
				must.NoError(t, err)
				assertWriteMeta(t, writeMeta)
				must.UUIDv4(t, aclRoleCreateResp.ID)
				must.Eq(t, role.Name, aclRoleCreateResp.Name)

				// Create a token with a role linking.
				token := &ACLToken{
					Name:  "token-with-role-link",
					Type:  "client",
					Roles: []*ACLTokenRoleLink{{Name: role.Name}},
				}

				out, wm, err := client.ACLTokens().Create(token, nil)
				must.NoError(t, err)
				assertWriteMeta(t, wm)
				must.NotNil(t, out)

				// Query the token and ensure it matches what was returned
				// during the creation.
				out2, qm, err := client.ACLTokens().Info(out.AccessorID, nil)
				must.NoError(t, err)
				assertQueryMeta(t, qm)
				must.Eq(t, out, out2)
				must.Len(t, 1, out.Roles)
				must.Eq(t, out.Roles[0].Name, aclPolicy.Name)
			},
		},

		{
			name: "token with role and policy link",
			testFn: func(client *Client) {

				// Create an ACL policy that can be referenced within the ACL
				// role.
				aclPolicy1 := ACLPolicy{
					Name:  "acl-role-api-test-1",
					Rules: `namespace "default" { policy = "read" }`,
				}
				writeMeta, err := testClient.ACLPolicies().Upsert(&aclPolicy1, nil)
				must.NoError(t, err)
				assertWriteMeta(t, writeMeta)

				// Create another that can be referenced within the ACL token
				// directly.
				aclPolicy2 := ACLPolicy{
					Name:  "acl-role-api-test-2",
					Rules: `namespace "fawlty" { policy = "read" }`,
				}
				writeMeta, err = testClient.ACLPolicies().Upsert(&aclPolicy2, nil)
				must.NoError(t, err)
				assertWriteMeta(t, writeMeta)

				// Create an ACL role referencing the previously created
				// policy.
				role := ACLRole{
					Name:     "acl-role-api-test-role-and-policy",
					Policies: []*ACLRolePolicyLink{{Name: aclPolicy1.Name}},
				}
				aclRoleCreateResp, writeMeta, err := testClient.ACLRoles().Create(&role, nil)
				must.NoError(t, err)
				assertWriteMeta(t, writeMeta)
				must.NotEq(t, "", aclRoleCreateResp.ID)
				must.Eq(t, role.Name, aclRoleCreateResp.Name)

				// Create a token with a role linking.
				token := &ACLToken{
					Name:     "token-with-role-and-policy-link",
					Type:     "client",
					Policies: []string{aclPolicy2.Name},
					Roles:    []*ACLTokenRoleLink{{Name: role.Name}},
				}

				out, wm, err := client.ACLTokens().Create(token, nil)
				must.NoError(t, err)
				assertWriteMeta(t, wm)
				must.NotNil(t, out)
				must.Len(t, 1, out.Policies)
				must.Eq(t, out.Policies[0], aclPolicy2.Name)
				must.Len(t, 1, out.Roles)
				must.Eq(t, out.Roles[0].Name, role.Name)

				// Query the token and ensure it matches what was returned
				// during the creation.
				out2, qm, err := client.ACLTokens().Info(out.AccessorID, nil)
				must.NoError(t, err)
				assertQueryMeta(t, qm)
				must.Eq(t, out, out2)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.testFn(testClient)
		})
	}
}

func TestACLTokens_Self(t *testing.T) {
	testutil.Parallel(t)

	c, s, _ := makeACLClient(t, nil, nil)
	defer s.Stop()
	at := c.ACLTokens()

	token := &ACLToken{
		Name:     "foo",
		Type:     "client",
		Policies: []string{"foo1"},
	}

	// Create the token
	out, wm, err := at.Create(token, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)
	must.NotNil(t, out)

	// Set the clients token to the new token
	c.SetSecretID(out.SecretID)
	at = c.ACLTokens()

	// Query the token
	out2, qm, err := at.Self(nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.Eq(t, out, out2)
}

func TestACLTokens_Delete(t *testing.T) {
	testutil.Parallel(t)

	c, s, _ := makeACLClient(t, nil, nil)
	defer s.Stop()
	at := c.ACLTokens()

	token := &ACLToken{
		Name:     "foo",
		Type:     "client",
		Policies: []string{"foo1"},
	}

	// Create the token
	out, wm, err := at.Create(token, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)
	must.NotNil(t, out)

	// Delete the token
	wm, err = at.Delete(out.AccessorID, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)
}

func TestACL_OneTimeToken(t *testing.T) {
	testutil.Parallel(t)

	c, s, _ := makeACLClient(t, nil, nil)
	defer s.Stop()
	at := c.ACLTokens()

	token := &ACLToken{
		Name:     "foo",
		Type:     "client",
		Policies: []string{"foo1"},
	}

	// Create the ACL token
	out, wm, err := at.Create(token, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)
	must.NotNil(t, out)

	// Get a one-time token
	c.SetSecretID(out.SecretID)
	out2, wm, err := at.UpsertOneTimeToken(nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)
	must.NotNil(t, out2)

	// Exchange the one-time token
	out3, wm, err := at.ExchangeOneTimeToken(out2.OneTimeSecretID, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)
	must.NotNil(t, out3)
	must.Eq(t, out.AccessorID, out3.AccessorID)
}

func TestACLTokens_BootstrapInvalidToken(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.ACL.Enabled = true
	})
	defer s.Stop()
	at := c.ACLTokens()

	bootkn := "badtoken"
	// Bootstrap with invalid token
	_, _, err := at.BootstrapOpts(bootkn, nil)
	must.EqError(t, err, "Unexpected response code: 400 (invalid acl token)")
}

func TestACLTokens_BootstrapValidToken(t *testing.T) {
	testutil.Parallel(t)
	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.ACL.Enabled = true
	})
	defer s.Stop()
	at := c.ACLTokens()

	bootkn := "2b778dd9-f5f1-6f29-b4b4-9a5fa948757a"
	// Bootstrap with Valid token
	out, wm, err := at.BootstrapOpts(bootkn, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)
	must.Eq(t, bootkn, out.SecretID)
}

func TestACLRoles(t *testing.T) {
	testutil.Parallel(t)

	testClient, testServer, _ := makeACLClient(t, nil, nil)
	defer testServer.Stop()

	// An initial listing shouldn't return any results.
	aclRoleListResp, queryMeta, err := testClient.ACLRoles().List(nil)
	must.NoError(t, err)
	must.SliceEmpty(t, aclRoleListResp)
	assertQueryMeta(t, queryMeta)

	// Create an ACL policy that can be referenced within the ACL role.
	aclPolicy := ACLPolicy{
		Name: "acl-role-api-test",
		Rules: `namespace "default" {
			policy = "read"
		}
		`,
	}
	writeMeta, err := testClient.ACLPolicies().Upsert(&aclPolicy, nil)
	must.NoError(t, err)
	assertWriteMeta(t, writeMeta)

	// Create an ACL role referencing the previously created policy.
	role := ACLRole{
		Name:     "acl-role-api-test",
		Policies: []*ACLRolePolicyLink{{Name: aclPolicy.Name}},
	}
	aclRoleCreateResp, writeMeta, err := testClient.ACLRoles().Create(&role, nil)
	must.NoError(t, err)
	assertWriteMeta(t, writeMeta)
	must.UUIDv4(t, aclRoleCreateResp.ID)
	must.Eq(t, role.Name, aclRoleCreateResp.Name)

	// Another listing should return one result.
	aclRoleListResp, queryMeta, err = testClient.ACLRoles().List(nil)
	must.NoError(t, err)
	must.Len(t, 1, aclRoleListResp)
	assertQueryMeta(t, queryMeta)

	// Read the role using its ID.
	aclRoleReadResp, queryMeta, err := testClient.ACLRoles().Get(aclRoleCreateResp.ID, nil)
	must.NoError(t, err)
	assertQueryMeta(t, queryMeta)
	must.Eq(t, aclRoleCreateResp, aclRoleReadResp)

	// Read the role using its name.
	aclRoleReadResp, queryMeta, err = testClient.ACLRoles().GetByName(aclRoleCreateResp.Name, nil)
	must.NoError(t, err)
	assertQueryMeta(t, queryMeta)
	must.Eq(t, aclRoleCreateResp, aclRoleReadResp)

	// Update the role name.
	role.Name = "acl-role-api-test-badger-badger-badger"
	role.ID = aclRoleCreateResp.ID
	aclRoleUpdateResp, writeMeta, err := testClient.ACLRoles().Update(&role, nil)
	must.NoError(t, err)
	assertWriteMeta(t, writeMeta)
	must.Eq(t, role.Name, aclRoleUpdateResp.Name)
	must.Eq(t, role.ID, aclRoleUpdateResp.ID)

	// Delete the role.
	writeMeta, err = testClient.ACLRoles().Delete(aclRoleCreateResp.ID, nil)
	must.NoError(t, err)
	assertWriteMeta(t, writeMeta)

	// Make sure there are no ACL roles now present.
	aclRoleListResp, queryMeta, err = testClient.ACLRoles().List(nil)
	must.NoError(t, err)
	must.SliceEmpty(t, aclRoleListResp)
	assertQueryMeta(t, queryMeta)
}

func TestACLAuthMethods(t *testing.T) {
	testutil.Parallel(t)

	testClient, testServer, _ := makeACLClient(t, nil, nil)
	defer testServer.Stop()

	// An initial listing shouldn't return any results.
	aclAuthMethodsListResp, queryMeta, err := testClient.ACLAuthMethods().List(nil)
	must.NoError(t, err)
	must.Len(t, 0, aclAuthMethodsListResp)
	assertQueryMeta(t, queryMeta)

	// Create an ACL auth-method.
	authMethod := ACLAuthMethod{
		Name:          "acl-auth-method-api-test",
		Type:          ACLAuthMethodTypeOIDC,
		TokenLocality: ACLAuthMethodTokenLocalityLocal,
		MaxTokenTTL:   15 * time.Minute,
		Default:       true,
	}
	_, writeMeta, err := testClient.ACLAuthMethods().Create(&authMethod, nil)
	must.NoError(t, err)
	assertWriteMeta(t, writeMeta)

	// Another listing should return one result.
	aclAuthMethodsListResp, queryMeta, err = testClient.ACLAuthMethods().List(nil)
	must.NoError(t, err)
	must.Len(t, 1, aclAuthMethodsListResp)
	must.Eq(t, authMethod.Name, aclAuthMethodsListResp[0].Name)
	must.True(t, aclAuthMethodsListResp[0].Default)
	assertQueryMeta(t, queryMeta)

	// Read the auth-method.
	aclAuthMethodReadResp, queryMeta, err := testClient.ACLAuthMethods().Get(authMethod.Name, nil)
	must.NoError(t, err)
	assertQueryMeta(t, queryMeta)
	must.NotNil(t, aclAuthMethodReadResp)
	must.Eq(t, authMethod.Name, aclAuthMethodReadResp.Name)
	must.Eq(t, authMethod.TokenLocality, aclAuthMethodReadResp.TokenLocality)
	must.Eq(t, authMethod.Type, aclAuthMethodReadResp.Type)

	// Update the auth-method token locality.
	authMethod.TokenLocality = ACLAuthMethodTokenLocalityGlobal
	_, writeMeta, err = testClient.ACLAuthMethods().Update(&authMethod, nil)
	must.NoError(t, err)
	assertWriteMeta(t, writeMeta)

	// Re-read the auth-method and check the locality.
	aclAuthMethodReadResp, queryMeta, err = testClient.ACLAuthMethods().Get(authMethod.Name, nil)
	must.NoError(t, err)
	assertQueryMeta(t, queryMeta)
	must.NotNil(t, aclAuthMethodReadResp)
	must.Eq(t, authMethod.Name, aclAuthMethodReadResp.Name)
	must.Eq(t, authMethod.TokenLocality, aclAuthMethodReadResp.TokenLocality)

	// Delete the role.
	writeMeta, err = testClient.ACLAuthMethods().Delete(authMethod.Name, nil)
	must.NoError(t, err)
	assertWriteMeta(t, writeMeta)

	// Make sure there are no ACL auth-methods now present.
	aclAuthMethodsListResp, queryMeta, err = testClient.ACLAuthMethods().List(nil)
	must.NoError(t, err)
	must.Len(t, 0, aclAuthMethodsListResp)
	assertQueryMeta(t, queryMeta)
}

func TestACLBindingRules(t *testing.T) {
	testutil.Parallel(t)

	testClient, testServer, _ := makeACLClient(t, nil, nil)
	defer testServer.Stop()

	aclAuthMethod := ACLAuthMethod{
		Name:          "auth0",
		Type:          ACLAuthMethodTypeOIDC,
		TokenLocality: ACLAuthMethodTokenLocalityGlobal,
		MaxTokenTTL:   10 * time.Hour,
		Default:       true,
	}
	_, _, err := testClient.ACLAuthMethods().Create(&aclAuthMethod, nil)
	must.NoError(t, err)

	// An initial listing shouldn't return any results.
	aclBindingRulesListResp, queryMeta, err := testClient.ACLBindingRules().List(nil)
	must.NoError(t, err)
	must.Len(t, 0, aclBindingRulesListResp)
	assertQueryMeta(t, queryMeta)

	// Create an ACL auth-method.
	bindingRule := ACLBindingRule{
		Description: "my-binding-rule",
		AuthMethod:  "auth0",
		Selector:    "nomad_engineering_team in list.groups",
		BindType:    "role",
		BindName:    "cluster-admin",
	}
	_, writeMeta, err := testClient.ACLBindingRules().Create(&bindingRule, nil)
	must.NoError(t, err)
	assertWriteMeta(t, writeMeta)

	// Another listing should return one result.
	aclBindingRulesListResp, queryMeta, err = testClient.ACLBindingRules().List(nil)
	must.NoError(t, err)
	must.Len(t, 1, aclBindingRulesListResp)
	must.NotEq(t, "", aclBindingRulesListResp[0].ID)
	must.Eq(t, "auth0", aclBindingRulesListResp[0].AuthMethod)
	assertQueryMeta(t, queryMeta)

	bindingRuleID := aclBindingRulesListResp[0].ID

	// Read the binding rule.
	aclBindingRuleReadResp, queryMeta, err := testClient.ACLBindingRules().Get(bindingRuleID, nil)
	must.NoError(t, err)
	assertQueryMeta(t, queryMeta)
	must.NotNil(t, aclBindingRuleReadResp)
	must.Eq(t, bindingRuleID, aclBindingRuleReadResp.ID)
	must.Eq(t, bindingRule.Description, aclBindingRuleReadResp.Description)
	must.Eq(t, bindingRule.AuthMethod, aclBindingRuleReadResp.AuthMethod)
	must.Eq(t, bindingRule.Selector, aclBindingRuleReadResp.Selector)
	must.Eq(t, bindingRule.BindType, aclBindingRuleReadResp.BindType)
	must.Eq(t, bindingRule.BindName, aclBindingRuleReadResp.BindName)

	// Update the binding rule description.
	bindingRule.ID = bindingRuleID
	bindingRule.Description = "my-binding-rule-updated"
	aclBindingRuleUpdateResp, writeMeta, err := testClient.ACLBindingRules().Update(&bindingRule, nil)
	must.NoError(t, err)
	assertWriteMeta(t, writeMeta)
	must.Eq(t, bindingRuleID, aclBindingRuleUpdateResp.ID)
	must.Eq(t, bindingRule.Description, aclBindingRuleUpdateResp.Description)
	must.Eq(t, bindingRule.AuthMethod, aclBindingRuleUpdateResp.AuthMethod)
	must.Eq(t, bindingRule.Selector, aclBindingRuleUpdateResp.Selector)
	must.Eq(t, bindingRule.BindType, aclBindingRuleUpdateResp.BindType)
	must.Eq(t, bindingRule.BindName, aclBindingRuleUpdateResp.BindName)

	// Delete the role.
	writeMeta, err = testClient.ACLBindingRules().Delete(bindingRuleID, nil)
	must.NoError(t, err)
	assertWriteMeta(t, writeMeta)

	// Make sure there are no ACL auth-methods now present.
	aclBindingRulesListResp, queryMeta, err = testClient.ACLBindingRules().List(nil)
	must.NoError(t, err)
	must.Len(t, 0, aclBindingRulesListResp)
	assertQueryMeta(t, queryMeta)
}
