package qemu

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ctestutil "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/pluginutils/hclutils"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	dtestutil "github.com/hashicorp/nomad/plugins/drivers/testutils"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

// TODO(preetha) - tests remaining
// using monitor socket for graceful shutdown

// Verifies starting a qemu image and stopping it
func TestQemuDriver_Start_Wait_Stop(t *testing.T) {
	ctestutil.QemuCompatible(t)
	if !testutil.IsCI() {
		t.Parallel()
	}

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

// Verifies monitor socket path for old qemu
func TestQemuDriver_GetMonitorPathOldQemu(t *testing.T) {
	ctestutil.QemuCompatible(t)
	if !testutil.IsCI() {
		t.Parallel()
	}

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

	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	fingerPrint := &drivers.Fingerprint{
		Attributes: map[string]*pstructs.Attribute{
			driverVersionAttr: pstructs.NewStringAttribute("2.0.0"),
		},
	}
	shortPath := strings.Repeat("x", 10)
	qemuDriver := d.(*Driver)
	_, err := qemuDriver.getMonitorPath(shortPath, fingerPrint)
	require.Nil(err)

	longPath := strings.Repeat("x", qemuLegacyMaxMonitorPathLen+100)
	_, err = qemuDriver.getMonitorPath(longPath, fingerPrint)
	require.NotNil(err)

	// Max length includes the '/' separator and socket name
	maxLengthCount := qemuLegacyMaxMonitorPathLen - len(qemuMonitorSocketName) - 1
	maxLengthLegacyPath := strings.Repeat("x", maxLengthCount)
	_, err = qemuDriver.getMonitorPath(maxLengthLegacyPath, fingerPrint)
	require.Nil(err)
}

// Verifies monitor socket path for new qemu version
func TestQemuDriver_GetMonitorPathNewQemu(t *testing.T) {
	ctestutil.QemuCompatible(t)
	if !testutil.IsCI() {
		t.Parallel()
	}

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

	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	fingerPrint := &drivers.Fingerprint{
		Attributes: map[string]*pstructs.Attribute{
			driverVersionAttr: pstructs.NewStringAttribute("2.99.99"),
		},
	}
	shortPath := strings.Repeat("x", 10)
	qemuDriver := d.(*Driver)
	_, err := qemuDriver.getMonitorPath(shortPath, fingerPrint)
	require.Nil(err)

	// Should not return an error in this qemu version
	longPath := strings.Repeat("x", qemuLegacyMaxMonitorPathLen+100)
	_, err = qemuDriver.getMonitorPath(longPath, fingerPrint)
	require.Nil(err)

	// Max length includes the '/' separator and socket name
	maxLengthCount := qemuLegacyMaxMonitorPathLen - len(qemuMonitorSocketName) - 1
	maxLengthLegacyPath := strings.Repeat("x", maxLengthCount)
	_, err = qemuDriver.getMonitorPath(maxLengthLegacyPath, fingerPrint)
	require.Nil(err)
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
	ctestutil.QemuCompatible(t)
	if !testutil.IsCI() {
		t.Parallel()
	}

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

//  Verifies getting resource usage stats
// TODO(preetha) this test needs random sleeps to pass
func TestQemuDriver_Stats(t *testing.T) {
	ctestutil.QemuCompatible(t)
	if !testutil.IsCI() {
		t.Parallel()
	}

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
	require := require.New(t)

	ctestutil.QemuCompatible(t)
	if !testutil.IsCI() {
		t.Parallel()
	}

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
	cfgStr := `
config {
  image_path = "/tmp/image_path"
  accelerator = "kvm"
  args = ["arg1", "arg2"]
  port_map {
    http = 80
    https = 443
  }
  graceful_shutdown = true
}`

	expected := &TaskConfig{
		ImagePath:   "/tmp/image_path",
		Accelerator: "kvm",
		Args:        []string{"arg1", "arg2"},
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

func TestIsAllowedImagePath(t *testing.T) {
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
