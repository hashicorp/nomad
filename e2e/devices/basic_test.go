// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package devices

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/execagent"
	"github.com/hashicorp/nomad/helper/discover"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

const (
	envGate = "NOMAD_E2E_DEVICE_SCHEDULING"
)

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

// TestDeviceScheduling runs device scheduling integration tests.
// These tests require the static example device plugin (nomad/file/mock) installed and configured

func TestDeviceScheduling(t *testing.T) {
	if os.Getenv(envGate) != "1" {
		t.Skip(envGate + " is not set; skipping")
	}
	cases := []struct {
		name            string
		jobFile         string
		expectedAllocs  int
		expectedDevices int
	}{
		{
			name:            "deviceCountOnly",
			jobFile:         "./input/device_count_only.hcl",
			expectedAllocs:  1,
			expectedDevices: 1,
		},
		{
			name:            "deviceWithConstraint",
			jobFile:         "./input/device_with_constraint.hcl",
			expectedAllocs:  1,
			expectedDevices: 1,
		},
		{
			name:            "deviceWithAffinity",
			jobFile:         "./input/device_with_affinity.hcl",
			expectedAllocs:  1,
			expectedDevices: 1,
		},
		{
			name:            "deviceWithConstraintAndAffinity",
			jobFile:         "./input/device_with_constraint_and_affinity.hcl",
			expectedAllocs:  1,
			expectedDevices: 2,
		},
		{
			name:    "deviceConstraintNoMatch",
			jobFile: "./input/device_constraint_no_match.hcl",
		},
		{
			name:            "firstAvailableBasic",
			jobFile:         "./input/first_available_with_basic.hcl",
			expectedAllocs:  1,
			expectedDevices: 2,
		},
		{
			name:            "firstAvailableBaseConstraint",
			jobFile:         "./input/first_available_with_base_constraint.hcl",
			expectedAllocs:  1,
			expectedDevices: 1,
		},
		{
			name:    "firstAvailableNoMatch",
			jobFile: "./input/first_available_no_match.hcl",
		},
	}
	nomadBinary, err := discover.NomadExecutable()
	must.NoError(t, err)
	must.FileExists(t, nomadBinary)

	agentCallbackFn := func(c *execagent.AgentTemplateVars) {
		c.AgentName = "device-test"
		c.LogLevel = hclog.Debug.String()
	}

	c, err := os.ReadFile("./input/basic_device_config.hcl")
	must.NoError(t, err)
	cfg := string(c)

	testServer, err := execagent.NewSingleModeAgent(
		nomadBinary,
		t.TempDir(),
		cfg,
		execagent.ModeBoth,
		nil,
		agentCallbackFn,
	)

	err = testServer.Start()
	must.NoError(t, err)
	t.Cleanup(func() { _ = testServer.Destroy() })

	nomadClient, err := testServer.Client()
	must.NoError(t, err)
	must.NotNil(t, nomadClient)
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			keyList, _, err := nomadClient.Keyring().List(nil)
			if err != nil {
				return err
			}
			if len(keyList) == 0 {
				return errors.New("no keys found")
			}
			return nil
		}),
		wait.Timeout(5*time.Second),
		wait.Gap(100*time.Millisecond),
	))

	for _, tc := range cases {

		t.Run(tc.name, func(t *testing.T) {
			// Check if any nodes have mock devices available
			if !hasDevicePlugin(t, nomadClient, "nomad/file/mock") {
				t.Skip("skipping: no nodes with nomad/file/mock device plugin")
			}
			expectedAllocs := 0
			expectedDevices := 0
			if tc.expectedAllocs != 0 {
				expectedAllocs = tc.expectedAllocs
			}

			if tc.expectedDevices != 0 {
				expectedDevices = tc.expectedDevices
			}
			jobID := "dc-" + tc.name + "_" + uuid.Short()
			t.Cleanup(func() { nomadClient.Jobs().Deregister(jobID, true, nil) })
			spec, err := os.ReadFile(tc.jobFile)
			must.NoError(t, err)

			job, err := nomadClient.Jobs().ParseHCLOpts(&api.JobsParseRequest{
				JobHCL:       string(spec),
				Canonicalize: true,
			})
			must.NoError(t, err)

			// Set custom job ID (distinguish among tests)
			job.ID = new(jobID)
			var idx uint64
			jobs := nomadClient.Jobs()

			resp, meta, err := jobs.Register(job, nil)
			must.NoError(t, err)
			idx = meta.LastIndex
			must.NotNil(t, nomadClient)
			must.NotEq(t, resp.EvalID, "")

			if expectedAllocs != 0 {
				var alloc *api.Allocation
				must.Wait(t, wait.InitialSuccess(
					wait.ErrorFunc(func() error {
						allocs, _, err := jobs.Allocations(jobID, false, &api.QueryOptions{WaitIndex: idx})
						must.NoError(t, err)
						must.Len(t, expectedAllocs, allocs, must.Sprintf("expected %d allocations", expectedAllocs))

						alloc, _, err = nomadClient.Allocations().Info(allocs[expectedAllocs-1].ID, nil)
						must.NoError(t, err)
						must.NotEq(t, api.AllocClientStatusFailed, alloc.ClientStatus)
						// Verify device was allocated
						must.NotNil(t, alloc.AllocatedResources)

						return nil
					}),
					wait.Timeout(5*time.Second),
					wait.Gap(100*time.Millisecond),
				))
				taskResources := alloc.AllocatedResources.Tasks["sleep"]
				must.NotNil(t, taskResources)
				must.SliceNotEmpty(t, taskResources.Devices,
					must.Sprint("expected devices to be allocated"))

				// Verify exactly 1 device
				totalDevices := 0
				for _, deviceResource := range taskResources.Devices {
					totalDevices += len(deviceResource.DeviceIDs)
				}
				must.Eq(t, expectedDevices, totalDevices, must.Sprintf("expected exactly %d device", expectedDevices))
			} else {
				evalID := resp.EvalID
				var err error
				var eval *api.Evaluation
				must.Wait(t, wait.ContinualSuccess(

					wait.TestFunc(func() (bool, error) {
						//testutil.WaitForResultRetries(30, func() (bool, error) {

						eval, _, err = nomadClient.Evaluations().Info(evalID, &api.QueryOptions{WaitIndex: idx})
						if err != nil {
							return false, err
						}
						if eval.Status == api.EvalStatusComplete || eval.Status == api.EvalStatusBlocked {
							return true, nil
						}
						return false, fmt.Errorf("eval status: %s", eval.Status)
					}),
					wait.Timeout(5*time.Second),
					wait.Gap(100*time.Millisecond),
				))
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

		})
	}

}

