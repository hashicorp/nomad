// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hostvolumemanager

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	cstructs "github.com/hashicorp/nomad/client/structs"
)

type HostVolumeManager struct {
	log     hclog.Logger
	plugins *sync.Map
}

func NewHostVolumeManager(sharedMountDir string, logger hclog.Logger) *HostVolumeManager {
	log := logger.Named("host_volumes")

	mgr := &HostVolumeManager{
		log:     log,
		plugins: &sync.Map{},
	}
	// db TODO(1.10.0): discover plugins on disk, need a new plugin dir
	// TODO: how do we define the external mounter plugins? plugin configs?
	mgr.setPlugin("mkdir", &HostVolumePluginMkdir{
		ID:         "mkdir",
		TargetPath: sharedMountDir,
		log:        log.With("plugin_id", "mkdir"),
	})
	mgr.setPlugin("example-host-volume", &HostVolumePluginExternal{
		ID:         "example-host-volume",
		Executable: "/opt/nomad/hostvolumeplugins/example-host-volume",
		TargetPath: sharedMountDir,
		log:        log.With("plugin_id", "example-host-volume"),
	})
	return mgr
}

// db TODO(1.10.0): fingerprint elsewhere / on sighup, and SetPlugin from afar?
func (hvm *HostVolumeManager) setPlugin(id string, plug HostVolumePlugin) {
	hvm.plugins.Store(id, plug)
}

func (hvm *HostVolumeManager) getPlugin(id string) (HostVolumePlugin, bool) {
	obj, ok := hvm.plugins.Load(id)
	if !ok {
		return nil, false
	}
	return obj.(HostVolumePlugin), true
}

func (hvm *HostVolumeManager) Create(ctx context.Context,
	req *cstructs.ClientHostVolumeCreateRequest) (*cstructs.ClientHostVolumeCreateResponse, error) {

	plug, ok := hvm.getPlugin(req.PluginID)
	if !ok {
		return nil, fmt.Errorf("no such plugin %q", req.PluginID)
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

	plug, ok := hvm.getPlugin(req.PluginID)
	if !ok {
		return nil, fmt.Errorf("no such plugin %q", req.PluginID)
	}

	err := plug.Delete(ctx, req)
	if err != nil {
		return nil, err
	}

	resp := &cstructs.ClientHostVolumeDeleteResponse{}

	// db TODO(1.10.0): save the client state!

	return resp, nil
}
