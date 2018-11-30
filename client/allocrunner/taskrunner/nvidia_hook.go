package taskrunner

import (
	"context"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/nvidia"
	"github.com/hashicorp/nomad/nomad/structs"
)

// nvidiaHook is custom for NVIDIA GPU devices to configure chroot environment
// with nvidia devices and utility binaries
//
// This is needed for two reasons:
//   * NVIDIA GPU device isolation is not as simple as bind-mounting a `/dev/nvidiaX` device
//   * Commonly, ML containers lack CUDA driver libraries/binaries are expected to be injected from the host
//
type nvidiaHook struct {
	runner *TaskRunner
	logger hclog.Logger
}

func newNvidiaHook(runner *TaskRunner, logger hclog.Logger) *deviceMountHook {
	h := &deviceMountHook{
		runner: runner,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (h *nvidiaHook) Name() string {
	return "nvidia_configure"
}

func (h *nvidiaHook) Prestart(ctx context.Context,
	req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {

	// Mount only when task runner is responsible for creating the chroot isolation for task.
	// Drivers with image based isolation (e.g. docker, qemu) need to mount devices, instead.
	if h.runner.driverCapabilities.FSIsolation != cstructs.FSIsolationChroot {
		resp.Done = true
		return nil
	}

	// TODO: Validate that we only use trusted value from hooks
	devices, ok := req.TaskEnv.EnvMap["NVIDIA_VISIBLE_DEVICES"]
	if !ok {
		resp.Done = true
		return nil
	}

	// Emit the event that we are going to be building the task directory
	h.runner.EmitEvent(structs.NewTaskEvent(structs.TaskSetup).SetMessage("configure NVIDIA runtime"))

	caps := []string{"all"}
	if capStr, ok := req.TaskEnv.EnvMap["NVIDIA_DRIVER_CAPABILITIES"]; ok {
		caps = strings.Split(capStr, ",")
	}

	// TODO: requirements

	err := nvidia.ConfigureContainer(h.runner.taskDir.Dir, nvidia.Config{
		Devices:      strings.Split(devices, ","),
		Capabilities: caps,
	})

	if err == nil {
		resp.Done = true
	}

	return err
}
