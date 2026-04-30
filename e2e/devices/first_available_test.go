// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package devices

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

// TestDeviceFirstAvailable runs end-to-end tests for the first_available
// device scheduling feature. These tests require:
// - A Nomad cluster with at least one Linux client
// - The example device plugin (nomad/file/mock) installed and configured
// - Mock device files created in the configured directory
//
// See plugins/device/cmd/example/README.md for setup instructions.
func TestDeviceFirstAvailable(t *testing.T) {
	nomadClient := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomadClient)
	e2eutil.WaitForNodesReady(t, nomadClient, 1)

	// Check if any nodes have mock devices available
	if !hasDevicePlugin(t, nomadClient, "nomad/file/mock") {
		t.Skip("skipping: no nodes with nomad/file/mock device plugin")
	}

	t.Run("testFirstAvailableSelectsCorrectOption", testFirstAvailableSelectsCorrectOption)
	t.Run("testFirstAvailableNoMatch", testFirstAvailableNoMatch)
}

// testFirstAvailableSelectsCorrectOption tests that first_available correctly
// evaluates options in order and selects the appropriate one. The first option
// has an impossible constraint (should fail), so the scheduler must fall back
// to the second option. We verify by checking that exactly 2 devices were
// allocated (second option's count), not 1 (first option's count) or 3 (third
// option's count).
func testFirstAvailableSelectsCorrectOption(t *testing.T) {
	nomadClient := e2eutil.NomadClient(t)

	jobID := "device-fa-second-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	// Register the job - first option has impossible constraint (should fail),
	// second option requests 2 devices (should be selected)
	allocs := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "./input/first_available_with_basic.hcl", jobID, "")
	must.Len(t, 1, allocs, must.Sprint("expected 1 allocation"))

	// Verify the allocation is running (fallback to second option succeeded)
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

	// Count total devices allocated - should be 2 (second option), not 1 (first option)
	totalDevices := 0
	for _, deviceResource := range taskResources.Devices {
		totalDevices += len(deviceResource.DeviceIDs)
	}
	must.Eq(t, 2, totalDevices,
		must.Sprint("expected exactly 2 devices from SECOND option, got different count indicating wrong option selected"))
}

// testFirstAvailableNoMatch tests that when no first_available options can be
// satisfied, the job fails to schedule with appropriate error messages.
func testFirstAvailableNoMatch(t *testing.T) {
	nomadClient := e2eutil.NomadClient(t)

	jobID := "device-fa-nomatch-" + uuid.Short()
	jobIDs := []string{jobID}
	t.Cleanup(e2eutil.CleanupJobsAndGC(t, &jobIDs))

	// Parse and register the job
	job, err := e2eutil.Parse2(t, "./input/first_available_no_match.hcl")
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
		// Wait until eval is complete or blocked
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

	// Check that the failure is due to device exhaustion
	for _, metrics := range eval.FailedTGAllocs {
		// Should see nodes exhausted or constraint filtered
		exhausted := metrics.NodesExhausted > 0 ||
			len(metrics.DimensionExhausted) > 0 ||
			len(metrics.ConstraintFiltered) > 0
		must.True(t, exhausted,
			must.Sprintf("expected device exhaustion, got metrics: %+v", metrics))
	}
}

// TestDeviceFirstAvailableParsing tests that jobs with first_available blocks
// are parsed correctly. These are unit-style tests that don't require a
// running Nomad cluster.
func TestDeviceFirstAvailableParsing(t *testing.T) {
	t.Run("testParseFirstAvailable", testParseFirstAvailable)
	t.Run("testParseWithBaseConstraint", testParseWithBaseConstraint)
}

// testParseFirstAvailable verifies parsing of first_available with multiple
// options including constraints.
func testParseFirstAvailable(t *testing.T) {
	job, err := e2eutil.Parse2(t, "./input/first_available_with_basic.hcl")
	must.NoError(t, err)
	must.NotNil(t, job)

	// Verify the structure was parsed correctly
	must.Len(t, 1, job.TaskGroups)
	task := job.TaskGroups[0].Tasks[0]
	must.NotNil(t, task.Resources)
	must.Len(t, 1, task.Resources.Devices)

	device := task.Resources.Devices[0]
	must.Eq(t, "nomad/file/mock", device.Name)
	must.Len(t, 3, device.FirstAvailable,
		must.Sprint("expected 3 first_available options"))

	// Verify first option: count=1, with impossible constraint
	opt1 := device.FirstAvailable[0]
	must.Eq(t, uint64(1), *opt1.Count)
	must.Len(t, 1, opt1.Constraints)
	must.Eq(t, "${device.attr.impossible_attr}", opt1.Constraints[0].LTarget)
	must.Eq(t, "impossible_value", opt1.Constraints[0].RTarget)

	// Verify second option: count=2, no constraints
	opt2 := device.FirstAvailable[1]
	must.Eq(t, uint64(2), *opt2.Count)
	must.Len(t, 0, opt2.Constraints)

	// Verify third option: count=3, no constraints
	opt3 := device.FirstAvailable[2]
	must.Eq(t, uint64(3), *opt3.Count)
	must.Len(t, 0, opt3.Constraints)
}

// testParseWithBaseConstraint verifies parsing with base and option constraints.
func testParseWithBaseConstraint(t *testing.T) {
	job, err := e2eutil.Parse2(t, "./input/first_available_with_base_constraint.hcl")
	must.NoError(t, err)
	must.NotNil(t, job)

	task := job.TaskGroups[0].Tasks[0]
	device := task.Resources.Devices[0]

	// Verify base constraint exists
	must.Len(t, 1, device.Constraints,
		must.Sprint("expected 1 base constraint"))
	must.Eq(t, "${device.attr.cool-attribute}", device.Constraints[0].LTarget)

	// Verify first_available options also have their own constraints
	must.Len(t, 2, device.FirstAvailable)
	must.Len(t, 1, device.FirstAvailable[0].Constraints)
	must.Len(t, 1, device.FirstAvailable[1].Constraints)
}
