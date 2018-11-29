package taskrunner

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	cstructs "github.com/hashicorp/nomad/client/structs"
)

type deviceMountHook struct {
	runner *TaskRunner
	logger hclog.Logger

	// Stored state to be used by Stop
	chroot *allocdir.TaskDir
	mounts []string
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

	h.chroot = h.runner.taskDir

	for _, d := range h.runner.hookResources.getDevices() {
		readOnly := d.Permissions != "rw"
		if err := h.chroot.Mount(d.HostPath, d.TaskPath, readOnly); err != nil {
			return fmt.Errorf("failed to mount device %s: %v", d.TaskPath, err)
		}

		h.mounts = append(h.mounts, d.TaskPath)
	}

	for _, d := range h.runner.hookResources.getMounts() {
		if err := h.chroot.Mount(d.HostPath, d.TaskPath, d.Readonly); err != nil {
			return fmt.Errorf("failed to mount mount %s: %v", d.TaskPath, err)
		}

		h.mounts = append(h.mounts, d.TaskPath)
	}

	return nil
}

func (h *deviceMountHook) Stop(context.Context, *interfaces.TaskStopRequest, *interfaces.TaskStopResponse) error {
	var merr multierror.Error

	for _, m := range h.mounts {
		if err := h.chroot.Unmount(m); err != nil {
			merr.Errors = append(merr.Errors, fmt.Errorf("failed to unmount %v: %v", m, err))
		}
	}

	return merr.ErrorOrNil()
}
