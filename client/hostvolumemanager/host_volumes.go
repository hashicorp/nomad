// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hostvolumemanager

import (
	"context"
	"errors"
	"path/filepath"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	ErrPluginNotExists     = errors.New("no such plugin")
	ErrPluginNotExecutable = errors.New("plugin not executable")
)

type HostVolumeStateManager interface {
	PutDynamicHostVolume(*cstructs.HostVolumeState) error
	GetDynamicHostVolumes() ([]*cstructs.HostVolumeState, error)
	DeleteDynamicHostVolume(string) error
}

type Config struct {
	// PluginDir is where external plugins may be found.
	PluginDir string

	// SharedMountDir is where plugins should place the directory
	// that will later become a volume HostPath
	SharedMountDir string

	// StateMgr manages client state to restore on agent restarts.
	StateMgr HostVolumeStateManager

	// UpdateNodeVols is run to update the node when a volume is created
	// or deleted.
	UpdateNodeVols HostVolumeNodeUpdater
}

type HostVolumeManager struct {
	pluginDir      string
	sharedMountDir string
	stateMgr       HostVolumeStateManager
	updateNodeVols HostVolumeNodeUpdater
	log            hclog.Logger
}

func NewHostVolumeManager(logger hclog.Logger, config Config) *HostVolumeManager {
	return &HostVolumeManager{
		pluginDir:      config.PluginDir,
		sharedMountDir: config.SharedMountDir,
		stateMgr:       config.StateMgr,
		updateNodeVols: config.UpdateNodeVols,
		log:            logger.Named("host_volume_manager"),
	}
}

func genVolConfig(req *cstructs.ClientHostVolumeCreateRequest, resp *HostVolumePluginCreateResponse) *structs.ClientHostVolumeConfig {
	if req == nil || resp == nil {
		return nil
	}
	return &structs.ClientHostVolumeConfig{
		Name: req.Name,
		ID:   req.ID,
		Path: resp.Path,

		// dynamic volumes, like CSI, have more robust `capabilities`,
		// so we always set ReadOnly to false, and let the scheduler
		// decide when to ignore this and check capabilities instead.
		ReadOnly: false,
	}
}

func (hvm *HostVolumeManager) restoreFromState(ctx context.Context) (VolumeMap, error) {
	vols, err := hvm.stateMgr.GetDynamicHostVolumes()
	if err != nil {
		return nil, err
	}

	volumes := make(VolumeMap)
	var mut sync.Mutex

	if len(vols) == 0 {
		return volumes, nil // nothing to do
	}

	// re-"create" the volumes - plugins have the best knowledge of their
	// side effects, and they must be idempotent.
	group := multierror.Group{}
	for _, vol := range vols {
		group.Go(func() error { // db TODO(1.10.0): document that plugins must be safe to run concurrently
			// missing plugins with associated volumes in state are considered
			// client-stopping errors. they need to be fixed by cluster admins.
			plug, err := hvm.getPlugin(vol.CreateReq.PluginID)
			if err != nil {
				return err
			}

			resp, err := plug.Create(ctx, vol.CreateReq)
			if err != nil {
				// plugin execution errors are only logged
				hvm.log.Error("failed to restore", "plugin_id", vol.CreateReq.PluginID, "volume_id", vol.ID, "error", err)
				return nil
			}
			mut.Lock()
			volumes[vol.CreateReq.Name] = genVolConfig(vol.CreateReq, resp)
			mut.Unlock()
			return nil
		})
	}
	mErr := group.Wait()
	return volumes, helper.FlattenMultierror(mErr.ErrorOrNil())
}

func (hvm *HostVolumeManager) getPlugin(id string) (HostVolumePlugin, error) {
	log := hvm.log.With("plugin_id", id)

	if id == HostVolumePluginMkdirID {
		return &HostVolumePluginMkdir{
			ID:         HostVolumePluginMkdirID,
			TargetPath: hvm.sharedMountDir,
			log:        log,
		}, nil
	}

	path := filepath.Join(hvm.pluginDir, id)
	return NewHostVolumePluginExternal(log, id, path, hvm.sharedMountDir)
}

func (hvm *HostVolumeManager) Create(ctx context.Context,
	req *cstructs.ClientHostVolumeCreateRequest) (*cstructs.ClientHostVolumeCreateResponse, error) {

	plug, err := hvm.getPlugin(req.PluginID)
	if err != nil {
		return nil, err
	}

	pluginResp, err := plug.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	volState := &cstructs.HostVolumeState{
		ID:        req.ID,
		CreateReq: req,
	}
	if err := hvm.stateMgr.PutDynamicHostVolume(volState); err != nil {
		// if we fail to write to state, delete the volume so it isn't left
		// lying around without Nomad knowing about it.
		hvm.log.Error("failed to save volume in state, so deleting", "volume_id", req.ID, "error", err)
		delErr := plug.Delete(ctx, &cstructs.ClientHostVolumeDeleteRequest{
			ID:         req.ID,
			PluginID:   req.PluginID,
			NodeID:     req.NodeID,
			HostPath:   hvm.sharedMountDir,
			Parameters: req.Parameters,
		})
		if delErr != nil {
			hvm.log.Warn("error deleting volume after state store failure", "volume_id", req.ID, "error", delErr)
			err = multierror.Append(err, delErr)
		}
		return nil, helper.FlattenMultierror(err)
	}

	hvm.updateNodeVols(req.Name, genVolConfig(req, pluginResp))

	resp := &cstructs.ClientHostVolumeCreateResponse{
		VolumeName:    req.Name,
		VolumeID:      req.ID,
		HostPath:      pluginResp.Path,
		CapacityBytes: pluginResp.SizeBytes,
	}

	return resp, nil
}

func (hvm *HostVolumeManager) Delete(ctx context.Context,
	req *cstructs.ClientHostVolumeDeleteRequest) (*cstructs.ClientHostVolumeDeleteResponse, error) {

	plug, err := hvm.getPlugin(req.PluginID)
	if err != nil {
		return nil, err
	}

	err = plug.Delete(ctx, req)
	if err != nil {
		return nil, err
	}

	if err := hvm.stateMgr.DeleteDynamicHostVolume(req.ID); err != nil {
		hvm.log.Error("failed to delete volume in state", "volume_id", req.ID, "error", err)
		return nil, err // bail so a user may retry
	}

	hvm.updateNodeVols(req.Name, nil)

	resp := &cstructs.ClientHostVolumeDeleteResponse{
		VolumeName: req.Name,
		VolumeID:   req.ID,
	}

	return resp, nil
}
