// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package qemu

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	ctestutil "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/pluginutils/hclutils"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	dtestutil "github.com/hashicorp/nomad/plugins/drivers/testutils"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

// TODO(preetha) - tests remaining
// using monitor socket for graceful shutdown

// Verifies starting a qemu image and stopping it
func TestQemuDriver_Start_Wait_Stop(t *testing.T) {
	ci.Parallel(t)
	ctestutil.QemuCompatible(t)

	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewQemuDriver(ctx, testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "linux",
		Resources: &drivers.Resources{
			NomadResources: &structs.AllocatedTaskResources{
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 512,
				},
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 100,
				},
				Networks: []*structs.NetworkResource{
					{
						ReservedPorts: []structs.Port{{Label: "main", Value: 22000}, {Label: "web", Value: 80}},
					},
				},
			},
		},
	}

	tc := &TaskConfig{
		ImagePath:        "linux-0.2.img",
		Accelerator:      "tcg",
		GracefulShutdown: false,
		PortMap: map[string]int{
			"main": 22,
			"web":  8080,
		},
		Args: []string{"-nodefconfig", "-nodefaults"},
	}
	require.NoError(task.EncodeConcreteDriverConfig(&tc))
	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	taskDir := filepath.Join(task.AllocDir, task.Name)

	copyFile("./test-resources/linux-0.2.img", filepath.Join(taskDir, "linux-0.2.img"), t)

	handle, _, err := harness.StartTask(task)
	require.NoError(err)

	require.NotNil(handle)

	// Ensure that sending a Signal returns an error
	err = d.SignalTask(task.ID, "SIGINT")
	require.NotNil(err)

	require.NoError(harness.DestroyTask(task.ID, true))

}

// copyFile moves an existing file to the destination
func copyFile(src, dst string, t *testing.T) {
	in, err := os.Open(src)
	if err != nil {
		t.Fatalf("copying %v -> %v failed: %v", src, dst, err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatalf("copying %v -> %v failed: %v", src, dst, err)
	}
	defer func() {
		if err := out.Close(); err != nil {
			t.Fatalf("copying %v -> %v failed: %v", src, dst, err)
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		t.Fatalf("copying %v -> %v failed: %v", src, dst, err)
	}
	if err := out.Sync(); err != nil {
		t.Fatalf("copying %v -> %v failed: %v", src, dst, err)
	}
}

// Verifies starting a qemu image and stopping it
func TestQemuDriver_User(t *testing.T) {
	ci.Parallel(t)
	ctestutil.QemuCompatible(t)

	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewQemuDriver(ctx, testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "linux",
		User: "alice",
		Resources: &drivers.Resources{
			NomadResources: &structs.AllocatedTaskResources{
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 512,
				},
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 100,
				},
				Networks: []*structs.NetworkResource{
					{
						ReservedPorts: []structs.Port{{Label: "main", Value: 22000}, {Label: "web", Value: 80}},
					},
				},
			},
		},
	}

	tc := &TaskConfig{
		ImagePath:        "linux-0.2.img",
		Accelerator:      "tcg",
		GracefulShutdown: false,
		PortMap: map[string]int{
			"main": 22,
			"web":  8080,
		},
		Args: []string{"-nodefconfig", "-nodefaults"},
	}
	require.NoError(task.EncodeConcreteDriverConfig(&tc))
	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	taskDir := filepath.Join(task.AllocDir, task.Name)

	copyFile("./test-resources/linux-0.2.img", filepath.Join(taskDir, "linux-0.2.img"), t)

	_, _, err := harness.StartTask(task)
	require.Error(err)
	require.Contains(err.Error(), "unknown user alice", err.Error())

}

//	Verifies getting resource usage stats
//
// TODO(preetha) this test needs random sleeps to pass
func TestQemuDriver_Stats(t *testing.T) {
	ci.Parallel(t)
	ctestutil.QemuCompatible(t)

	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewQemuDriver(ctx, testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "linux",
		Resources: &drivers.Resources{
			NomadResources: &structs.AllocatedTaskResources{
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 512,
				},
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 100,
				},
				Networks: []*structs.NetworkResource{
					{
						ReservedPorts: []structs.Port{{Label: "main", Value: 22000}, {Label: "web", Value: 80}},
					},
				},
			},
		},
	}

	tc := &TaskConfig{
		ImagePath:        "linux-0.2.img",
		Accelerator:      "tcg",
		GracefulShutdown: false,
		PortMap: map[string]int{
			"main": 22,
			"web":  8080,
		},
		Args: []string{"-nodefconfig", "-nodefaults"},
	}
	require.NoError(task.EncodeConcreteDriverConfig(&tc))
	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	taskDir := filepath.Join(task.AllocDir, task.Name)

	copyFile("./test-resources/linux-0.2.img", filepath.Join(taskDir, "linux-0.2.img"), t)

	handle, _, err := harness.StartTask(task)
	require.NoError(err)

	require.NotNil(handle)

	// Wait for task to start
	_, err = harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(err)

	// Wait until task started
	require.NoError(harness.WaitUntilStarted(task.ID, 1*time.Second))
	time.Sleep(30 * time.Second)
	statsCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	statsCh, err := harness.TaskStats(statsCtx, task.ID, time.Second*10)
	require.NoError(err)

	select {
	case stats := <-statsCh:
		t.Logf("CPU:%+v Memory:%+v\n", stats.ResourceUsage.CpuStats, stats.ResourceUsage.MemoryStats)
		require.NotZero(stats.ResourceUsage.MemoryStats.RSS)
		require.NoError(harness.DestroyTask(task.ID, true))
	case <-time.After(time.Second * 1):
		require.Fail("timeout receiving from stats")
	}

}

