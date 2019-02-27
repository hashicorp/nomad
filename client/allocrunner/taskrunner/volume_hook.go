package taskrunner

import (
	"context"
	"fmt"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

type volumeHook struct {
	alloc  *structs.Allocation
	runner *TaskRunner
	logger log.Logger
}

func newVolumeHook(runner *TaskRunner, logger log.Logger) *volumeHook {
	h := &volumeHook{
		alloc:  runner.Alloc(),
		runner: runner,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*volumeHook) Name() string {
	return "volumes"
}

func (h *volumeHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	volumes := h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup).Volumes

	mounts := h.runner.hookResources.getMounts()

	for _, m := range req.Task.VolumeMounts {
		v := volumes[m.Volume]
		// TODO: Validate mounts before now.
		if v.Type == "host" {
			mcfg := &drivers.MountConfig{
				HostPath: v.Config["source"].(string),
				TaskPath: m.Destination,
				Readonly: v.ReadOnly || m.ReadOnly,
			}
			mounts = append(mounts, mcfg)
		} else {
			return fmt.Errorf("Unsupported mount type: %s", v.Type)
		}
	}

	h.runner.hookResources.setMounts(mounts)

	resp.Done = true
	return nil
}
