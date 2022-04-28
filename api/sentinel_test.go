// +build ent

package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSentinelPolicies_ListUpsert(t *testing.T) {
	t.Parallel()
	c, s, _ := makeACLClient(t, nil, nil)
	defer s.Stop()
	ap := c.SentinelPolicies()

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
	policy := &SentinelPolicy{
		Name:             "test",
		Description:      "test",
		EnforcementLevel: "advisory",
		Scope:            "submit-job",
		Policy:           "main = rule { true }",
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

func TestSentinelPolicies_Delete(t *testing.T) {
	t.Parallel()
	c, s, _ := makeACLClient(t, nil, nil)
	defer s.Stop()
	ap := c.SentinelPolicies()

	// Register a policy
	policy := &SentinelPolicy{
		Name:             "test",
		Description:      "test",
		EnforcementLevel: "advisory",
		Scope:            "submit-job",
		Policy:           "main = rule { true } ",
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

func TestSentinelPolicies_Info(t *testing.T) {
	t.Parallel()
	c, s, _ := makeACLClient(t, nil, nil)
	defer s.Stop()
	ap := c.SentinelPolicies()

	// Register a policy
	policy := &SentinelPolicy{
		Name:             "test",
		Description:      "test",
		EnforcementLevel: "advisory",
		Scope:            "submit-job",
		Policy:           "main = rule { true }",
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
