package taskrunner

import (
	"context"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

type mountHook struct {
	alloc  *structs.Allocation
	runner *TaskRunner
	logger log.Logger
}

func newMountHook(runner *TaskRunner, logger log.Logger) *mountHook {
	h := &mountHook{
		alloc:  runner.Alloc(),
		runner: runner,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*mountHook) Name() string {
	return "mounts"
}

func (h *mountHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	volumes := h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup).HostVolumes
	volumesByName := make(map[string]*structs.HostVolume, len(volumes))
	for _, v := range volumes {
		volumesByName[v.Name] = v
	}

	h.logger.Info("Mount Configuration:", "volumes", volumesByName, "requested", req.Task.VolumeMounts[0])

	mounts := h.runner.hookResources.getMounts()

	for _, mount := range req.Task.VolumeMounts {
		v := volumesByName[mount.VolumeName]
		dmount := &drivers.MountConfig{
			HostPath: v.Path,
			TaskPath: mount.MountPath,
			Readonly: v.ReadOnly || mount.ReadOnly,
		}
		mounts = append(mounts, dmount)
	}

	h.runner.hookResources.setMounts(mounts)

	resp.Done = true
	return nil
}
