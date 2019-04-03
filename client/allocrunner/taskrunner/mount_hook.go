package taskrunner

import (
	"context"
	"fmt"

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

	mounts := h.runner.hookResources.getMounts()

	for _, s := range req.Task.VolumeMounts {
		v, ok := volumes[s.VolumeName]
		if !ok {
			return fmt.Errorf("Could not find host volume declaration named %s", s.VolumeName)
		}

		dm := &drivers.MountConfig{
			HostPath: v.Path,
			TaskPath: s.MountPath,
			Readonly: v.ReadOnly || s.ReadOnly,
		}
		mounts = append(mounts, dm)
	}

	h.runner.hookResources.setMounts(mounts)

	resp.Done = true
	return nil
}
