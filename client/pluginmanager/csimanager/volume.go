package csimanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/csi"
)

var _ VolumeMounter = &volumeManager{}

const (
	DefaultMountActionTimeout = 2 * time.Minute
	StagingDirName            = "staging"
	AllocSpecificDirName      = "per-alloc"
)

// volumeManager handles the state of attached volumes for a given CSI Plugin.
//
// volumeManagers outlive the lifetime of a given allocation as volumes may be
// shared by multiple allocations on the same node.
//
// volumes are stored by an eriched volume usage struct as the CSI Spec requires
// slightly different usage based on the given usage model.
type volumeManager struct {
	logger hclog.Logger
	plugin csi.CSIPlugin

	volumes   map[string]interface{}
	volumesMu sync.Mutex

	// mountRoot is the root of where plugin directories and mounts may be created
	// e.g /opt/nomad.d/statedir/csi/my-csi-plugin/
	mountRoot string

	// requiresStaging shows whether the plugin requires that the volume manager
	// calls NodeStageVolume and NodeUnstageVolume RPCs during setup and teardown
	requiresStaging bool
}

func newVolumeManager(logger hclog.Logger, plugin csi.CSIPlugin, rootDir string) *volumeManager {
	return &volumeManager{
		logger:    logger.Named("volume_manager"),
		plugin:    plugin,
		mountRoot: rootDir,
	}
}

// MountVolume performs the steps required for using a given volume
// configuration for the provided allocation.
//
// TODO: Validate remote volume attachment and implement.
func (v *volumeManager) MountVolume(ctx context.Context, vol *structs.CSIVolume, alloc *structs.Allocation) (*MountInfo, error) {
	return nil, fmt.Errorf("Unimplemented")
}

func (v *volumeManager) UnmountVolume(ctx context.Context, vol *structs.CSIVolume, alloc *structs.Allocation) error {
	return fmt.Errorf("Unimplemented")
}
