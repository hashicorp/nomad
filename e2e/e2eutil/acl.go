// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package e2eutil

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

// ApplyJobPolicy applies an ACL job policy or noops if ACLs are disabled.
// Registers a cleanup function to delete the policy.
func ApplyJobPolicy(t *testing.T, nomad *api.Client, ns, j, g, task, rules string) *api.ACLPolicy {

	policy := &api.ACLPolicy{
		Name: j + uuid.Short(),
		Description: fmt.Sprintf("Policy for test=%s ns=%s job=%s group=%s task=%s rules=%s",
			t.Name(), ns, j, g, task, rules),
		Rules: rules,
		JobACL: &api.JobACL{
			Namespace: ns,
			JobID:     j,
			Group:     g,
			Task:      task,
		},
	}

	wm, err := nomad.ACLPolicies().Upsert(policy, nil)
	if err != nil {
		if strings.Contains(err.Error(), "ACL support disabled") {
			t.Logf("ACL support disabled. Skipping ApplyJobPolicy(t, c, %q, %q, %q, %q, %q)",
				ns, j, g, task, rules)
			return nil
		}
		must.NoError(t, err)
	}

	t.Cleanup(func() {
		_, err := nomad.ACLPolicies().Delete(policy.Name, nil)
		test.NoError(t, err)
	})

	policy.CreateIndex = wm.LastIndex
	policy.ModifyIndex = wm.LastIndex
	return policy
}
