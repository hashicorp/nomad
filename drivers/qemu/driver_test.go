package qemu

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl2/hcl"
	ctestutil "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

// TODO(preetha) - tests remaining
// fingerprinting
// stats
// using monitor socket for graceful shutdown

// Verifies starting a qemu image and stopping it
func TestQemuDriver_Start_Wait_Stop(t *testing.T) {
	ctestutil.QemuCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	require := require.New(t)
	d := NewQemuDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "linux",
		Resources: &drivers.Resources{
			NomadResources: &structs.Resources{
				MemoryMB: 512,
				CPU:      100,
				Networks: []*structs.NetworkResource{
					{
						ReservedPorts: []structs.Port{{Label: "main", Value: 22000}, {Label: "web", Value: 80}},
					},
				},
			},
		},
	}

	taskConfig := map[string]interface{}{
		"image_path":        "linux-0.2.img",
		"accelerator":       "tcg",
		"graceful_shutdown": false,
		"port_map": map[string]int{
			"main": 22,
			"web":  8080,
		},
		"args": []string{"-nodefconfig", "-nodefaults"},
	}
	encodeDriverHelper(require, task, taskConfig)
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
	if !testutil.IsTravis() {
		t.Parallel()
	}

	require := require.New(t)
	d := NewQemuDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "linux",
		Resources: &drivers.Resources{
			NomadResources: &structs.Resources{
				MemoryMB: 512,
				CPU:      100,
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
		Attributes: map[string]string{
			qemuDriverVersionAttr: "2.0.0",
		},
	}
	shortPath := strings.Repeat("x", 10)
	qemuDriver := d.(*QemuDriver)
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
	if !testutil.IsTravis() {
		t.Parallel()
	}

	require := require.New(t)
	d := NewQemuDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "linux",
		Resources: &drivers.Resources{
			NomadResources: &structs.Resources{
				MemoryMB: 512,
				CPU:      100,
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
		Attributes: map[string]string{
			qemuDriverVersionAttr: "2.99.99",
		},
	}
	shortPath := strings.Repeat("x", 10)
	qemuDriver := d.(*QemuDriver)
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

//encodeDriverhelper sets up the task config spec and encodes qemu specific driver configuration
func encodeDriverHelper(require *require.Assertions, task *drivers.TaskConfig, taskConfig map[string]interface{}) {
	evalCtx := &hcl.EvalContext{
		Functions: shared.GetStdlibFuncs(),
	}
	spec, diag := hclspec.Convert(taskConfigSpec)
	require.False(diag.HasErrors(), diag.Error())
	taskConfigCtyVal, diag := shared.ParseHclInterface(taskConfig, spec, evalCtx)
	require.False(diag.HasErrors(), diag.Error())
	err := task.EncodeDriverConfig(taskConfigCtyVal)
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
	if !testutil.IsTravis() {
		t.Parallel()
	}

	require := require.New(t)
	d := NewQemuDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "linux",
		User: "alice",
		Resources: &drivers.Resources{
			NomadResources: &structs.Resources{
				MemoryMB: 512,
				CPU:      100,
				Networks: []*structs.NetworkResource{
					{
						ReservedPorts: []structs.Port{{Label: "main", Value: 22000}, {Label: "web", Value: 80}},
					},
				},
			},
		},
	}

	taskConfig := map[string]interface{}{
		"image_path":        "linux-0.2.img",
		"accelerator":       "tcg",
		"graceful_shutdown": false,
		"port_map": map[string]int{
			"main": 22,
			"web":  8080,
		},
		"args": []string{"-nodefconfig", "-nodefaults"},
	}
	encodeDriverHelper(require, task, taskConfig)
	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	taskDir := filepath.Join(task.AllocDir, task.Name)

	copyFile("./test-resources/linux-0.2.img", filepath.Join(taskDir, "linux-0.2.img"), t)

	_, _, err := harness.StartTask(task)
	require.Error(err)
	require.Contains(err.Error(), "unknown user alice", err.Error())

}
