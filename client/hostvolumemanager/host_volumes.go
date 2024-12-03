// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hostvolumemanager

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
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

type HostVolumeManager struct {
	pluginDir      string
	sharedMountDir string
	stateMgr       HostVolumeStateManager

	log hclog.Logger
}

func NewHostVolumeManager(logger hclog.Logger,
	state HostVolumeStateManager, restoreTimeout time.Duration,
	pluginDir, sharedMountDir string) (*HostVolumeManager, error) {

	log := logger.Named("host_volume_mgr")

	// db TODO(1.10.0): how do we define the external mounter plugins? plugin configs?
	hvm := &HostVolumeManager{
		pluginDir:      pluginDir,
		sharedMountDir: sharedMountDir,
		stateMgr:       state,
		log:            log,
	}

	if err := hvm.restoreState(state, restoreTimeout); err != nil {
		return nil, err
	}

	return hvm, nil
}

func (hvm *HostVolumeManager) restoreState(state HostVolumeStateManager, timeout time.Duration) error {
	vols, err := state.GetDynamicHostVolumes()
	if err != nil {
		return err
	}
	if len(vols) == 0 {
		return nil // nothing to do
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

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			if _, err := plug.Create(ctx, vol.CreateReq); err != nil {
				// plugin execution errors are only logged
				hvm.log.Error("failed to restore", "plugin_id", vol.CreateReq.PluginID, "volume_id", vol.ID, "error", err)
			}
			return nil
		})
	}
	mErr := group.Wait()
	return helper.FlattenMultierror(mErr.ErrorOrNil())
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

	// db TODO(1.10.0): now we need to add the volume to the node fingerprint!

	resp := &cstructs.ClientHostVolumeCreateResponse{
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

	resp := &cstructs.ClientHostVolumeDeleteResponse{}

	if err := hvm.stateMgr.DeleteDynamicHostVolume(req.ID); err != nil {
		hvm.log.Error("failed to delete volume in state", "volume_id", req.ID, "error", err)
		return nil, err // bail so a user may retry
	}

	return resp, nil
}
