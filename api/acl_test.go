package api

import (
	"testing"

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

	// Expect out bootstrap token
	result, qm, err := at.List(nil)

	must.NoError(t, err)
	must.NonZero(t, qm.LastIndex)
	must.Len(t, 1, result)
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
}

func TestACLTokens_Info(t *testing.T) {
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

	// Query the token
	out2, qm, err := at.Info(out.AccessorID, nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.Eq(t, out, out2)
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
