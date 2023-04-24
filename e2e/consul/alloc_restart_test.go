// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
)

func testAllocRestart(t *testing.T) {
	nc := e2eutil.NomadClient(t)
	cc := e2eutil.ConsulClient(t).Catalog()

	const jobFile = "./input/alloc_restart.hcl"
	jobID := "alloc-restart-" + uuid.Short()
	jobIDs := []string{jobID}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer e2eutil.CleanupJobsAndGCWithContext(t, ctx, &jobIDs)

	// register our job
	err := e2eutil.Register(jobID, jobFile)
	must.NoError(t, err)

	// wait for our allocation to be running
	allocID := e2eutil.SingleAllocID(t, jobID, "", 0)
	e2eutil.WaitForAllocRunning(t, nc, allocID)

	// make sure our service is registered
	services, _, err := cc.Service("alloc-restart-http", "", nil)
	must.NoError(t, err)
	must.Len(t, 1, services)

	// restart the alloc
	stderr, err := e2eutil.Command("nomad", "alloc", "restart", allocID)
	must.NoError(t, err, must.Sprintf("stderr: %s", stderr))

	// wait for alloc running again
	e2eutil.WaitForAllocRunning(t, nc, allocID)

	// make sure our service is still registered
	services, _, err = cc.Service("alloc-restart-http", "", nil)
	must.NoError(t, err)
	must.Len(t, 1, services)

	err = e2eutil.StopJob(jobID)
	must.NoError(t, err)

	// make sure our service is no longer registered
	services, _, err = cc.Service("alloc-restart-http", "", nil)
	must.NoError(t, err)
	must.SliceEmpty(t, services)
}
