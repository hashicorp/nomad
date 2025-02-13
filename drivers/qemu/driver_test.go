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
	"github.com/hashicorp/nomad/client/lib/cgroupslib"
	"github.com/hashicorp/nomad/client/lib/numalib"
	ctestutil "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/pluginutils/hclutils"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	dtestutil "github.com/hashicorp/nomad/plugins/drivers/testutils"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

// TODO(preetha) - tests remaining
// using monitor socket for graceful shutdown

// Verifies starting a qemu image and stopping it
func TestQemuDriver_Start_Wait_Stop(t *testing.T) {
	ci.Parallel(t)
	ctestutil.QemuCompatible(t)
	ctestutil.CgroupsCompatible(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	topology := numalib.Scan(numalib.PlatformScanners(false))
	d := NewQemuDriver(ctx, testlog.HCLogger(t))
	d.(*Driver).nomadConfig = &base.ClientDriverConfig{Topology: topology}
	harness := dtestutil.NewDriverHarness(t, d)
	allocID := uuid.Generate()
	harness.MakeTaskCgroup(allocID, "linux")

	task := &drivers.TaskConfig{
		AllocID:   allocID,
		ID:        uuid.Generate(),
		Name:      "linux",
		Resources: testResources(allocID, "linux"),
	}

	tc := &TaskConfig{
		ImagePath:        "linux-0.2.img",
		Accelerator:      "tcg",
		GracefulShutdown: false,
		PortMap: map[string]int{
			"main": 22,
			"web":  8080,
		},
		Args: []string{"-nodefaults"},
	}
	must.NoError(t, task.EncodeConcreteDriverConfig(&tc))
	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	taskDir := filepath.Join(task.AllocDir, task.Name)

	copyFile("./test-resources/linux-0.2.img", filepath.Join(taskDir, "linux-0.2.img"), t)

	handle, _, err := harness.StartTask(task)
	must.NoError(t, err)

	must.NotNil(t, handle)

	// Ensure that sending a Signal returns an error
	err = d.SignalTask(task.ID, "SIGINT")
	must.EqError(t, err, "QEMU driver can't signal commands")

	must.NoError(t, harness.DestroyTask(task.ID, true))

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
	ctestutil.CgroupsCompatible(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	topology := numalib.Scan(numalib.PlatformScanners(false))
	d := NewQemuDriver(ctx, testlog.HCLogger(t))
	d.(*Driver).nomadConfig = &base.ClientDriverConfig{Topology: topology}
	harness := dtestutil.NewDriverHarness(t, d)
	allocID := uuid.Generate()
	harness.MakeTaskCgroup(allocID, "linux")

	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "linux",
		User:      "alice",
		Resources: testResources(allocID, "linux"),
	}

	tc := &TaskConfig{
		ImagePath:        "linux-0.2.img",
		Accelerator:      "tcg",
		GracefulShutdown: false,
		PortMap: map[string]int{
			"main": 22,
			"web":  8080,
		},
		Args: []string{"-nodefaults"},
	}
	must.NoError(t, task.EncodeConcreteDriverConfig(&tc))
	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	taskDir := filepath.Join(task.AllocDir, task.Name)

	copyFile("./test-resources/linux-0.2.img", filepath.Join(taskDir, "linux-0.2.img"), t)

	_, _, err := harness.StartTask(task)
	must.ErrorContains(t, err, "unknown user alice")
}

// TestQemuDriver_Stats	verifies we can get resources usage stats
func TestQemuDriver_Stats(t *testing.T) {
	ci.Parallel(t)
	ctestutil.QemuCompatible(t)
	ctestutil.CgroupsCompatible(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	topology := numalib.Scan(numalib.PlatformScanners(false))
	d := NewQemuDriver(ctx, testlog.HCLogger(t))
	d.(*Driver).nomadConfig = &base.ClientDriverConfig{Topology: topology}
	harness := dtestutil.NewDriverHarness(t, d)
	allocID := uuid.Generate()
	harness.MakeTaskCgroup(allocID, "linux")

	task := &drivers.TaskConfig{
		AllocID:   allocID,
		ID:        uuid.Generate(),
		Name:      "linux",
		Resources: testResources(allocID, "linux"),
	}

	tc := &TaskConfig{
		ImagePath:        "linux-0.2.img",
		Accelerator:      "tcg",
		GracefulShutdown: false,
		PortMap: map[string]int{
			"main": 22,
			"web":  8080,
		},
		Args: []string{"-nodefaults"},
	}
	must.NoError(t, task.EncodeConcreteDriverConfig(&tc))
	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	taskDir := filepath.Join(task.AllocDir, task.Name)

	copyFile("./test-resources/linux-0.2.img", filepath.Join(taskDir, "linux-0.2.img"), t)

	handle, _, err := harness.StartTask(task)
	must.NoError(t, err)
	must.NotNil(t, handle)

	// Wait for task to start
	exitCh, err := harness.WaitTask(context.Background(), handle.Config.ID)
	must.NoError(t, err)

	// Wait until task started
	must.NoError(t, harness.WaitUntilStarted(task.ID, 1*time.Second))

	t.Cleanup(func() { harness.DestroyTask(task.ID, true) })

	timeout := 30 * time.Second
	statsCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	statsCh, err := harness.TaskStats(statsCtx, task.ID, time.Second*1)
	must.NoError(t, err)

	for {
		select {
		case exitCode := <-exitCh:
			t.Fatalf("should not have exited: %+v", exitCode)
		case stats := <-statsCh:
			if stats == nil {
				continue // receiving empty stats races with ctx.Done
			}
			t.Logf("CPU:%+v Memory:%+v\n",
				stats.ResourceUsage.CpuStats, stats.ResourceUsage.MemoryStats)
			if stats.ResourceUsage.MemoryStats.RSS != 0 {
				must.NonZero(t, stats.ResourceUsage.MemoryStats.RSS)
				must.NoError(t, harness.DestroyTask(task.ID, true))
				return
			}
		case <-ctx.Done():
			t.Fatal("timed out before receiving non-zero stats")
		}
	}
}

func TestQemuDriver_Fingerprint(t *testing.T) {
	ci.Parallel(t)

	ctestutil.QemuCompatible(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewQemuDriver(ctx, testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	fingerCh, err := harness.Fingerprint(context.Background())
	must.NoError(t, err)
	select {
	case finger := <-fingerCh:
		must.Eq(t, drivers.HealthStateHealthy, finger.Health)
		ok, _ := finger.Attributes["driver.qemu"].GetBool()
		must.True(t, ok)
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		t.Fatal("timeout receiving fingerprint")
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
	must.Eq(t, expected, tc)
}

func TestIsAllowedDriveInterface(t *testing.T) {
	validInterfaces := []string{"ide", "scsi", "sd", "mtd", "floppy", "pflash", "virtio", "none"}
	invalidInterfaces := []string{"foo", "virtio-foo"}

	for _, i := range validInterfaces {
		must.True(t, isAllowedDriveInterface(i),
			must.Sprintf("drive_interface should be allowed: %v", i))
	}

	for _, i := range invalidInterfaces {
		must.False(t, isAllowedDriveInterface(i),
			must.Sprintf("drive_interface should not be allowed: %v", i))
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
		must.True(t, isAllowedImagePath(allowedPaths, allocDir, p),
			must.Sprintf("path should be allowed: %v", p))
	}

	for _, p := range invalidPaths {
		must.False(t, isAllowedImagePath(allowedPaths, allocDir, p),
			must.Sprintf("path should be not allowed: %v", p))
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
		must.NoError(t, validateArgs(pluginConfigAllowList, args))
		must.NoError(t, validateArgs([]string{}, args))

	}
	for _, args := range invalidArgs {
		must.Error(t, validateArgs(pluginConfigAllowList, args))
		must.NoError(t, validateArgs([]string{}, args))
	}

}

func testResources(allocID, task string) *drivers.Resources {
	if allocID == "" || task == "" {
		panic("must be set")
	}

	r := &drivers.Resources{
		NomadResources: &structs.AllocatedTaskResources{
			Memory: structs.AllocatedMemoryResources{
				MemoryMB: 128,
			},
			Cpu: structs.AllocatedCpuResources{
				CpuShares: 100,
			},
			Networks: []*structs.NetworkResource{
				{
					ReservedPorts: []structs.Port{
						{Label: "main", Value: 22000}, {Label: "web", Value: 8888}},
				},
			},
		},
		LinuxResources: &drivers.LinuxResources{
			MemoryLimitBytes: 134217728,
			CPUShares:        100,
			CpusetCgroupPath: cgroupslib.LinuxResourcesPath(allocID, task, false),
		},
	}

	return r
}
