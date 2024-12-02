// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hostvolumemanager

import (
	"context"
	"errors"
	"path/filepath"

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

	log hclog.Logger
}

func NewHostVolumeManager(logger hclog.Logger, pluginDir, sharedMountDir string) *HostVolumeManager {
	log := logger.Named("host_volume_mgr")

	// db TODO(1.10.0): how do we define the external mounter plugins? plugin configs?
	return &HostVolumeManager{
		log:            log,
		pluginDir:      pluginDir,
		sharedMountDir: sharedMountDir,
	}
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

	resp := &cstructs.ClientHostVolumeCreateResponse{
		HostPath:      pluginResp.Path,
		CapacityBytes: pluginResp.SizeBytes,
	}

	// db TODO(1.10.0): now we need to add it to the node fingerprint!
	// db TODO(1.10.0): and save it in client state!

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

	// db TODO(1.10.0): save the client state!

	return resp, nil
}
