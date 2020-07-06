package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScalingPolicies_ListPolicies(t *testing.T) {
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
	job.TaskGroups[0].Scaling = &ScalingPolicy{
		Max: int64ToPtr(100),
	}
	_, _, err = jobs.Register(job, nil)
	require.NoError(err)

	// Check that we have a scaling policy now
	policies, _, err = scaling.ListPolicies(nil)
	require.NoError(err)
	if len(policies) != 1 {
		t.Fatalf("expected 1 scaling policy, got: %d", len(policies))
	}

	policy := policies[0]

	// Check that the scaling policy references the right namespace
	namespace := DefaultNamespace
	if job.Namespace != nil && *job.Namespace != "" {
		namespace = *job.Namespace
	}
	require.Equal(policy.Target["Namespace"], namespace)

	// Check that the scaling policy references the right job
	require.Equal(policy.Target["Job"], *job.ID)

	// Check that the scaling policy references the right group
	require.Equal(policy.Target["Group"], *job.TaskGroups[0].Name)
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
		Min:     int64ToPtr(1),
		Max:     int64ToPtr(1),
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
		if p.Target["Job"] == *job.ID {
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
	namespace := DefaultNamespace
	if job.Namespace != nil && *job.Namespace != "" {
		namespace = *job.Namespace
	}
	expectedTarget := map[string]string{
		"Namespace": namespace,
		"Job":       *job.ID,
		"Group":     *job.TaskGroups[0].Name,
	}
	require.Equal(expectedTarget, resp.Target)
	require.Equal(policy.Policy, resp.Policy)
	require.Equal(policy.Enabled, resp.Enabled)
	require.Equal(*policy.Min, *resp.Min)
	require.Equal(policy.Max, resp.Max)
}
