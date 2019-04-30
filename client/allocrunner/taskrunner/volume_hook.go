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
	volumes := h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup).HostVolumes

	mounts := h.runner.hookResources.getMounts()

	for _, m := range req.Task.VolumeMounts {
		volumeRequest, ok := volumes[m.Volume]
		if !ok {
			return fmt.Errorf("Could not find volume declaration named: %s", m.Volume)
		}

		// Look up the local Host Volume based on the Source parameter
		hostVolume, ok := h.runner.clientConfig.Node.HostVolumes[volumeRequest.Config.Source]
		if !ok {
			h.logger.Error("Failed to find host volume", "existing", h.runner.clientConfig.Node.HostVolumes, "requested", volumeRequest)
			return fmt.Errorf("Could not find host volume named: %s", m.Volume)
		}

		mcfg := &drivers.MountConfig{
			HostPath: hostVolume.Source,
			TaskPath: m.Destination,
			Readonly: hostVolume.ReadOnly || volumeRequest.Volume.ReadOnly || m.ReadOnly,
		}
		mounts = append(mounts, mcfg)
	}

	h.runner.hookResources.setMounts(mounts)

	resp.Done = true
	return nil
}
