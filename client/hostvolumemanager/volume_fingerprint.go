// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hostvolumemanager

import (
	"context"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

// this file is for fingerprinting *volumes*
// *plugins* are detected in client/fingerprint/dynamic_host_volumes.go

// HostVolumeNodeUpdater is used to add or remove volumes from the Node.
type HostVolumeNodeUpdater func(name string, volume *structs.ClientHostVolumeConfig)

// VolumeMap keys are volume `name`s, identical to Node.HostVolumes.
type VolumeMap map[string]*structs.ClientHostVolumeConfig

// UpdateVolumeMap returns true if it changes the provided `volumes` map.
// If `vol` is nil, key `name` will be removed from the map, if present.
// If it is not nil, `name: vol` will be set on the map, if different.
//
// Since it may mutate the map, the caller should make a copy
// or acquire a lock as appropriate for their context.
func UpdateVolumeMap(log hclog.Logger, volumes VolumeMap, name string, vol *structs.ClientHostVolumeConfig) (changed bool) {
	current, exists := volumes[name]
	if vol == nil {
		if exists {
			delete(volumes, name)
			changed = true
		}
	} else {
		// if the volume already exists with no ID, it will be because it was
		// added to client agent config after having been previously created
		// as a dynamic vol. dynamic takes precedence, but log a warning.
		if exists && current.ID == "" {
			log.Warn("overriding static host volume with dynamic", "name", name, "id", vol.ID)
		}
		if !exists || !vol.Equal(current) {
			volumes[name] = vol
			changed = true
		}
	}
	return changed
}

// WaitForFirstFingerprint implements client.FingerprintingPluginManager
// so any existing volumes are added to the client node on agent start.
func (hvm *HostVolumeManager) WaitForFirstFingerprint(ctx context.Context) <-chan struct{} {
	// the fingerprint manager puts batchFirstFingerprintsTimeout (50 seconds)
	// on the context that it sends to us here so we don't need another
	// timeout. we just need to cancel to report when we are done.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	volumes, err := hvm.restoreFromState(ctx)
	if err != nil {
		hvm.log.Error("failed to restore state", "error", err)
		return ctx.Done()
	}
	for name, vol := range volumes {
		hvm.updateNodeVols(name, vol) // => batchNodeUpdates.updateNodeFromHostVolume()
	}
	return ctx.Done()
}
func (hvm *HostVolumeManager) Run()      {}
func (hvm *HostVolumeManager) Shutdown() {}
func (hvm *HostVolumeManager) PluginType() string {
	// "Plugin"Type is misleading, because this is for *volumes* but ok.
	return "dynamic_host_volume"
}
