package taskrunner

import (
	"context"
	"fmt"

	log "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
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

func validateHostVolumes(requested map[string]*structs.VolumeRequest, client map[string]*structs.ClientHostVolumeConfig) error {
	var result error

	for n, req := range requested {
		if req.Volume.Type != "host" {
			continue
		}

		cfg, err := structs.ParseHostVolumeConfig(req.Config)
		if err != nil {
			result = multierror.Append(result, fmt.Errorf("failed to parse config for %s: %v", n, err))
		}

		_, ok := client[cfg.Source]
		if !ok {
			result = multierror.Append(result, fmt.Errorf("missing %s", cfg.Source))
		}
	}

	return result
}

func (h *volumeHook) hostVolumeMountConfigurations(vmounts []*structs.VolumeMount, volumes map[string]*structs.VolumeRequest, client map[string]*structs.ClientHostVolumeConfig) ([]*drivers.MountConfig, error) {
	var mounts []*drivers.MountConfig
	for _, m := range vmounts {
		req, ok := volumes[m.Volume]
		if !ok {
			// Should never happen unless we misvalidated on job submission
			return nil, fmt.Errorf("No group volume declaration found named: %s", m.Volume)
		}

		cfg, err := structs.ParseHostVolumeConfig(req.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse config for %s: %v", m.Volume, err)
		}

		hostVolume, ok := client[cfg.Source]
		if !ok {
			// Should never happen, but unless the client volumes were mutated during
			// the execution of this hook.
			return nil, fmt.Errorf("No host volume named: %s", cfg.Source)
		}

		mcfg := &drivers.MountConfig{
			HostPath: hostVolume.Source,
			TaskPath: m.Destination,
			Readonly: hostVolume.ReadOnly || req.Volume.ReadOnly || m.ReadOnly,
		}
		mounts = append(mounts, mcfg)
	}

	return mounts, nil
}

func (h *volumeHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	volumes := h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup).Volumes
	mounts := h.runner.hookResources.getMounts()

	hostVolumes := h.runner.clientConfig.Node.HostVolumes

	// Always validate volumes to ensure that we do not allow volumes to be used
	// if a host is restarted and loses the host volume configuration.
	if err := validateHostVolumes(volumes, hostVolumes); err != nil {
		h.logger.Error("Requested Host Volume does not exist", "existing", hostVolumes, "requested", volumes)
		return fmt.Errorf("host volume validation error: %v", err)
	}

	requestedMounts, err := h.hostVolumeMountConfigurations(req.Task.VolumeMounts, volumes, hostVolumes)
	if err != nil {
		h.logger.Error("Failed to generate volume mounts", "error", err)
		return err
	}

	// Because this hook is also ran on restores, we only add mounts that do not
	// already exist. Although this loop is somewhat expensive, there are only
	// a small number of mounts that exist within most individual tasks. We may
	// want to revisit this using a `hookdata` param to be "mount only once"
REQUESTED:
	for _, m := range requestedMounts {
		for _, em := range mounts {
			if em.IsEqual(m) {
				continue REQUESTED
			}
		}

		mounts = append(mounts, m)
	}

	h.runner.hookResources.setMounts(mounts)
	return nil
}
