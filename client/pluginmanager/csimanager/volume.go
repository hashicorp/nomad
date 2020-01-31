package csimanager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
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

func (v *volumeManager) allocDirForVolume(vol *structs.CSIVolume, alloc *structs.Allocation) string {
	return filepath.Join(v.mountRoot, AllocSpecificDirName, alloc.ID, vol.ID, "todo-provide-usage-options")
}

// ensureStagingDir attempts to create a directory for use when staging a volume
// and then validates that the path is not already a mount point for e.g an
// existing volume stage.
//
// Returns whether the directory is a pre-existing mountpoint, the staging path,
// and any errors that occurred.
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

// ensureAllocDir attempts to create a directory for use when publishing a volume
// and then validates that the path is not already a mount point (e.g when reattaching
// to existing allocs).
//
// Returns whether the directory is a pre-existing mountpoint, the publish path,
// and any errors that occurred.
func (v *volumeManager) ensureAllocDir(vol *structs.CSIVolume, alloc *structs.Allocation) (bool, string, error) {
	allocPath := v.allocDirForVolume(vol, alloc)

	// Make the alloc path, owned by the Nomad User
	if err := os.MkdirAll(allocPath, 0700); err != nil && !os.IsExist(err) {
		return false, "", fmt.Errorf("failed to create allocation directory for volume (%s): %v", vol.ID, err)
	}

	// Validate that it is not already a mount point
	m := mount.New()
	isNotMount, err := m.IsNotAMountPoint(allocPath)
	if err != nil {
		return false, "", fmt.Errorf("mount point detection failed for volume (%s): %v", vol.ID, err)
	}

	return !isNotMount, allocPath, nil
}

func capabilitiesFromVolume(vol *structs.CSIVolume) (*csi.VolumeCapability, error) {
	var accessType csi.VolumeAccessType
	switch vol.AttachmentMode {
	case structs.CSIVolumeAttachmentModeBlockDevice:
		accessType = csi.VolumeAccessTypeBlock
	case structs.CSIVolumeAttachmentModeFilesystem:
		accessType = csi.VolumeAccessTypeMount
	default:
		// These fields are validated during job submission, but here we perform a
		// final check during transformation into the requisite CSI Data type to
		// defend against development bugs and corrupted state - and incompatible
		// nomad versions in the future.
		return nil, fmt.Errorf("Unknown volume attachment mode: %s", vol.AttachmentMode)
	}

	var accessMode csi.VolumeAccessMode
	switch vol.AccessMode {
	case structs.CSIVolumeAccessModeSingleNodeReader:
		accessMode = csi.VolumeAccessModeSingleNodeReaderOnly
	case structs.CSIVolumeAccessModeSingleNodeWriter:
		accessMode = csi.VolumeAccessModeSingleNodeWriter
	case structs.CSIVolumeAccessModeMultiNodeMultiWriter:
		accessMode = csi.VolumeAccessModeMultiNodeMultiWriter
	case structs.CSIVolumeAccessModeMultiNodeSingleWriter:
		accessMode = csi.VolumeAccessModeMultiNodeSingleWriter
	case structs.CSIVolumeAccessModeMultiNodeReader:
		accessMode = csi.VolumeAccessModeMultiNodeReaderOnly
	default:
		// These fields are validated during job submission, but here we perform a
		// final check during transformation into the requisite CSI Data type to
		// defend against development bugs and corrupted state - and incompatible
		// nomad versions in the future.
		return nil, fmt.Errorf("Unknown volume access mode: %v", vol.AccessMode)
	}

	return &csi.VolumeCapability{
		AccessType:         accessType,
		AccessMode:         accessMode,
		VolumeMountOptions: &csi.VolumeMountOptions{
			// GH-7007: Currently we have no way to provide these
		},
	}, nil
}

