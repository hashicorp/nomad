// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hostvolumemanager

import (
	"context"
	"errors"
	"fmt"
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
	ErrVolumeNameExists    = errors.New("volume name already exists on this node")
)

// HostVolumeStateManager manages the lifecycle of volumes in client state.
type HostVolumeStateManager interface {
	PutDynamicHostVolume(*cstructs.HostVolumeState) error
	GetDynamicHostVolumes() ([]*cstructs.HostVolumeState, error)
	DeleteDynamicHostVolume(string) error
}

// Config is used to configure a HostVolumeManager.
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

// HostVolumeManager executes plugins, manages volume metadata in client state,
// and registers volumes with the client node.
type HostVolumeManager struct {
	pluginDir      string
	sharedMountDir string
	stateMgr       HostVolumeStateManager
	updateNodeVols HostVolumeNodeUpdater
	builtIns       map[string]HostVolumePlugin
	locker         *volLocker
	log            hclog.Logger
}

// NewHostVolumeManager includes default builtin plugins.
func NewHostVolumeManager(logger hclog.Logger, config Config) *HostVolumeManager {
	logger = logger.Named("host_volume_manager")
	return &HostVolumeManager{
		pluginDir:      config.PluginDir,
		sharedMountDir: config.SharedMountDir,
		stateMgr:       config.StateMgr,
		updateNodeVols: config.UpdateNodeVols,
		builtIns: map[string]HostVolumePlugin{
			HostVolumePluginMkdirID: &HostVolumePluginMkdir{
				ID:         HostVolumePluginMkdirID,
				TargetPath: config.SharedMountDir,
				log:        logger.With("plugin_id", HostVolumePluginMkdirID),
			},
		},
		locker: &volLocker{},
		log:    logger,
	}
}

// Create runs the appropriate plugin for the given request, saves the request
// to state, and updates the node with the volume.
func (hvm *HostVolumeManager) Create(ctx context.Context,
	req *cstructs.ClientHostVolumeCreateRequest) (*cstructs.ClientHostVolumeCreateResponse, error) {

	plug, err := hvm.getPlugin(req.PluginID)
	if err != nil {
		return nil, err
	}

	// can't have two of the same volume name w/ different IDs per client node
	if err := hvm.locker.lock(req.Name, req.ID); err != nil {
		return nil, err
	}

	pluginResp, err := plug.Create(ctx, req)
	if err != nil {
		hvm.locker.release(req.Name)
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
		// free up the volume name whether delete succeeded or not.
		hvm.locker.release(req.Name)
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

// Delete runs the appropriate plugin for the given request, removes it from
// state, and updates the node to remove the volume.
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

	// free up volume name for reuse
	hvm.locker.release(req.Name)

	hvm.updateNodeVols(req.Name, nil)

	resp := &cstructs.ClientHostVolumeDeleteResponse{
		VolumeName: req.Name,
		VolumeID:   req.ID,
	}

	return resp, nil
}

// getPlugin finds either a built-in plugin or an external plugin.
func (hvm *HostVolumeManager) getPlugin(id string) (HostVolumePlugin, error) {
	if plug, ok := hvm.builtIns[id]; ok {
		return plug, nil
	}
	log := hvm.log.With("plugin_id", id)
	path := filepath.Join(hvm.pluginDir, id)
	return NewHostVolumePluginExternal(log, id, path, hvm.sharedMountDir)
}

// restoreFromState loads all volumes from client state and runs Create for
// each one, so volumes are restored upon agent restart or host reboot.
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
		group.Go(func() error {
			// missing plugins with associated volumes in state are considered
			// client-stopping errors. they need to be fixed by cluster admins.
			plug, err := hvm.getPlugin(vol.CreateReq.PluginID)
			if err != nil {
				return err
			}

			// lock the name so future creates can't produce duplicates.
			err = hvm.locker.lock(vol.CreateReq.Name, vol.CreateReq.ID)
			// state should never have duplicate vol names, and restore happens
			// prior to node registration, so new creates shouldn't come in
			// concurrently, but check for error just in case.
			if err != nil {
				hvm.log.Error("error during restore", "volume_id", vol.ID, "error", err)
				// don't stop the world if it does happen, because an admin
				// couldn't do anything about it short of wiping client state.
				return nil
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

// genVolConfig generates the host volume config for the node to report as
// available to the servers for job scheduling.
func genVolConfig(req *cstructs.ClientHostVolumeCreateRequest, resp *HostVolumePluginCreateResponse) *structs.ClientHostVolumeConfig {
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

// volLocker is used to ensure that volumes on each node are unique by name.
// The volume scheduler will prevent this too, but only after node fingerprint,
// so we need to protect against concurrent duplicate creates.
type volLocker struct {
	locks sync.Map
}

// lock the provided name, error if it was already locked with a different ID
func (l *volLocker) lock(name, id string) error {
	current, exists := l.locks.LoadOrStore(name, id)
	if exists && id != current.(string) {
		return fmt.Errorf("%w: name=%q id=%q", ErrVolumeNameExists, name, id)
	}
	return nil
}

func (l *volLocker) release(name string) {
	l.locks.Delete(name)
}

// only used in tests to assert lock state
func (l *volLocker) isLocked(name string) bool {
	_, locked := l.locks.Load(name)
	return locked
}
