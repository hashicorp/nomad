package api

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScalingPolicies_List(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	scaling := c.Scaling()
	jobs := c.Jobs()

	// Check that we don't have any scaling policies before registering a job that has one
	policies, _, err := scaling.ListPolicies(nil)
	require.NoError(err)
	require.Empty(policies, "expected 0 scaling policies, got: %d", len(policies))

	// Register a job with a scaling policy
	job := testJob()
	job.TaskGroups[0].Scaling = &ScalingPolicy{}
	_, _, err = jobs.Register(job, nil)
	require.NoError(err)

	// Check that we have a scaling policy now
	policies, _, err = scaling.ListPolicies(nil)
	require.NoError(err)
	if len(policies) != 1 {
		t.Fatalf("expected 1 scaling policy, got: %d", len(policies))
	}

	policy := policies[0]

	// Check that the scaling policy references the right job
	require.Equalf(policy.JobID, *job.ID, "expected JobID=%s, got: %s", *job.ID, policy.JobID)

	// Check that the scaling policy references the right target
	expectedTarget := fmt.Sprintf("/v1/job/%s/%s/scale", *job.ID, *job.TaskGroups[0].Name)
	require.Equalf(expectedTarget, policy.Target, "expected Target=%s, got: %s", expectedTarget, policy.Target)
}

func TestScalingPolicies_GetPolicy(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	scaling := c.Scaling()
	jobs := c.Jobs()

	// Empty ID should return 404
	_, _, err := scaling.GetPolicy("", nil)
	require.Error(err)
	require.Containsf(err.Error(), "404", "expected 404 error, got: %s", err.Error())

	// Inexistent ID should return 404
	_, _, err = scaling.GetPolicy("i-dont-exist", nil)
	require.Error(err)
	require.Containsf(err.Error(), "404", "expected 404 error, got: %s", err.Error())

	// Register a job with a scaling policy
	job := testJob()
	policy := &ScalingPolicy{
		Enabled: boolToPtr(true),
		Policy: map[string]interface{}{
			"key": "value",
		},
	}
	job.TaskGroups[0].Scaling = policy
	_, _, err = jobs.Register(job, nil)
	require.NoError(err)

	// Find newly created scaling policy ID
	var policyID string
	policies, _, err := scaling.ListPolicies(nil)
	require.NoError(err)
	for _, p := range policies {
		if p.JobID == *job.ID {
			policyID = p.ID
			break
		}
	}
	if policyID == "" {
		t.Fatalf("unable to find scaling policy for job %s", *job.ID)
	}

	// Fetch scaling policy
	resp, _, err := scaling.GetPolicy(policyID, nil)
	require.NoError(err)

	// Check that the scaling policy fields match
	expectedTarget := fmt.Sprintf("/v1/job/%s/%s/scale", *job.ID, *job.TaskGroups[0].Name)
	require.Equalf(expectedTarget, resp.Target, "expected Target=%s, got: %s", expectedTarget, policy.Target)
	require.Equal(policy.Policy, resp.Policy)
	require.Equal(policy.Enabled, resp.Enabled)
}
