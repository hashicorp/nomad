// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"

	log "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

type volumeHook struct {
	alloc   *structs.Allocation
	runner  *TaskRunner
	logger  log.Logger
	taskEnv *taskenv.TaskEnv
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

func validateHostVolumes(requestedByAlias map[string]*structs.VolumeRequest, clientVolumesByName map[string]*structs.ClientHostVolumeConfig, allocName string) error {
	var result error

	for _, req := range requestedByAlias {
		// This is a defensive check, but this function should only ever receive
		// host-type volumes.
		if req.Type != structs.VolumeTypeHost {
			continue
		}

		source := req.Source
		if req.PerAlloc {
			source = source + structs.AllocSuffix(allocName)
		}

		_, ok := clientVolumesByName[source]
		if !ok {
			result = multierror.Append(result, fmt.Errorf("missing %s", source))
		}
	}

	return result
}

// hostVolumeMountConfigurations takes the users requested volume mounts,
// volumes, and the client host volume configuration and converts them into a
// format that can be used by drivers.
func (h *volumeHook) hostVolumeMountConfigurations(taskMounts []*structs.VolumeMount, taskVolumesByAlias map[string]*structs.VolumeRequest, clientVolumesByName map[string]*structs.ClientHostVolumeConfig, allocName string) ([]*drivers.MountConfig, error) {
	var mounts []*drivers.MountConfig
	for _, m := range taskMounts {
		req, ok := taskVolumesByAlias[m.Volume]
		if !ok {
			// This function receives only the task volumes that are of type Host,
			// if we can't find a group volume then we assume the mount is for another
			// type.
			continue
		}

		// This is a defensive check, but this function should only ever receive
		// host-type volumes.
		if req.Type != structs.VolumeTypeHost {
			continue
		}

		source := req.Source
		if req.PerAlloc {
			source = source + structs.AllocSuffix(allocName)
		}
		hostVolume, ok := clientVolumesByName[source]
		if !ok {
			// Should never happen, but unless the client volumes were mutated during
			// the execution of this hook.
			return nil, fmt.Errorf("no host volume named: %s", source)
		}

		mcfg := &drivers.MountConfig{
			HostPath:        hostVolume.Path,
			TaskPath:        m.Destination,
			Readonly:        hostVolume.ReadOnly || req.ReadOnly || m.ReadOnly,
			PropagationMode: m.PropagationMode,
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

func (h *volumeHook) prepareHostVolumes(req *interfaces.TaskPrestartRequest, volumes map[string]*structs.VolumeRequest) ([]*drivers.MountConfig, error) {
	hostVolumes := h.runner.clientConfig.Node.HostVolumes

	// Always validate volumes to ensure that we do not allow volumes to be used
	// if a host is restarted and loses the host volume configuration.
	if err := validateHostVolumes(volumes, hostVolumes, req.Alloc.Name); err != nil {
		h.logger.Error("Requested Host Volume does not exist", "existing", hostVolumes, "requested", volumes)
		return nil, fmt.Errorf("host volume validation error: %v", err)
	}

	hostVolumeMounts, err := h.hostVolumeMountConfigurations(req.Task.VolumeMounts, volumes, hostVolumes, req.Alloc.Name)
	if err != nil {
		h.logger.Error("Failed to generate host volume mounts", "error", err)
		return nil, err
	}

	if len(hostVolumeMounts) > 0 {
		caps, err := h.runner.DriverCapabilities()
		if err != nil {
			return nil, fmt.Errorf("could not validate task driver capabilities: %v", err)
		}
		if caps.MountConfigs == drivers.MountConfigSupportNone {
			return nil, fmt.Errorf(
				"task driver %q for %q does not support host volumes",
				h.runner.task.Driver, h.runner.task.Name)
		}
	}

	return hostVolumeMounts, nil
}

// partitionMountsByVolume takes a list of volume mounts and returns them in the
// form of volume-alias:[]volume-mount because one volume may be mounted multiple
// times.
func partitionMountsByVolume(xs []*structs.VolumeMount) map[string][]*structs.VolumeMount {
	result := make(map[string][]*structs.VolumeMount)
	for _, mount := range xs {
		result[mount.Volume] = append(result[mount.Volume], mount)
	}

	return result
}

func (h *volumeHook) prepareCSIVolumes(req *interfaces.TaskPrestartRequest, volumes map[string]*structs.VolumeRequest) ([]*drivers.MountConfig, error) {
	if len(volumes) == 0 {
		return nil, nil
	}

	var mounts []*drivers.MountConfig

	mountRequests := partitionMountsByVolume(req.Task.VolumeMounts)
	csiMountPoints := h.runner.allocHookResources.GetCSIMounts()
	for alias, request := range volumes {
		mountsForAlias, ok := mountRequests[alias]
		if !ok {
			// This task doesn't use the volume
			continue
		}

		csiMountPoint, ok := csiMountPoints[alias]
		if !ok {
			return nil, fmt.Errorf("No CSI Mount Point found for volume: %s", alias)
		}

		for _, m := range mountsForAlias {
			mcfg := &drivers.MountConfig{
				HostPath:        csiMountPoint.Source,
				TaskPath:        m.Destination,
				Readonly:        request.ReadOnly || m.ReadOnly,
				PropagationMode: m.PropagationMode,
			}
			mounts = append(mounts, mcfg)
		}
	}

	if len(mounts) > 0 {
		caps, err := h.runner.DriverCapabilities()
		if err != nil {
			return nil, fmt.Errorf("could not validate task driver capabilities: %v", err)
		}
		if caps.MountConfigs == drivers.MountConfigSupportNone {
			return nil, fmt.Errorf(
				"task driver %q for %q does not support CSI",
				h.runner.task.Driver, h.runner.task.Name)
		}
	}

	return mounts, nil
}

func (h *volumeHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	h.taskEnv = req.TaskEnv
	interpolateVolumeMounts(req.Task.VolumeMounts, h.taskEnv)

	volumes := partitionVolumesByType(h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup).Volumes)

	hostVolumeMounts, err := h.prepareHostVolumes(req, volumes[structs.VolumeTypeHost])
	if err != nil {
		return err
	}

	csiVolumeMounts, err := h.prepareCSIVolumes(req, volumes[structs.VolumeTypeCSI])
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

func interpolateVolumeMounts(mounts []*structs.VolumeMount, taskEnv *taskenv.TaskEnv) {
	for _, mount := range mounts {
		mount.Volume = taskEnv.ReplaceEnv(mount.Volume)
		mount.Destination = taskEnv.ReplaceEnv(mount.Destination)
		mount.PropagationMode = taskEnv.ReplaceEnv(mount.PropagationMode)
	}
}
