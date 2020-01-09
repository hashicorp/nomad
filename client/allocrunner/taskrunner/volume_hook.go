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

func validateHostVolumes(requestedByAlias map[string]*structs.VolumeRequest, clientVolumesByName map[string]*structs.ClientHostVolumeConfig) error {
	var result error

	for _, req := range requestedByAlias {
		// This is a defensive check, but this function should only ever recieve
		// host-type volumes.
		if req.Type != structs.VolumeTypeHost {
			continue
		}

		_, ok := clientVolumesByName[req.Source]
		if !ok {
			result = multierror.Append(result, fmt.Errorf("missing %s", req.Source))
		}
	}

	return result
}

// hostVolumeMountConfigurations takes the users requested volume mounts,
// volumes, and the client host volume configuration and converts them into a
// format that can be used by drivers.
func (h *volumeHook) hostVolumeMountConfigurations(taskMounts []*structs.VolumeMount, taskVolumesByAlias map[string]*structs.VolumeRequest, clientVolumesByName map[string]*structs.ClientHostVolumeConfig) ([]*drivers.MountConfig, error) {
	var mounts []*drivers.MountConfig
	for _, m := range taskMounts {
		req, ok := taskVolumesByAlias[m.Volume]
		if !ok {
			// Should never happen unless we misvalidated on job submission
			return nil, fmt.Errorf("No group volume declaration found named: %s", m.Volume)
		}

		// This is a defensive check, but this function should only ever recieve
		// host-type volumes.
		if req.Type != structs.VolumeTypeHost {
			continue
		}

		hostVolume, ok := clientVolumesByName[req.Source]
		if !ok {
			// Should never happen, but unless the client volumes were mutated during
			// the execution of this hook.
			return nil, fmt.Errorf("No host volume named: %s", req.Source)
		}

		mcfg := &drivers.MountConfig{
			HostPath: hostVolume.Path,
			TaskPath: m.Destination,
			Readonly: hostVolume.ReadOnly || req.ReadOnly || m.ReadOnly,
		}
		mounts = append(mounts, mcfg)
	}

	return mounts, nil
}

// partitionVolumesByType takes a map of volume-alias to volume-request and
// returns them in the form of volume-type:(volume-alias:volume-request)
func partitionVolumesByType(xs map[string]*structs.VolumeRequest) map[string]map[string]*structs.VolumeRequest {
	result := make(map[string]map[string]*structs.VolumeRequest)
	for name, req := range xs {
		txs, ok := result[req.Type]
		if !ok {
			txs = make(map[string]*structs.VolumeRequest)
			result[req.Type] = txs
		}
		txs[name] = req
	}

	return result
}

func (h *volumeHook) prepareHostVolumes(volumes map[string]*structs.VolumeRequest, req *interfaces.TaskPrestartRequest) ([]*drivers.MountConfig, error) {
	hostVolumes := h.runner.clientConfig.Node.HostVolumes

	// Always validate volumes to ensure that we do not allow volumes to be used
	// if a host is restarted and loses the host volume configuration.
	if err := validateHostVolumes(volumes, hostVolumes); err != nil {
		h.logger.Error("Requested Host Volume does not exist", "existing", hostVolumes, "requested", volumes)
		return nil, fmt.Errorf("host volume validation error: %v", err)
	}

	hostVolumeMounts, err := h.hostVolumeMountConfigurations(req.Task.VolumeMounts, volumes, hostVolumes)
	if err != nil {
		h.logger.Error("Failed to generate host volume mounts", "error", err)
		return nil, err
	}

	return hostVolumeMounts, nil
}

func (h *volumeHook) prepareCSIVolumes(req *interfaces.TaskPrestartRequest) ([]*drivers.MountConfig, error) {
	return nil, nil
}

func (h *volumeHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	volumes := partitionVolumesByType(h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup).Volumes)

	hostVolumeMounts, err := h.prepareHostVolumes(volumes[structs.VolumeTypeHost], req)
	if err != nil {
		return err
	}

	csiVolumeMounts, err := h.prepareCSIVolumes(req)
	if err != nil {
		return err
	}

	// Because this hook is also ran on restores, we only add mounts that do not
	// already exist. Although this loop is somewhat expensive, there are only
	// a small number of mounts that exist within most individual tasks. We may
	// want to revisit this using a `hookdata` param to be "mount only once"
	mounts := h.runner.hookResources.getMounts()
	for _, m := range hostVolumeMounts {
		mounts = ensureMountpointInserted(mounts, m)
	}
	for _, m := range csiVolumeMounts {
		mounts = ensureMountpointInserted(mounts, m)
	}
	h.runner.hookResources.setMounts(mounts)

	return nil
}
