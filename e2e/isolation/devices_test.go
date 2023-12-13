// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package isolation

import (
	"testing"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
)

func TestCgroupDevices(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 1)

	t.Run("testDevicesUsable", testDevicesUsable)
}

func testDevicesUsable(t *testing.T) {
	nomad := e2eutil.NomadClient(t)

	jobID := "cgroup-devices-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	// start job
	allocs := e2eutil.RegisterAndWaitForAllocs(t, nomad, "./input/cgroup_devices.hcl", jobID, "")
	must.Len(t, 2, allocs)

	// pick one to stop and one to verify
	allocA := allocs[0].ID
	allocB := allocs[1].ID

	// verify devices are working
	checkDev(t, allocA)
	checkDev(t, allocB)

	// stop the chosen alloc
	_, err := e2eutil.Command("nomad", "alloc", "stop", "-detach", allocA)
	must.NoError(t, err)
	e2eutil.WaitForAllocStopped(t, nomad, allocA)

	// verify device of remaining alloc
	checkDev(t, allocB)
}

func checkDev(t *testing.T, allocID string) {
	_, err := e2eutil.Command("nomad", "alloc", "exec", allocID, "dd", "if=/dev/zero", "of=/dev/null", "count=1")
	must.NoError(t, err)
}
