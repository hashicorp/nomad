// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package devices

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

// TestDeviceScheduling runs end-to-end tests for traditional device scheduling
// (count, constraint, affinity without first_available). These tests require:
// - A Nomad cluster with at least one Linux client
// - The example device plugin (nomad/file/mock) installed and configured
// - Mock device files created in the configured directory
//
// See plugins/device/cmd/example/README.md for setup instructions.
func TestDeviceScheduling(t *testing.T) {
	nomadClient := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomadClient)
	e2eutil.WaitForNodesReady(t, nomadClient, 1)

	// Check if any nodes have mock devices available
	if !hasDevicePlugin(t, nomadClient, "nomad/file/mock") {
		t.Skip("skipping: no nodes with nomad/file/mock device plugin")
	}

	t.Run("testDeviceCountOnly", testDeviceCountOnly)
	t.Run("testDeviceWithConstraint", testDeviceWithConstraint)
	t.Run("testDeviceWithAffinity", testDeviceWithAffinity)
	t.Run("testDeviceWithConstraintAndAffinity", testDeviceWithConstraintAndAffinity)
	t.Run("testDeviceConstraintNoMatch", testDeviceConstraintNoMatch)
}

// hasDevicePlugin checks if any node in the cluster has the specified device
// plugin available.
func hasDevicePlugin(t *testing.T, client *api.Client, deviceName string) bool {
	t.Helper()

	nodes, _, err := client.Nodes().List(nil)
	must.NoError(t, err)

	for _, nodeStub := range nodes {
		node, _, err := client.Nodes().Info(nodeStub.ID, nil)
		must.NoError(t, err)

		if node.NodeResources != nil && node.NodeResources.Devices != nil {
			for _, device := range node.NodeResources.Devices {
				fullName := device.Vendor + "/" + device.Type + "/" + device.Name
				if strings.Contains(fullName, deviceName) ||
					strings.Contains(device.Name, deviceName) {
					return true
				}
			}
		}
	}
	return false
}

// testDeviceCountOnly tests that a job with only device count specified
// can be successfully scheduled.
func testDeviceCountOnly(t *testing.T) {
	nomadClient := e2eutil.NomadClient(t)

	jobID := "device-count-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	allocs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "./input/device_count_only.hcl", jobID, "")
	must.Len(t, 1, allocs, must.Sprint("expected 1 allocation"))

	alloc, _, err := nomadClient.Allocations().Info(allocs[0].ID, nil)
	must.NoError(t, err)
	must.Eq(t, api.AllocClientStatusRunning, alloc.ClientStatus,
		must.Sprintf("allocation status: %s, description: %s",
			alloc.ClientStatus, alloc.ClientDescription))

	// Verify device was allocated
	must.NotNil(t, alloc.AllocatedResources)
	taskResources := alloc.AllocatedResources.Tasks["sleep"]
	must.NotNil(t, taskResources)
	must.SliceNotEmpty(t, taskResources.Devices,
		must.Sprint("expected devices to be allocated"))

	// Verify exactly 1 device
	totalDevices := 0
	for _, deviceResource := range taskResources.Devices {
		totalDevices += len(deviceResource.DeviceIDs)
	}
	must.Eq(t, 1, totalDevices, must.Sprint("expected exactly 1 device"))
}

// testDeviceWithConstraint tests that a job with device count and constraint
// can be successfully scheduled when the constraint is satisfied.
func testDeviceWithConstraint(t *testing.T) {
	nomadClient := e2eutil.NomadClient(t)

	jobID := "device-constraint-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	allocs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "./input/device_with_constraint.hcl", jobID, "")
	must.Len(t, 1, allocs, must.Sprint("expected 1 allocation"))

	alloc, _, err := nomadClient.Allocations().Info(allocs[0].ID, nil)
	must.NoError(t, err)
	must.Eq(t, api.AllocClientStatusRunning, alloc.ClientStatus,
		must.Sprintf("allocation status: %s, description: %s",
			alloc.ClientStatus, alloc.ClientDescription))

	// Verify device was allocated
	must.NotNil(t, alloc.AllocatedResources)
	taskResources := alloc.AllocatedResources.Tasks["sleep"]
	must.NotNil(t, taskResources)
	must.SliceNotEmpty(t, taskResources.Devices,
		must.Sprint("expected devices to be allocated"))
}

