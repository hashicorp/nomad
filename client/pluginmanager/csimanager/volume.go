package csimanager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
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

	// containerMountPoint is the location _inside_ the plugin container that the
	// `mountRoot` is bound in to.
	containerMountPoint string

	// requiresStaging shows whether the plugin requires that the volume manager
	// calls NodeStageVolume and NodeUnstageVolume RPCs during setup and teardown
	requiresStaging bool
}

func newVolumeManager(logger hclog.Logger, plugin csi.CSIPlugin, rootDir, containerRootDir string, requiresStaging bool) *volumeManager {
	return &volumeManager{
		logger:              logger.Named("volume_manager"),
		plugin:              plugin,
		mountRoot:           rootDir,
		containerMountPoint: containerRootDir,
		requiresStaging:     requiresStaging,
		volumes:             make(map[string]interface{}),
	}
}

func (v *volumeManager) stagingDirForVolume(root string, vol *structs.CSIVolume) string {
	return filepath.Join(root, StagingDirName, vol.ID, "todo-provide-usage-options")
}

func (v *volumeManager) allocDirForVolume(root string, vol *structs.CSIVolume, alloc *structs.Allocation) string {
	return filepath.Join(root, AllocSpecificDirName, alloc.ID, vol.ID, "todo-provide-usage-options")
}

// ensureStagingDir attempts to create a directory for use when staging a volume
// and then validates that the path is not already a mount point for e.g an
// existing volume stage.
//
// Returns whether the directory is a pre-existing mountpoint, the staging path,
// and any errors that occurred.
func (v *volumeManager) ensureStagingDir(vol *structs.CSIVolume) (string, bool, error) {
	stagingPath := v.stagingDirForVolume(v.mountRoot, vol)

	// Make the staging path, owned by the Nomad User
	if err := os.MkdirAll(stagingPath, 0700); err != nil && !os.IsExist(err) {
		return "", false, fmt.Errorf("failed to create staging directory for volume (%s): %v", vol.ID, err)

	}

	// Validate that it is not already a mount point
	m := mount.New()
	isNotMount, err := m.IsNotAMountPoint(stagingPath)
	if err != nil {
		return "", false, fmt.Errorf("mount point detection failed for volume (%s): %v", vol.ID, err)
	}

	return stagingPath, !isNotMount, nil
}

// ensureAllocDir attempts to create a directory for use when publishing a volume
// and then validates that the path is not already a mount point (e.g when reattaching
// to existing allocs).
//
// Returns whether the directory is a pre-existing mountpoint, the publish path,
// and any errors that occurred.
func (v *volumeManager) ensureAllocDir(vol *structs.CSIVolume, alloc *structs.Allocation) (string, bool, error) {
	allocPath := v.allocDirForVolume(v.mountRoot, vol, alloc)

	// Make the alloc path, owned by the Nomad User
	if err := os.MkdirAll(allocPath, 0700); err != nil && !os.IsExist(err) {
		return "", false, fmt.Errorf("failed to create allocation directory for volume (%s): %v", vol.ID, err)
	}

	// Validate that it is not already a mount point
	m := mount.New()
	isNotMount, err := m.IsNotAMountPoint(allocPath)
	if err != nil {
		return "", false, fmt.Errorf("mount point detection failed for volume (%s): %v", vol.ID, err)
	}

	return allocPath, !isNotMount, nil
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
func (v *volumeManager) stageVolume(ctx context.Context, vol *structs.CSIVolume) error {
	logger := hclog.FromContext(ctx)
	logger.Trace("Preparing volume staging environment")
	hostStagingPath, isMount, err := v.ensureStagingDir(vol)
	if err != nil {
		return err
	}
	pluginStagingPath := v.stagingDirForVolume(v.containerMountPoint, vol)

	logger.Trace("Volume staging environment", "pre-existing_mount", isMount, "host_staging_path", hostStagingPath, "plugin_staging_path", pluginStagingPath)

	if isMount {
		logger.Debug("re-using existing staging mount for volume", "staging_path", hostStagingPath)
		return nil
	}

	capability, err := capabilitiesFromVolume(vol)
	if err != nil {
		return err
	}

	// We currently treat all explicit CSI NodeStageVolume errors (aside from timeouts, codes.ResourceExhausted, and codes.Unavailable)
	// as fatal.
	// In the future, we can provide more useful error messages based on
	// different types of error. For error documentation see:
	// https://github.com/container-storage-interface/spec/blob/4731db0e0bc53238b93850f43ab05d9355df0fd9/spec.md#nodestagevolume-errors
	return v.plugin.NodeStageVolume(ctx,
		vol.ID,
		nil, /* TODO: Get publishContext from Server */
		pluginStagingPath,
		capability,
		grpc_retry.WithPerRetryTimeout(DefaultMountActionTimeout),
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100*time.Millisecond)),
	)
}

