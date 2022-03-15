package qemu

import (
	"context"
	"path/filepath"
	"testing"

	ctestutil "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	dtestutil "github.com/hashicorp/nomad/plugins/drivers/testutils"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

// Verifies starting a qemu image and stopping it
func TestQemuPorts_User(t *testing.T) {
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