// testDeviceWithAffinity tests that a job with device count and affinity
// can be successfully scheduled.
func testDeviceWithAffinity(t *testing.T) {
	nomadClient := e2eutil.NomadClient(t)

	jobID := "device-affinity-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	allocs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "./input/device_with_affinity.hcl", jobID, "")
	must.Len(t, 1, allocs, must.Sprint("expected 1 allocation"))

	alloc, _, err := nomadClient.Allocations().Info(allocs[0].ID, nil)
	must.NoError(t, err)
	must.Eq(t, api.AllocClientStatusRunning, alloc.ClientStatus,
		must.Sprintf("allocation status: %s, description: %s",
			alloc.ClientStatus, alloc.ClientDescription))

	// Verify device was allocated
	must.NotNil(t, alloc.AllocatedResources)
	taskResources := alloc.AllocatedResources.Tasks["sleep"]
	must.NotNil(t, taskResources)
	must.SliceNotEmpty(t, taskResources.Devices,
		must.Sprint("expected devices to be allocated"))
}

// testDeviceWithConstraintAndAffinity tests that a job with device count,
// constraint, and affinity can be successfully scheduled.
func testDeviceWithConstraintAndAffinity(t *testing.T) {
	nomadClient := e2eutil.NomadClient(t)

	jobID := "device-both-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	allocs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "./input/device_with_constraint_and_affinity.hcl", jobID, "")
	must.Len(t, 1, allocs, must.Sprint("expected 1 allocation"))

	alloc, _, err := nomadClient.Allocations().Info(allocs[0].ID, nil)
	must.NoError(t, err)
	must.Eq(t, api.AllocClientStatusRunning, alloc.ClientStatus,
		must.Sprintf("allocation status: %s, description: %s",
			alloc.ClientStatus, alloc.ClientDescription))

	// Verify devices were allocated
	must.NotNil(t, alloc.AllocatedResources)
	taskResources := alloc.AllocatedResources.Tasks["sleep"]
	must.NotNil(t, taskResources)
	must.SliceNotEmpty(t, taskResources.Devices,
		must.Sprint("expected devices to be allocated"))

	// Verify 2 devices were allocated
	totalDevices := 0
	for _, deviceResource := range taskResources.Devices {
		totalDevices += len(deviceResource.DeviceIDs)
	}
	must.Eq(t, 2, totalDevices, must.Sprint("expected exactly 2 devices"))
}

// testDeviceConstraintNoMatch tests that when a device constraint cannot be
// satisfied, the job fails to schedule with appropriate error messages.
func testDeviceConstraintNoMatch(t *testing.T) {
	nomadClient := e2eutil.NomadClient(t)

	jobID := "device-nomatch-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	// Parse and register the job
	job, err := e2eutil.Parse2(t, "./input/device_constraint_no_match.hcl")
	must.NoError(t, err)
	job.ID = &jobID

	resp, _, err := nomadClient.Jobs().Register(job, nil)
	must.NoError(t, err)

	evalID := resp.EvalID

	// Wait for the evaluation to complete (it should fail to place)
	var eval *api.Evaluation
	testutil.WaitForResultRetries(30, func() (bool, error) {
		time.Sleep(500 * time.Millisecond)
		eval, _, err = nomadClient.Evaluations().Info(evalID, nil)
		if err != nil {
			return false, err
		}
		if eval.Status == api.EvalStatusComplete || eval.Status == api.EvalStatusBlocked {
			return true, nil
		}
		return false, fmt.Errorf("eval status: %s", eval.Status)
	}, func(err error) {
		must.NoError(t, err)
	})

	// The evaluation should have failed task group allocations
	must.MapNotEmpty(t, eval.FailedTGAllocs,
		must.Sprint("expected failed task group allocations"))

	// Check that the failure is due to device exhaustion or constraint filtering
	for _, metrics := range eval.FailedTGAllocs {
		exhausted := metrics.NodesExhausted > 0 ||
			len(metrics.DimensionExhausted) > 0 ||
			len(metrics.ConstraintFiltered) > 0
		must.True(t, exhausted,
			must.Sprintf("expected device exhaustion, got metrics: %+v", metrics))
	}
}

