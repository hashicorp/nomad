package api

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestACLPolicies_ListUpsert(t *testing.T) {
	testutil.Parallel(t)
	c, s, _ := makeACLClient(t, nil, nil)
	defer s.Stop()
	ap := c.ACLPolicies()

	// Listing when nothing exists returns empty
	result, qm, err := ap.List(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if qm.LastIndex != 1 {
		t.Fatalf("bad index: %d", qm.LastIndex)
	}
	if n := len(result); n != 0 {
		t.Fatalf("expected 0 policies, got: %d", n)
	}

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
	assert.Nil(t, err)
	assertWriteMeta(t, wm)

	// Check the list again
	result, qm, err = ap.List(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)
	if len(result) != 1 {
		t.Fatalf("expected policy, got: %#v", result)
	}
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
	assert.Nil(t, err)
	assertWriteMeta(t, wm)

	// Delete the policy
	wm, err = ap.Delete(policy.Name, nil)
	assert.Nil(t, err)
	assertWriteMeta(t, wm)

	// Check the list again
	result, qm, err := ap.List(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)
	if len(result) != 0 {
		t.Fatalf("unexpected policy, got: %#v", result)
	}
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
	assert.Nil(t, err)
	assertWriteMeta(t, wm)

	// Query the policy
	out, qm, err := ap.Info(policy.Name, nil)
	assert.Nil(t, err)
	assertQueryMeta(t, qm)
	assert.Equal(t, policy.Name, out.Name)
}

func TestACLTokens_List(t *testing.T) {
	testutil.Parallel(t)
	c, s, _ := makeACLClient(t, nil, nil)
	defer s.Stop()
	at := c.ACLTokens()

	// Expect the bootstrap token.
	result, qm, err := at.List(nil)
	require.NoError(t, err)
	require.NotEqual(t, 0, qm.LastIndex)
	require.Len(t, result, 1)
	require.Nil(t, result[0].ExpirationTime)

	// Create a token with an expiry.
	token := &ACLToken{
		Name:          "token-with-expiry",
		Type:          "client",
		Policies:      []string{"foo1"},
		ExpirationTTL: 1 * time.Hour,
	}
	createExpirationResp, _, err := at.Create(token, nil)
	require.Nil(t, err)

	// Perform the listing again and ensure we have two entries along with the
	// expiration correctly set and available.
	listResp, qm, err := at.List(nil)
	require.Nil(t, err)
	assertQueryMeta(t, qm)
	require.Len(t, listResp, 2)

	for _, tokenStub := range listResp {
		if tokenStub.AccessorID == createExpirationResp.AccessorID {
			require.NotNil(t, tokenStub.ExpirationTime)
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
	assert.Nil(t, err)
	assertWriteMeta(t, wm)
	assert.NotNil(t, out)

	// Update the token
	out.Name = "other"
	out2, wm, err := at.Update(out, nil)
	assert.Nil(t, err)
	assertWriteMeta(t, wm)
	assert.NotNil(t, out2)

	// Verify the change took hold
	assert.Equal(t, out.Name, out2.Name)

	// Try updating the token to include a TTL which is not allowed.
	out2.ExpirationTTL = 10 * time.Minute
	out3, _, err := at.Update(out2, nil)
	require.Error(t, err)
	require.Nil(t, out3)

	// Try adding a role link to our token, which should be possible. For this
	// we need to create a policy and link to this from a role.
	aclPolicy := ACLPolicy{
		Name:  "acl-role-api-test",
		Rules: `namespace "default" { policy = "read" }`,
	}
	writeMeta, err := c.ACLPolicies().Upsert(&aclPolicy, nil)
	require.NoError(t, err)
	assertWriteMeta(t, writeMeta)

	// Create an ACL role referencing the previously created
	// policy.
	role := ACLRole{
		Name:     "acl-role-api-test",
		Policies: []*ACLRolePolicyLink{{Name: aclPolicy.Name}},
	}
	aclRoleCreateResp, writeMeta, err := c.ACLRoles().Create(&role, nil)
	require.NoError(t, err)
	assertWriteMeta(t, writeMeta)
	require.NotEmpty(t, aclRoleCreateResp.ID)
	require.Equal(t, role.Name, aclRoleCreateResp.Name)

	out2.Roles = []*ACLTokenRoleLink{{Name: aclRoleCreateResp.Name}}
	out2.ExpirationTTL = 0

	out3, _, err = at.Update(out2, nil)
	require.NoError(t, err)
	require.NotNil(t, out3)
	require.Len(t, out3.Policies, 1)
	require.Equal(t, out3.Policies[0], "foo1")
	require.Len(t, out3.Roles, 1)
	require.Equal(t, out3.Roles[0].Name, role.Name)
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
				require.Nil(t, err)
				assertWriteMeta(t, wm)
				require.NotNil(t, out)

				// Query the token
				out2, qm, err := client.ACLTokens().Info(out.AccessorID, nil)
				require.Nil(t, err)
				assertQueryMeta(t, qm)
				require.Equal(t, out, out2)
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
				require.Nil(t, err)
				assertWriteMeta(t, wm)
				require.NotNil(t, out)

				// Query the token and ensure it matches what was returned
				// during the creation as well as ensuring the expiration time
				// is set.
				out2, qm, err := client.ACLTokens().Info(out.AccessorID, nil)
				require.Nil(t, err)
				assertQueryMeta(t, qm)
				require.Equal(t, out, out2)
				require.NotNil(t, out2.ExpirationTime)
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
				require.NoError(t, err)
				assertWriteMeta(t, writeMeta)

				// Create an ACL role referencing the previously created
				// policy.
				role := ACLRole{
					Name:     "acl-role-api-test",
					Policies: []*ACLRolePolicyLink{{Name: aclPolicy.Name}},
				}
				aclRoleCreateResp, writeMeta, err := testClient.ACLRoles().Create(&role, nil)
				require.NoError(t, err)
				assertWriteMeta(t, writeMeta)
				require.NotEmpty(t, aclRoleCreateResp.ID)
				require.Equal(t, role.Name, aclRoleCreateResp.Name)

				// Create a token with a role linking.
				token := &ACLToken{
					Name:  "token-with-role-link",
					Type:  "client",
					Roles: []*ACLTokenRoleLink{{Name: role.Name}},
				}

				out, wm, err := client.ACLTokens().Create(token, nil)
				require.Nil(t, err)
				assertWriteMeta(t, wm)
				require.NotNil(t, out)

				// Query the token and ensure it matches what was returned
				// during the creation.
				out2, qm, err := client.ACLTokens().Info(out.AccessorID, nil)
				require.Nil(t, err)
				assertQueryMeta(t, qm)
				require.Equal(t, out, out2)
				require.Len(t, out.Roles, 1)
				require.Equal(t, out.Roles[0].Name, aclPolicy.Name)
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
				require.NoError(t, err)
				assertWriteMeta(t, writeMeta)

				// Create another that can be referenced within the ACL token
				// directly.
				aclPolicy2 := ACLPolicy{
					Name:  "acl-role-api-test-2",
					Rules: `namespace "fawlty" { policy = "read" }`,
				}
				writeMeta, err = testClient.ACLPolicies().Upsert(&aclPolicy2, nil)
				require.NoError(t, err)
				assertWriteMeta(t, writeMeta)

				// Create an ACL role referencing the previously created
				// policy.
				role := ACLRole{
					Name:     "acl-role-api-test-role-and-policy",
					Policies: []*ACLRolePolicyLink{{Name: aclPolicy1.Name}},
				}
				aclRoleCreateResp, writeMeta, err := testClient.ACLRoles().Create(&role, nil)
				require.NoError(t, err)
				assertWriteMeta(t, writeMeta)
				require.NotEmpty(t, aclRoleCreateResp.ID)
				require.Equal(t, role.Name, aclRoleCreateResp.Name)

				// Create a token with a role linking.
				token := &ACLToken{
					Name:     "token-with-role-and-policy-link",
					Type:     "client",
					Policies: []string{aclPolicy2.Name},
					Roles:    []*ACLTokenRoleLink{{Name: role.Name}},
				}

				out, wm, err := client.ACLTokens().Create(token, nil)
				require.Nil(t, err)
				assertWriteMeta(t, wm)
				require.NotNil(t, out)
				require.Len(t, out.Policies, 1)
				require.Equal(t, out.Policies[0], aclPolicy2.Name)
				require.Len(t, out.Roles, 1)
				require.Equal(t, out.Roles[0].Name, role.Name)

				// Query the token and ensure it matches what was returned
				// during the creation.
				out2, qm, err := client.ACLTokens().Info(out.AccessorID, nil)
				require.Nil(t, err)
				assertQueryMeta(t, qm)
				require.Equal(t, out, out2)
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
	assert.Nil(t, err)
	assertWriteMeta(t, wm)
	assert.NotNil(t, out)

	// Set the clients token to the new token
	c.SetSecretID(out.SecretID)
	at = c.ACLTokens()

	// Query the token
	out2, qm, err := at.Self(nil)
	if assert.Nil(t, err) {
		assertQueryMeta(t, qm)
		assert.Equal(t, out, out2)
	}
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
	assert.Nil(t, err)
	assertWriteMeta(t, wm)
	assert.NotNil(t, out)

	// Delete the token
	wm, err = at.Delete(out.AccessorID, nil)
	assert.Nil(t, err)
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
	assert.Nil(t, err)
	assertWriteMeta(t, wm)
	assert.NotNil(t, out)

	// Get a one-time token
	c.SetSecretID(out.SecretID)
	out2, wm, err := at.UpsertOneTimeToken(nil)
	assert.Nil(t, err)
	assertWriteMeta(t, wm)
	assert.NotNil(t, out2)

	// Exchange the one-time token
	out3, wm, err := at.ExchangeOneTimeToken(out2.OneTimeSecretID, nil)
	assert.Nil(t, err)
	assertWriteMeta(t, wm)
	assert.NotNil(t, out3)
	assert.Equal(t, out3.AccessorID, out.AccessorID)
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
	assert.EqualError(t, err, "Unexpected response code: 400 (invalid acl token)")
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
	assert.NoError(t, err)
	assertWriteMeta(t, wm)
	assert.Equal(t, bootkn, out.SecretID)
}

func TestACLRoles(t *testing.T) {
	testutil.Parallel(t)

	testClient, testServer, _ := makeACLClient(t, nil, nil)
	defer testServer.Stop()

	// An initial listing shouldn't return any results.
	aclRoleListResp, queryMeta, err := testClient.ACLRoles().List(nil)
	require.NoError(t, err)
	require.Empty(t, aclRoleListResp)
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
	require.NoError(t, err)
	assertWriteMeta(t, writeMeta)

	// Create an ACL role referencing the previously created policy.
	role := ACLRole{
		Name:     "acl-role-api-test",
		Policies: []*ACLRolePolicyLink{{Name: aclPolicy.Name}},
	}
	aclRoleCreateResp, writeMeta, err := testClient.ACLRoles().Create(&role, nil)
	require.NoError(t, err)
	assertWriteMeta(t, writeMeta)
	require.NotEmpty(t, aclRoleCreateResp.ID)
	require.Equal(t, role.Name, aclRoleCreateResp.Name)

	// Another listing should return one result.
	aclRoleListResp, queryMeta, err = testClient.ACLRoles().List(nil)
	require.NoError(t, err)
	require.Len(t, aclRoleListResp, 1)
	assertQueryMeta(t, queryMeta)

	// Read the role using its ID.
	aclRoleReadResp, queryMeta, err := testClient.ACLRoles().Get(aclRoleCreateResp.ID, nil)
	require.NoError(t, err)
	assertQueryMeta(t, queryMeta)
	require.Equal(t, aclRoleCreateResp, aclRoleReadResp)

	// Read the role using its name.
	aclRoleReadResp, queryMeta, err = testClient.ACLRoles().GetByName(aclRoleCreateResp.Name, nil)
	require.NoError(t, err)
	assertQueryMeta(t, queryMeta)
	require.Equal(t, aclRoleCreateResp, aclRoleReadResp)

	// Update the role name.
	role.Name = "acl-role-api-test-badger-badger-badger"
	role.ID = aclRoleCreateResp.ID
	aclRoleUpdateResp, writeMeta, err := testClient.ACLRoles().Update(&role, nil)
	require.NoError(t, err)
	assertWriteMeta(t, writeMeta)
	require.Equal(t, role.Name, aclRoleUpdateResp.Name)
	require.Equal(t, role.ID, aclRoleUpdateResp.ID)

	// Delete the role.
	writeMeta, err = testClient.ACLRoles().Delete(aclRoleCreateResp.ID, nil)
	require.NoError(t, err)
	assertWriteMeta(t, writeMeta)

	// Make sure there are no ACL roles now present.
	aclRoleListResp, queryMeta, err = testClient.ACLRoles().List(nil)
	require.NoError(t, err)
	require.Empty(t, aclRoleListResp)
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
	must.SliceNotEmpty(t, aclAuthMethodsListResp[0].Hash)
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
