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
		volumeRequest, ok := volumes[m.Volume]
		if !ok {
			return fmt.Errorf("Could not find volume declaration named: %s", m.Volume)
		}

		if volumeRequest.Type != "host" {
			// We currently only handle host volumes in this hook, and other types
			// should not get scheduled yet.
			continue
		}

		hostVolume, ok := h.runner.clientConfig.Node.HostVolumes[volumeRequest.Name]
		if !ok {
			h.logger.Error("Failed to find host volume", "existing", h.runner.clientConfig.Node.HostVolumes, "requested", volumeRequest)
			return fmt.Errorf("Could not find host volume named: %s", m.Volume)
		}

		mcfg := &drivers.MountConfig{
			HostPath: hostVolume.Source,
			TaskPath: m.Destination,
			Readonly: hostVolume.ReadOnly || volumeRequest.ReadOnly || m.ReadOnly,
		}
		mounts = append(mounts, mcfg)
	}

	h.runner.hookResources.setMounts(mounts)

	resp.Done = true
	return nil
}
