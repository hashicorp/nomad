// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

// testServiceReversion asserts we can
// - submit a job with a service
// - update that job and modify service
// - revert the job, restoring the original service
func testServiceReversion(t *testing.T) {
	const jobFile = "./input/service_reversion.nomad"
	jobID := "service-reversion-" + uuid.Short()
	jobIDs := []string{jobID}

	// Defer a cleanup function to remove the job. This will trigger if the
	// test fails, unless the cancel function is called.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer e2eutil.CleanupJobsAndGCWithContext(t, ctx, &jobIDs)

	// initial register of job, with service="one"
	vars := []string{"-var", "service=one"}
	err := e2eutil.RegisterWithArgs(jobID, jobFile, vars...)
	must.NoError(t, err)

	// wait for job to be running
	err = e2eutil.WaitForAllocStatusExpected(jobID, "", []string{structs.AllocClientStatusRunning})
	must.NoError(t, err)

	// get our consul client
	consulClient := e2eutil.ConsulClient(t)

	assertService := func(name string, count int) {
		services, _, consulErr := consulClient.Catalog().Service(name, "", nil)
		must.NoError(t, consulErr)
		must.Len(t, count, services, must.Sprintf("expected %d instances of %s, got %d", count, name, len(services)))
	}

	// query services, assert 1 instance of "one"
	assertService("one", 1)
	assertService("two", 0)

	// second register of job, with service="two"
	vars = []string{"-var", "service=two"}
	err = e2eutil.RegisterWithArgs(jobID, jobFile, vars...)
	must.NoError(t, err)

	// wait for job to be running
	err = e2eutil.WaitForAllocStatusExpected(jobID, "", []string{structs.AllocClientStatusRunning})
	must.NoError(t, err)

	// query services, assert 0 instance of "one" (replaced), 1 of "two"
	assertService("one", 0)
	assertService("two", 1)

	// now revert our job back to version 0
	err = e2eutil.Revert(jobID, jobFile, 0)
	must.NoError(t, err)

	// wait for job to be running
	err = e2eutil.WaitForAllocStatusExpected(jobID, "", []string{structs.AllocClientStatusRunning})
	must.NoError(t, err)

	// query services, assert 1 instance of "one" (reverted), 1 of "two" (removed)
	assertService("one", 1)
	assertService("two", 0)
}
