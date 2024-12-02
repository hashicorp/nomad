// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hostvolumemanager

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	cstructs "github.com/hashicorp/nomad/client/structs"
)

var (
	ErrPluginNotExists     = errors.New("no such plugin")
	ErrPluginNotExecutable = errors.New("plugin not executable")
)

type HostVolumeManager struct {
	pluginDir      string
	sharedMountDir string
	stateMgr       HostVolumeStateManager

	log hclog.Logger
}

func NewHostVolumeManager(logger hclog.Logger,
	stateMgr HostVolumeStateManager,
	pluginDir, sharedMountDir string) (*HostVolumeManager, error) {

	log := logger.Named("host_volume_mgr")

	// db TODO(1.10.0): how do we define the external mounter plugins? plugin configs?
	hvm := &HostVolumeManager{
		pluginDir:      pluginDir,
		sharedMountDir: sharedMountDir,
		stateMgr:       stateMgr,
		log:            log,
	}

	if err := hvm.restoreState(stateMgr); err != nil {
		return nil, err
	}

	return hvm, nil
}

func (hvm *HostVolumeManager) restoreState(state HostVolumeStateManager) error {
	vols, err := state.GetDynamicHostVolumes()
	if err != nil {
		return err
	}
	if len(vols) == 0 {
		return nil // nothing to do
	}

	// re-"create" the volumes - plugins have the best knowledge of their
	// side effects, and they should be idempotent.
	var wg sync.WaitGroup
	for _, vol := range vols {
		wg.Add(1)
		func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			// note: this will rewrite client state that we just restored
			if _, err := hvm.Create(ctx, vol.CreateReq); err != nil {
				hvm.log.Error("failed to restore", "volume_id", vol.ID, "error", err)
				// db TODO: multierror w/ mutex?
			}
		}()
	}
	wg.Wait()
	return nil
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

	volState := &HostVolumeState{
		ID:        req.ID,
		CreateReq: req,
	}
	if err := hvm.stateMgr.PutDynamicHostVolume(volState); err != nil {
		hvm.log.Error("failed to save volume in state", "volume_id", req.ID, "error", err)
		// db TODO: bail or nah?
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
		// db TODO: bail or nah?
	}

	return resp, nil
}
