// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package apitests

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/stretchr/testify/assert"
)

func TestJobs_Parse(t *testing.T) {
	ci.Parallel(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	jobs := c.Jobs()

	checkJob := func(job *api.Job, expectedRegion string) {
		if job == nil {
			t.Fatal("job should not be nil")
		}

		region := job.Region

		if region == nil {
			if expectedRegion != "" {
				t.Fatalf("expected job region to be '%s' but was unset", expectedRegion)
			}
		} else {
			if expectedRegion != *region {
				t.Fatalf("expected job region '%s', but got '%s'", expectedRegion, *region)
			}
		}
	}
	job, err := jobs.ParseHCL(mock.HCL(), true)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	checkJob(job, "global")

	job, err = jobs.ParseHCL(mock.HCL(), false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	checkJob(job, "")
}

func TestJobs_Summary_WithACL(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	c, s, root := makeACLClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	invalidToken := mock.ACLToken()

	// Registering with an invalid  token should fail
	c.SetSecretID(invalidToken.SecretID)
	job := testJob()
	_, _, err := jobs.Register(job, nil)
	assert.NotNil(err)

	// Register with token should succeed
	c.SetSecretID(root.SecretID)
	resp2, wm, err := jobs.Register(job, nil)
	assert.Nil(err)
	assert.NotNil(resp2)
	assert.NotEqual("", resp2.EvalID)
	assertWriteMeta(t, wm)

	// Query the job summary with an invalid token should fail
	c.SetSecretID(invalidToken.SecretID)
	result, _, err := jobs.Summary(*job.ID, nil)
	assert.NotNil(err)

	// Query the job summary with a valid token should succeed
	c.SetSecretID(root.SecretID)
	result, qm, err := jobs.Summary(*job.ID, nil)
	assert.Nil(err)
	assertQueryMeta(t, qm)

	// Check that the result is what we expect
	assert.Equal(*job.ID, result.JobID)
}
