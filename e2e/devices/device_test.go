// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package devices

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/execagent"
	"github.com/hashicorp/nomad/helper/discover"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

const (
	envGate    = "NOMAD_E2E_PLUGIN_PATH"
	deviceName = "nomad/file/mock"
)

// hasDevicePlugin validates the device plugin is available or skips the test
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
// These tests require the static enabled version of the example device
// plugin (nomad/file/mock) to be installed

func TestDeviceScheduling(t *testing.T) {
	if os.Getenv(envGate) == "" {
		t.Fatal(envGate + " is not set; skipping")
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
	}

	// Set up binaries & nomad agent
	nomadBinary, err := discover.NomadExecutable()
	must.NoError(t, err)
	must.FileExists(t, nomadBinary)

	agentCallbackFn := func(c *execagent.AgentTemplateVars) {
		c.AgentName = "device-test"
		c.LogLevel = hclog.Warn.String()
	}
	pluginPath := os.Getenv(envGate)
	must.FileExists(t, pluginPath+"/nomad-device-example")

	c, err := os.ReadFile("./input/basic_device_config.hcl")
	must.NoError(t, err)

	cfg := string(c)
	cfg = cfg + fmt.Sprintf("\nplugin_dir=\"%s\"", strings.TrimSuffix(pluginPath, "/nomad-device-example"))

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
		wait.BoolFunc(func() bool {
			nodes, _, err := nomadClient.Nodes().List(nil)
			if err != nil {
				return false
			}

			for _, nodeStub := range nodes {
				node, _, err := nomadClient.Nodes().Info(nodeStub.ID, nil)
				if err != nil {
					return false
				}
				if node.NodeResources != nil && node.NodeResources.Devices != nil {
					for _, device := range node.NodeResources.Devices {
						fullName := device.Vendor + "/" + device.Type + "/" + device.Name
						if strings.Contains(fullName, deviceName) ||
							strings.Contains(device.Name, deviceName) {
							return true
						} else {
							return false
						}
					}
				}
			}
			return false
		}),
		wait.Attempts(2),
		wait.Timeout(15*time.Second),
		wait.Gap(100*time.Millisecond),
	))

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Confirm nodes have mock devices available
			must.True(t, hasDevicePlugin(t, nomadClient, "nomad/file/mock"))
			expectedAllocs := 0
			expectedDevices := 0
			if tc.expectedAllocs != 0 {
				expectedAllocs = tc.expectedAllocs
			}
			if tc.expectedDevices != 0 {
				expectedDevices = tc.expectedDevices
			}

			// Build and register job
			jobID := "dc-" + tc.name + "_" + uuid.Short()
			spec, err := os.ReadFile(tc.jobFile)
			must.NoError(t, err)

			job, err := nomadClient.Jobs().ParseHCLOpts(&api.JobsParseRequest{
				JobHCL:       string(spec),
				Canonicalize: true,
			})
			must.NoError(t, err)

			job.ID = new(jobID)
			var idx uint64
			jobs := nomadClient.Jobs()

			resp, meta, err := jobs.Register(job, nil)
			must.NoError(t, err)

			idx = meta.LastIndex
			must.NotEq(t, resp.EvalID, "")

			t.Cleanup(func() { nomadClient.Jobs().Deregister(jobID, true, nil) })

			if expectedAllocs != 0 {
				var alloc *api.Allocation
				must.Wait(t, wait.InitialSuccess(
					wait.ErrorFunc(func() error {
						allocs, _, err := jobs.Allocations(jobID, false, &api.QueryOptions{WaitIndex: idx})
						must.NoError(t, err)
						must.Len(t, expectedAllocs, allocs, must.Sprintf("expected %d allocations", expectedAllocs))

						alloc, _, err = nomadClient.Allocations().Info(allocs[expectedAllocs-1].ID, nil)
						must.NoError(t, err)

						// Verify allocation didn't fail and device was allocated
						must.NotEq(t, api.AllocClientStatusFailed, alloc.ClientStatus)
						must.NotNil(t, alloc.AllocatedResources)

						return nil
					}),
					wait.Timeout(5*time.Second),
					wait.Gap(100*time.Millisecond),
				))

				// Verify expected devices
				taskResources := alloc.AllocatedResources.Tasks["sleep"]
				must.NotNil(t, taskResources)
				must.SliceNotEmpty(t, taskResources.Devices,
					must.Sprint("expected devices to be allocated"))

				totalDevices := 0
				for _, deviceResource := range taskResources.Devices {
					totalDevices += len(deviceResource.DeviceIDs)
				}
				must.Eq(t, expectedDevices, totalDevices, must.Sprintf("expected exactly %d device", expectedDevices))
			} else {
				evalID := resp.EvalID

				var err error
				var eval *api.Evaluation

				must.Wait(t, wait.InitialSuccess(
					wait.TestFunc(func() (bool, error) {
						eval, _, err = nomadClient.Evaluations().Info(evalID, &api.QueryOptions{WaitIndex: idx})
						if err != nil {
							return false, err
						}
						if eval.Status == api.EvalStatusComplete || eval.Status == api.EvalStatusBlocked {
							return true, nil
						}
						return false, fmt.Errorf("eval status: %s", eval.Status)
					}),
					wait.Attempts(3),
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

					must.True(t, exhausted, must.Sprintf("expected device exhaustion, got metrics: %+v", metrics))
				}

			}

		})
	}
}