// TestDeviceParsing tests that traditional device configurations (count,
// constraint, affinity) are parsed correctly. These are unit-style tests
// that don't require a running Nomad cluster.
func TestDeviceParsing(t *testing.T) {
	if os.Getenv(envGate) != "1" {
		t.Skip(envGate + " is not set; skipping")
	}
	t.Run("testParseDeviceCountOnly", testParseDeviceCountOnly)
	t.Run("testParseDeviceWithConstraint", testParseDeviceWithConstraint)
	t.Run("testParseDeviceWithAffinity", testParseDeviceWithAffinity)
	t.Run("testParseDeviceWithConstraintAndAffinity", testParseDeviceWithConstraintAndAffinity)
	t.Run("testParseFirstAvailable", testParseFirstAvailable)
	t.Run("testParseWithBaseConstraint", testParseWithBaseConstraint)
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
	must.Eq(t, "${device.attr.cool-attribute}", device.Constraints[0].LTarget)
	must.Eq(t, "attribute-wearing-sunglasses", device.Constraints[0].RTarget)
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
	must.Eq(t, "${device.attr.cool-attribute}", device.Constraints[0].LTarget)
	must.Eq(t, "attribute-wearing-sunglasses", device.Constraints[0].RTarget)

	// Verify affinity
	must.Len(t, 1, device.Affinities)
	must.Eq(t, "${device.attr.priority}", device.Affinities[0].LTarget)
	must.Eq(t, "high", device.Affinities[0].RTarget)
	must.Eq(t, int8(50), *device.Affinities[0].Weight)

	// No first_available
	must.Len(t, 0, device.FirstAvailable)
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
