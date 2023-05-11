// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build ent

package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestSentinelPolicies_ListUpsert(t *testing.T) {
	testutil.Parallel(t)
	c, s, _ := makeACLClient(t, nil, nil)
	defer s.Stop()
	ap := c.SentinelPolicies()

	// Listing when nothing exists returns empty
	result, qm, err := ap.List(nil)
	must.NoError(t, err)
	must.Positive(t, qm.LastIndex)
	must.SliceEmpty(t, result)

	// Register a policy
	policy := &SentinelPolicy{
		Name:             "test",
		Description:      "test",
		EnforcementLevel: "advisory",
		Scope:            "submit-job",
		Policy:           "main = rule { true }",
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

func TestSentinelPolicies_Delete(t *testing.T) {
	testutil.Parallel(t)

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
	must.SliceEmpty(t, result)
}

func TestSentinelPolicies_Info(t *testing.T) {
	testutil.Parallel(t)

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
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Query the policy
	out, qm, err := ap.Info(policy.Name, nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.Eq(t, policy.Name, out.Name)
}