// TestDeviceParsing tests that traditional device configurations (count,
// constraint, affinity) are parsed correctly. These are unit-style tests
// that don't require a running Nomad cluster.
func TestDeviceParsing(t *testing.T) {
	t.Run("testParseDeviceCountOnly", testParseDeviceCountOnly)
	t.Run("testParseDeviceWithConstraint", testParseDeviceWithConstraint)
	t.Run("testParseDeviceWithAffinity", testParseDeviceWithAffinity)
	t.Run("testParseDeviceWithConstraintAndAffinity", testParseDeviceWithConstraintAndAffinity)
}

// testParseDeviceCountOnly verifies parsing of a device with only count.
func testParseDeviceCountOnly(t *testing.T) {
	job, err := e2eutil.Parse2(t, "./input/device_count_only.hcl")
	must.NoError(t, err)
	must.NotNil(t, job)

	must.Len(t, 1, job.TaskGroups)
	task := job.TaskGroups[0].Tasks[0]
	must.NotNil(t, task.Resources)
	must.Len(t, 1, task.Resources.Devices)

	device := task.Resources.Devices[0]
	must.Eq(t, "nomad/file/mock", device.Name)
	must.Eq(t, uint64(1), *device.Count)
	must.Len(t, 0, device.Constraints)
	must.Len(t, 0, device.Affinities)
	must.Len(t, 0, device.FirstAvailable)
}

// testParseDeviceWithConstraint verifies parsing of a device with count and constraint.
func testParseDeviceWithConstraint(t *testing.T) {
	job, err := e2eutil.Parse2(t, "./input/device_with_constraint.hcl")
	must.NoError(t, err)
	must.NotNil(t, job)

	task := job.TaskGroups[0].Tasks[0]
	device := task.Resources.Devices[0]

	must.Eq(t, "nomad/file/mock", device.Name)
	must.Eq(t, uint64(1), *device.Count)
	must.Len(t, 1, device.Constraints)
	must.Eq(t, "${device.attr.type}", device.Constraints[0].LTarget)
	must.Eq(t, "file", device.Constraints[0].RTarget)
	must.Len(t, 0, device.Affinities)
	must.Len(t, 0, device.FirstAvailable)
}

// testParseDeviceWithAffinity verifies parsing of a device with count and affinity.
func testParseDeviceWithAffinity(t *testing.T) {
	job, err := e2eutil.Parse2(t, "./input/device_with_affinity.hcl")
	must.NoError(t, err)
	must.NotNil(t, job)

	task := job.TaskGroups[0].Tasks[0]
	device := task.Resources.Devices[0]

	must.Eq(t, "nomad/file/mock", device.Name)
	must.Eq(t, uint64(1), *device.Count)
	must.Len(t, 0, device.Constraints)
	must.Len(t, 1, device.Affinities)
	must.Eq(t, "${device.attr.priority}", device.Affinities[0].LTarget)
	must.Eq(t, "high", device.Affinities[0].RTarget)
	must.Eq(t, int8(100), *device.Affinities[0].Weight)
	must.Len(t, 0, device.FirstAvailable)
}

// testParseDeviceWithConstraintAndAffinity verifies parsing of a device with
// count, constraint, and affinity.
func testParseDeviceWithConstraintAndAffinity(t *testing.T) {
	job, err := e2eutil.Parse2(t, "./input/device_with_constraint_and_affinity.hcl")
	must.NoError(t, err)
	must.NotNil(t, job)

	task := job.TaskGroups[0].Tasks[0]
	device := task.Resources.Devices[0]

	must.Eq(t, "nomad/file/mock", device.Name)
	must.Eq(t, uint64(2), *device.Count)

	// Verify constraint
	must.Len(t, 1, device.Constraints)
	must.Eq(t, "${device.attr.type}", device.Constraints[0].LTarget)
	must.Eq(t, "file", device.Constraints[0].RTarget)

	// Verify affinity
	must.Len(t, 1, device.Affinities)
	must.Eq(t, "${device.attr.priority}", device.Affinities[0].LTarget)
	must.Eq(t, "high", device.Affinities[0].RTarget)
	must.Eq(t, int8(50), *device.Affinities[0].Weight)

	// No first_available
	must.Len(t, 0, device.FirstAvailable)
}