func (v *volumeManager) publishVolume(ctx context.Context, vol *structs.CSIVolume, alloc *structs.Allocation) (*MountInfo, error) {
	logger := hclog.FromContext(ctx)
	var pluginStagingPath string
	if v.requiresStaging {
		pluginStagingPath = v.stagingDirForVolume(v.containerMountPoint, vol)
	}

	hostTargetPath, isMount, err := v.ensureAllocDir(vol, alloc)
	if err != nil {
		return nil, err
	}
	pluginTargetPath := v.allocDirForVolume(v.containerMountPoint, vol, alloc)

	if isMount {
		logger.Debug("Re-using existing published volume for allocation")
		return &MountInfo{Source: hostTargetPath}, nil
	}

	capabilities, err := capabilitiesFromVolume(vol)
	if err != nil {
		return nil, err
	}

	err = v.plugin.NodePublishVolume(ctx, &csi.NodePublishVolumeRequest{
		VolumeID:          vol.ID,
		PublishContext:    nil, // TODO: get publishcontext from server
		StagingTargetPath: pluginStagingPath,
		TargetPath:        pluginTargetPath,
		VolumeCapability:  capabilities,
	},
		grpc_retry.WithPerRetryTimeout(DefaultMountActionTimeout),
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100*time.Millisecond)),
	)

	return &MountInfo{Source: hostTargetPath}, err
}

// MountVolume performs the steps required for using a given volume
// configuration for the provided allocation.
//
// TODO: Validate remote volume attachment and implement.
func (v *volumeManager) MountVolume(ctx context.Context, vol *structs.CSIVolume, alloc *structs.Allocation) (*MountInfo, error) {
	logger := v.logger.With("volume_id", vol.ID, "alloc_id", alloc.ID)
	ctx = hclog.WithContext(ctx, logger)

	if v.requiresStaging {
		if err := v.stageVolume(ctx, vol); err != nil {
			return nil, err
		}
	}

	return v.publishVolume(ctx, vol, alloc)
}

// unstageVolume is the inverse operation of `stageVolume` and must be called
// once for each staging path that a volume has been staged under.
// It is safe to call multiple times and a plugin is required to return OK if
// the volume has been unstaged or was never staged on the node.
func (v *volumeManager) unstageVolume(ctx context.Context, vol *structs.CSIVolume) error {
	logger := hclog.FromContext(ctx)
	logger.Trace("Unstaging volume")
	stagingPath := v.stagingDirForVolume(v.containerMountPoint, vol)
	return v.plugin.NodeUnstageVolume(ctx,
		vol.ID,
		stagingPath,
		grpc_retry.WithPerRetryTimeout(DefaultMountActionTimeout),
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100*time.Millisecond)),
	)
}

func combineErrors(maybeErrs ...error) error {
	var result *multierror.Error
	for _, err := range maybeErrs {
		if err == nil {
			continue
		}

		result = multierror.Append(result, err)
	}

	return result.ErrorOrNil()
}

func (v *volumeManager) unpublishVolume(ctx context.Context, vol *structs.CSIVolume, alloc *structs.Allocation) error {
	pluginTargetPath := v.allocDirForVolume(v.containerMountPoint, vol, alloc)

	rpcErr := v.plugin.NodeUnpublishVolume(ctx, vol.ID, pluginTargetPath,
		grpc_retry.WithPerRetryTimeout(DefaultMountActionTimeout),
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100*time.Millisecond)),
	)

	hostTargetPath := v.allocDirForVolume(v.mountRoot, vol, alloc)
	if _, err := os.Stat(hostTargetPath); os.IsNotExist(err) {
		// Host Target Path already got destroyed, just return any rpcErr
		return rpcErr
	}

	// Host Target Path was not cleaned up, attempt to do so here. If it's still
	// a mount then removing the dir will fail and we'll return any rpcErr and the
	// file error.
	rmErr := os.Remove(hostTargetPath)
	if rmErr != nil {
		return combineErrors(rpcErr, rmErr)
	}

	// We successfully removed the directory, return any rpcErrors that were
	// encountered, but because we got here, they were probably flaky or was
	// cleaned up externally. We might want to just return `nil` here in the
	// future.
	return rpcErr
}

func (v *volumeManager) UnmountVolume(ctx context.Context, vol *structs.CSIVolume, alloc *structs.Allocation) error {
	logger := v.logger.With("volume_id", vol.ID, "alloc_id", alloc.ID)
	ctx = hclog.WithContext(ctx, logger)

	err := v.unpublishVolume(ctx, vol, alloc)
	if err != nil {
		return err
	}

	if !v.requiresStaging {
		return nil
	}

	// TODO(GH-7029): Implement volume usage tracking and only unstage volumes
	//                when the last alloc stops using it.
	return v.unstageVolume(ctx, vol)
}
