package csimanager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/mount"
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
// volumes are stored by an enriched volume usage struct as the CSI Spec requires
// slightly different usage based on the given usage model.
type volumeManager struct {
	logger hclog.Logger
	plugin csi.CSIPlugin

	volumes map[string]interface{}
	// volumesMu sync.Mutex

	// mountRoot is the root of where plugin directories and mounts may be created
	// e.g /opt/nomad.d/statedir/csi/my-csi-plugin/
	mountRoot string

	// requiresStaging shows whether the plugin requires that the volume manager
	// calls NodeStageVolume and NodeUnstageVolume RPCs during setup and teardown
	requiresStaging bool
}

func newVolumeManager(logger hclog.Logger, plugin csi.CSIPlugin, rootDir string, requiresStaging bool) *volumeManager {
	return &volumeManager{
		logger:          logger.Named("volume_manager"),
		plugin:          plugin,
		mountRoot:       rootDir,
		requiresStaging: requiresStaging,
		volumes:         make(map[string]interface{}),
	}
}

func (v *volumeManager) stagingDirForVolume(vol *structs.CSIVolume) string {
	return filepath.Join(v.mountRoot, StagingDirName, vol.ID, "todo-provide-usage-options")
}

// ensureStagingDir attempts to create a directory for use when staging a volume
// and then validates that the path is not already a mount point for e.g an
// existing volume stage.
//
// Returns whether the directory is a pre-existing mountpoint, the staging path,
// and any errors that occured.
func (v *volumeManager) ensureStagingDir(vol *structs.CSIVolume) (bool, string, error) {
	stagingPath := v.stagingDirForVolume(vol)

	// Make the staging path, owned by the Nomad User
	if err := os.MkdirAll(stagingPath, 0700); err != nil && !os.IsExist(err) {
		return false, "", fmt.Errorf("failed to create staging directory for volume (%s): %v", vol.ID, err)
	}

	// Validate that it is not already a mount point
	m := mount.New()
	isNotMount, err := m.IsNotAMountPoint(stagingPath)
	if err != nil {
		return false, "", fmt.Errorf("mount point detection failed for volume (%s): %v", vol.ID, err)
	}

	return !isNotMount, stagingPath, nil
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
