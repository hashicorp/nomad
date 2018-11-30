package taskrunner

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	cstructs "github.com/hashicorp/nomad/client/structs"
)

type deviceMountHook struct {
	runner *TaskRunner
	logger hclog.Logger
}

func newDeviceMountHook(runner *TaskRunner, logger hclog.Logger) *deviceMountHook {
	h := &deviceMountHook{
		runner: runner,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (h *deviceMountHook) Name() string {
	return "device_mounting"
}

func (h *deviceMountHook) Prestart(ctx context.Context,
	req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {

	// Mount only when task runner is responsible for creating the chroot isolation for task.
	// Drivers with image based isolation (e.g. docker, qemu) need to mount devices, instead.
	if h.runner.driverCapabilities.FSIsolation != cstructs.FSIsolationChroot {
		resp.Done = true
		return nil
	}

	chroot := h.runner.taskDir

	for _, d := range h.runner.hookResources.getDevices() {
		readOnly := d.Permissions != "rw"
		if err := chroot.Mount(d.HostPath, d.TaskPath, readOnly); err != nil {
			return fmt.Errorf("failed to mount device %s: %v", d.TaskPath, err)
		}
	}

	for _, d := range h.runner.hookResources.getMounts() {
		if err := chroot.Mount(d.HostPath, d.TaskPath, d.Readonly); err != nil {
			return fmt.Errorf("failed to mount mount %s: %v", d.TaskPath, err)
		}
	}

	return nil
}
