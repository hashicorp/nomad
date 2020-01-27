package csimanager

import (
	"context"
	"errors"

	"github.com/hashicorp/nomad/client/pluginmanager"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	DriverNotFoundErr = errors.New("Driver not found")
)

type MountInfo struct {
}

type VolumeMounter interface {
	MountVolume(ctx context.Context, vol *structs.CSIVolume, alloc *structs.Allocation) (*MountInfo, error)
	UnmountVolume(ctx context.Context, vol *structs.CSIVolume, alloc *structs.Allocation) error
}

type Manager interface {
	// PluginManager returns a PluginManager for use by the node fingerprinter.
	PluginManager() pluginmanager.PluginManager

	// MounterForVolume returns a VolumeMounter for the given requested volume.
	// If there is no plugin registered for this volume type, a DriverNotFoundErr
	// will be returned.
	MounterForVolume(ctx context.Context, volume *structs.CSIVolume) (VolumeMounter, error)

	// Shutdown shuts down the Manager and unmounts any locally attached volumes.
	Shutdown()
}