// stageVolume prepares a volume for use by allocations. When a plugin exposes
// the STAGE_UNSTAGE_VOLUME capability it MUST be called once-per-volume for a
// given usage mode before the volume can be NodePublish-ed.
func (v *volumeManager) stageVolume(ctx context.Context, vol *structs.CSIVolume) (string, error) {
	logger := hclog.FromContext(ctx)
	logger.Trace("Preparing volume staging environment")
	existingMount, stagingPath, err := v.ensureStagingDir(vol)
	if err != nil {
		return "", err
	}
	logger.Trace("Volume staging environment", "pre-existing_mount", existingMount, "staging_path", stagingPath)

	if existingMount {
		logger.Debug("re-using existing staging mount for volume", "staging_path", stagingPath)
		return stagingPath, nil
	}

	capability, err := capabilitiesFromVolume(vol)
	if err != nil {
		return "", err
	}

	// We currently treat all explicit CSI NodeStageVolume errors (aside from timeouts, codes.ResourceExhausted, and codes.Unavailable)
	// as fatal.
	// In the future, we can provide more useful error messages based on
	// different types of error. For error documentation see:
	// https://github.com/container-storage-interface/spec/blob/4731db0e0bc53238b93850f43ab05d9355df0fd9/spec.md#nodestagevolume-errors
	return stagingPath, v.plugin.NodeStageVolume(ctx,
		vol.ID,
		nil, /* TODO: Get publishContext from Server */
		stagingPath,
		capability,
		grpc_retry.WithPerRetryTimeout(DefaultMountActionTimeout),
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100*time.Millisecond)),
	)
}

func (v *volumeManager) publishVolume(ctx context.Context, vol *structs.CSIVolume, alloc *structs.Allocation, stagingPath string) (*MountInfo, error) {
	logger := hclog.FromContext(ctx)

	preexistingMountForAlloc, targetPath, err := v.ensureAllocDir(vol, alloc)
	if err != nil {
		return nil, err
	}

	if preexistingMountForAlloc {
		logger.Debug("Re-using existing published volume for allocation")
		return &MountInfo{Source: targetPath}, nil
	}

	capabilities, err := capabilitiesFromVolume(vol)
	if err != nil {
		return nil, err
	}

	err = v.plugin.NodePublishVolume(ctx, &csi.NodePublishVolumeRequest{
		VolumeID:          vol.ID,
		PublishContext:    nil, // TODO: get publishcontext from server
		StagingTargetPath: stagingPath,
		TargetPath:        targetPath,
		VolumeCapability:  capabilities,
	},
		grpc_retry.WithPerRetryTimeout(DefaultMountActionTimeout),
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100*time.Millisecond)),
	)

	return &MountInfo{Source: targetPath}, err
}

// MountVolume performs the steps required for using a given volume
// configuration for the provided allocation.
//
// TODO: Validate remote volume attachment and implement.
func (v *volumeManager) MountVolume(ctx context.Context, vol *structs.CSIVolume, alloc *structs.Allocation) (*MountInfo, error) {
	logger := v.logger.With("volume_id", vol.ID, "alloc_id", alloc.ID)
	ctx = hclog.WithContext(ctx, logger)

	var stagingPath string
	var err error

	if v.requiresStaging {
		stagingPath, err = v.stageVolume(ctx, vol)
		if err != nil {
			return nil, err
		}
	}

	return v.publishVolume(ctx, vol, alloc, stagingPath)
}

// unstageVolume is the inverse operation of `stageVolume` and must be called
// once for each staging path that a volume has been staged under.
// It is safe to call multiple times and a plugin is required to return OK if
// the volume has been unstaged or was never staged on the node.
func (v *volumeManager) unstageVolume(ctx context.Context, vol *structs.CSIVolume) error {
	logger := hclog.FromContext(ctx)
	logger.Trace("Unstaging volume")
	stagingPath := v.stagingDirForVolume(vol)
	return v.plugin.NodeUnstageVolume(ctx,
		vol.ID,
		stagingPath,
		grpc_retry.WithPerRetryTimeout(DefaultMountActionTimeout),
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100*time.Millisecond)),
	)
}

func (v *volumeManager) UnmountVolume(ctx context.Context, vol *structs.CSIVolume, alloc *structs.Allocation) error {
	logger := v.logger.With("volume_id", vol.ID, "alloc_id", alloc.ID)
	ctx = hclog.WithContext(ctx, logger)

	// TODO(GH-7030): NodeUnpublishVolume

	if !v.requiresStaging {
		return nil
	}

	// TODO(GH-7029): Implement volume usage tracking and only unstage volumes
	//                when the last alloc stops using it.
	return v.unstageVolume(ctx, vol)
}
