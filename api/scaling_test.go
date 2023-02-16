package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestScalingPolicies_ListPolicies(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	scaling := c.Scaling()
	jobs := c.Jobs()

	// Check that we don't have any scaling policies before registering a job that has one
	policies, _, err := scaling.ListPolicies(nil)
	must.NoError(t, err)
	must.SliceEmpty(t, policies)

	// Register a job with a scaling policy
	job := testJob()
	job.TaskGroups[0].Scaling = &ScalingPolicy{
		Max: pointerOf(int64(100)),
	}
	_, _, err = jobs.Register(job, nil)
	must.NoError(t, err)

	// Check that we have a scaling policy now
	policies, _, err = scaling.ListPolicies(nil)
	must.NoError(t, err)
	must.Len(t, 1, policies)

	policy := policies[0]

	// Check that the scaling policy references the right namespace
	namespace := DefaultNamespace
	if job.Namespace != nil && *job.Namespace != "" {
		namespace = *job.Namespace
	}
	must.Eq(t, policy.Target["Namespace"], namespace)

	// Check that the scaling policy references the right job
	must.Eq(t, policy.Target["Job"], *job.ID)

	// Check that the scaling policy references the right group
	must.Eq(t, policy.Target["Group"], *job.TaskGroups[0].Name)

	// Check that the scaling policy has the right type
	must.Eq(t, ScalingPolicyTypeHorizontal, policy.Type)
}

func TestScalingPolicies_GetPolicy(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	scaling := c.Scaling()
	jobs := c.Jobs()

	// Empty ID should return 404
	_, _, err := scaling.GetPolicy("", nil)
	must.ErrorContains(t, err, "404")

	// Non-existent ID should return 404
	_, _, err = scaling.GetPolicy("i-do-not-exist", nil)
	must.ErrorContains(t, err, "404")

	// Register a job with a scaling policy
	job := testJob()
	policy := &ScalingPolicy{
		Enabled: pointerOf(true),
		Min:     pointerOf(int64(1)),
		Max:     pointerOf(int64(1)),
		Policy: map[string]interface{}{
			"key": "value",
		},
	}
	job.TaskGroups[0].Scaling = policy
	_, _, err = jobs.Register(job, nil)
	must.NoError(t, err)

	// Find newly created scaling policy ID
	var policyID string
	policies, _, err := scaling.ListPolicies(nil)
	must.NoError(t, err)
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
	must.NoError(t, err)

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
	must.Eq(t, expectedTarget, resp.Target)
	must.Eq(t, policy.Policy, resp.Policy)
	must.Eq(t, policy.Enabled, resp.Enabled)
	must.Eq(t, *policy.Min, *resp.Min)
	must.Eq(t, policy.Max, resp.Max)
	must.Eq(t, ScalingPolicyTypeHorizontal, resp.Type)
}