func TestQemuDriver_Fingerprint(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	ctestutil.QemuCompatible(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewQemuDriver(ctx, testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	fingerCh, err := harness.Fingerprint(context.Background())
	require.NoError(err)
	select {
	case finger := <-fingerCh:
		require.Equal(drivers.HealthStateHealthy, finger.Health)
		require.True(finger.Attributes["driver.qemu"].GetBool())
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		require.Fail("timeout receiving fingerprint")
	}
}

func TestConfig_ParseAllHCL(t *testing.T) {
	ci.Parallel(t)

	cfgStr := `
config {
  image_path = "/tmp/image_path"
  drive_interface = "virtio"
  accelerator = "kvm"
  args = ["arg1", "arg2"]
  port_map {
    http = 80
    https = 443
  }
  graceful_shutdown = true
}`

	expected := &TaskConfig{
		ImagePath:      "/tmp/image_path",
		DriveInterface: "virtio",
		Accelerator:    "kvm",
		Args:           []string{"arg1", "arg2"},
		PortMap: map[string]int{
			"http":  80,
			"https": 443,
		},
		GracefulShutdown: true,
	}

	var tc *TaskConfig
	hclutils.NewConfigParser(taskConfigSpec).ParseHCL(t, cfgStr, &tc)

	require.EqualValues(t, expected, tc)
}

func TestIsAllowedDriveInterface(t *testing.T) {
	validInterfaces := []string{"ide", "scsi", "sd", "mtd", "floppy", "pflash", "virtio", "none"}
	invalidInterfaces := []string{"foo", "virtio-foo"}

	for _, i := range validInterfaces {
		require.Truef(t, isAllowedDriveInterface(i), "drive_interface should be allowed: %v", i)
	}

	for _, i := range invalidInterfaces {
		require.Falsef(t, isAllowedDriveInterface(i), "drive_interface should be not allowed: %v", i)
	}
}

func TestIsAllowedImagePath(t *testing.T) {
	ci.Parallel(t)

	allowedPaths := []string{"/tmp", "/opt/qemu"}
	allocDir := "/opt/nomad/some-alloc-dir"

	validPaths := []string{
		"local/path",
		"/tmp/subdir/qemu-image",
		"/opt/qemu/image",
		"/opt/qemu/subdir/image",
		"/opt/nomad/some-alloc-dir/local/image.img",
	}

	invalidPaths := []string{
		"/image.img",
		"../image.img",
		"/tmpimage.img",
		"/opt/other/image.img",
		"/opt/nomad-submatch.img",
	}

	for _, p := range validPaths {
		require.Truef(t, isAllowedImagePath(allowedPaths, allocDir, p), "path should be allowed: %v", p)
	}

	for _, p := range invalidPaths {
		require.Falsef(t, isAllowedImagePath(allowedPaths, allocDir, p), "path should be not allowed: %v", p)
	}
}

func TestArgsAllowList(t *testing.T) {
	ci.Parallel(t)

	pluginConfigAllowList := []string{"-drive", "-net", "-snapshot"}

	validArgs := [][]string{
		{"-drive", "/path/to/wherever", "-snapshot"},
		{"-net", "tap,vlan=0,ifname=tap0"},
	}

	invalidArgs := [][]string{
		{"-usbdevice", "mouse"},
		{"-singlestep"},
		{"--singlestep"},
		{" -singlestep"},
		{"\t-singlestep"},
	}

	for _, args := range validArgs {
		require.NoError(t, validateArgs(pluginConfigAllowList, args))
		require.NoError(t, validateArgs([]string{}, args))

	}
	for _, args := range invalidArgs {
		require.Error(t, validateArgs(pluginConfigAllowList, args))
		require.NoError(t, validateArgs([]string{}, args))
	}

}
