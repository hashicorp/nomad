// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package csimanager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	logger  hclog.Logger
	eventer TriggerNodeEvent
	plugin  csi.CSIPlugin

	usageTracker *volumeUsageTracker

	// mountRoot is the root of where plugin directories and mounts may be created
	// e.g /opt/nomad.d/statedir/csi/my-csi-plugin/
	mountRoot string

	// containerMountPoint is the location _inside_ the plugin container that the
	// `mountRoot` is bound in to.
	containerMountPoint string

	// requiresStaging shows whether the plugin requires that the volume manager
	// calls NodeStageVolume and NodeUnstageVolume RPCs during setup and teardown
	requiresStaging bool

	// externalNodeID is the identity of a given nomad client as observed by the
	// storage provider (ex. a hostname, VM instance ID, etc.)
	externalNodeID string
}

func newVolumeManager(logger hclog.Logger, eventer TriggerNodeEvent, plugin csi.CSIPlugin, rootDir, containerRootDir string, requiresStaging bool, externalID string) *volumeManager {

	return &volumeManager{
		logger:              logger.Named("volume_manager"),
		eventer:             eventer,
		plugin:              plugin,
		mountRoot:           rootDir,
		containerMountPoint: containerRootDir,
		requiresStaging:     requiresStaging,
		usageTracker:        newVolumeUsageTracker(),
		externalNodeID:      externalID,
	}
}

func (v *volumeManager) stagingDirForVolume(root string, volID string, usage *UsageOptions) string {
	return filepath.Join(root, StagingDirName, volID, usage.ToFS())
}

func (v *volumeManager) allocDirForVolume(root string, volID, allocID string) string {
	return filepath.Join(root, AllocSpecificDirName, allocID, volID)
}

func (v *volumeManager) targetForVolume(root string, volID, allocID string, usage *UsageOptions) string {
	return filepath.Join(root, AllocSpecificDirName, allocID, volID, usage.ToFS())
}

// ensureStagingDir attempts to create a directory for use when staging a volume
// and then validates that the path is not already a mount point for e.g an
// existing volume stage.
//
// Returns whether the directory is a pre-existing mountpoint, the staging path,
// and any errors that occurred.
func (v *volumeManager) ensureStagingDir(vol *structs.CSIVolume, usage *UsageOptions) (string, bool, error) {
	stagingPath := v.stagingDirForVolume(v.mountRoot, vol.ID, usage)

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
func (v *volumeManager) ensureAllocDir(vol *structs.CSIVolume, alloc *structs.Allocation, usage *UsageOptions) (string, bool, error) {
	allocPath := v.allocDirForVolume(v.mountRoot, vol.ID, alloc.ID)

	// Make the alloc path, owned by the Nomad User
	if err := os.MkdirAll(allocPath, 0700); err != nil && !os.IsExist(err) {
		return "", false, fmt.Errorf("failed to create allocation directory for volume (%s): %v", vol.ID, err)
	}

	// Validate that the target is not already a mount point
	targetPath := v.targetForVolume(v.mountRoot, vol.ID, alloc.ID, usage)

	m := mount.New()
	isNotMount, err := m.IsNotAMountPoint(targetPath)

	switch {
	case errors.Is(err, os.ErrNotExist):
		// ignore; path does not exist and as such is not a mount
	case err != nil:
		return "", false, fmt.Errorf("mount point detection failed for volume (%s): %v", vol.ID, err)
	}

	return targetPath, !isNotMount, nil
}

func volumeCapability(vol *structs.CSIVolume, usage *UsageOptions) (*csi.VolumeCapability, error) {
	var opts *structs.CSIMountOptions
	if vol.MountOptions == nil {
		opts = usage.MountOptions
	} else {
		opts = vol.MountOptions.Copy()
		opts.Merge(usage.MountOptions)
	}

	capability, err := csi.VolumeCapabilityFromStructs(usage.AttachmentMode, usage.AccessMode, opts)
	if err != nil {
		return nil, err
	}

	return capability, nil
}

// stageVolume prepares a volume for use by allocations. When a plugin exposes
// the STAGE_UNSTAGE_VOLUME capability it MUST be called once-per-volume for a
// given usage mode before the volume can be NodePublish-ed.
func (v *volumeManager) stageVolume(ctx context.Context, vol *structs.CSIVolume, usage *UsageOptions, publishContext map[string]string) error {
	logger := hclog.FromContext(ctx)
	logger.Trace("Preparing volume staging environment")
	hostStagingPath, isMount, err := v.ensureStagingDir(vol, usage)
	if err != nil {
		return err
	}
	pluginStagingPath := v.stagingDirForVolume(v.containerMountPoint, vol.ID, usage)

	logger.Trace("Volume staging environment", "pre-existing_mount", isMount, "host_staging_path", hostStagingPath, "plugin_staging_path", pluginStagingPath)

	if isMount {
		logger.Debug("re-using existing staging mount for volume", "staging_path", hostStagingPath)
		return nil
	}

	capability, err := volumeCapability(vol, usage)
	if err != nil {
		return err
	}

	req := &csi.NodeStageVolumeRequest{
		ExternalID:        vol.RemoteID(),
		PublishContext:    publishContext,
		StagingTargetPath: pluginStagingPath,
		VolumeCapability:  capability,
		Secrets:           vol.Secrets,
		VolumeContext:     vol.Context,
	}

	// CSI NodeStageVolume errors for timeout, codes.Unavailable and
	// codes.ResourceExhausted are retried; all other errors are fatal.
	return v.plugin.NodeStageVolume(ctx, req,
		grpc_retry.WithPerRetryTimeout(DefaultMountActionTimeout),
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100*time.Millisecond)),
	)
}

func (v *volumeManager) publishVolume(ctx context.Context, vol *structs.CSIVolume, alloc *structs.Allocation, usage *UsageOptions, publishContext map[string]string) (*MountInfo, error) {
	logger := hclog.FromContext(ctx)
	var pluginStagingPath string
	if v.requiresStaging {
		pluginStagingPath = v.stagingDirForVolume(v.containerMountPoint, vol.ID, usage)
	}

	hostTargetPath, isMount, err := v.ensureAllocDir(vol, alloc, usage)
	if err != nil {
		return nil, err
	}
	pluginTargetPath := v.targetForVolume(v.containerMountPoint, vol.ID, alloc.ID, usage)

	if isMount {
		logger.Debug("Re-using existing published volume for allocation")
		return &MountInfo{Source: hostTargetPath}, nil
	}

	capabilities, err := volumeCapability(vol, usage)
	if err != nil {
		return nil, err
	}

	// CSI NodePublishVolume errors for timeout, codes.Unavailable and
	// codes.ResourceExhausted are retried; all other errors are fatal.
	err = v.plugin.NodePublishVolume(ctx, &csi.NodePublishVolumeRequest{
		ExternalID:        vol.RemoteID(),
		PublishContext:    publishContext,
		StagingTargetPath: pluginStagingPath,
		TargetPath:        pluginTargetPath,
		VolumeCapability:  capabilities,
		Readonly:          usage.ReadOnly,
		Secrets:           vol.Secrets,
		VolumeContext:     vol.Context,
	},
		grpc_retry.WithPerRetryTimeout(DefaultMountActionTimeout),
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100*time.Millisecond)),
	)

	return &MountInfo{Source: hostTargetPath}, err
}

// MountVolume performs the steps required for using a given volume
// configuration for the provided allocation.
// It is passed the publishContext from remote attachment, and specific usage
// modes from the CSI Hook.
// It then uses this state to stage and publish the volume as required for use
// by the given allocation.
func (v *volumeManager) MountVolume(ctx context.Context, vol *structs.CSIVolume, alloc *structs.Allocation, usage *UsageOptions, publishContext map[string]string) (mountInfo *MountInfo, err error) {
	logger := v.logger.With("volume_id", vol.ID, "alloc_id", alloc.ID)
	ctx = hclog.WithContext(ctx, logger)

	if v.requiresStaging {
		err = v.stageVolume(ctx, vol, usage, publishContext)
	}

	if err == nil {
		mountInfo, err = v.publishVolume(ctx, vol, alloc, usage, publishContext)
	}

	if err == nil {
		v.usageTracker.Claim(alloc.ID, vol.ID, usage)
	}

	event := structs.NewNodeEvent().
		SetSubsystem(structs.NodeEventSubsystemStorage).
		SetMessage("Mount volume").
		AddDetail("volume_id", vol.ID)
	if err == nil {
		event.AddDetail("success", "true")
	} else {
		event.AddDetail("success", "false")
		event.AddDetail("error", err.Error())
	}

	v.eventer(event)

	return mountInfo, err
}

// unstageVolume is the inverse operation of `stageVolume` and must be called
// once for each staging path that a volume has been staged under.
// It is safe to call multiple times and a plugin is required to return OK if
// the volume has been unstaged or was never staged on the node.
func (v *volumeManager) unstageVolume(ctx context.Context, volID, remoteID string, usage *UsageOptions) error {
	logger := hclog.FromContext(ctx)
	logger.Trace("Unstaging volume")
	stagingPath := v.stagingDirForVolume(v.containerMountPoint, volID, usage)

	// CSI NodeUnstageVolume errors for timeout, codes.Unavailable and
	// codes.ResourceExhausted are retried; all other errors are fatal.
	return v.plugin.NodeUnstageVolume(ctx,
		remoteID,
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

func (v *volumeManager) unpublishVolume(ctx context.Context, volID, remoteID, allocID string, usage *UsageOptions) error {
	pluginTargetPath := v.targetForVolume(v.containerMountPoint, volID, allocID, usage)

	// CSI NodeUnpublishVolume errors for timeout, codes.Unavailable and
	// codes.ResourceExhausted are retried; all other errors are fatal.
	rpcErr := v.plugin.NodeUnpublishVolume(ctx, remoteID, pluginTargetPath,
		grpc_retry.WithPerRetryTimeout(DefaultMountActionTimeout),
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100*time.Millisecond)),
	)

	hostTargetPath := v.targetForVolume(v.mountRoot, volID, allocID, usage)
	if _, err := os.Stat(hostTargetPath); os.IsNotExist(err) {
		if rpcErr != nil && strings.Contains(rpcErr.Error(), "no mount point") {
			// host target path was already destroyed, nothing to do here.
			// this helps us in the case that a previous GC attempt cleaned
			// up the volume on the node but the controller RPCs failed
			rpcErr = fmt.Errorf("%w: %v", structs.ErrCSIClientRPCIgnorable, rpcErr)
		}
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
	// cleaned up externally.
	return fmt.Errorf("%w: %v", structs.ErrCSIClientRPCIgnorable, rpcErr)
}

func (v *volumeManager) UnmountVolume(ctx context.Context, volID, remoteID, allocID string, usage *UsageOptions) (err error) {
	logger := v.logger.With("volume_id", volID, "alloc_id", allocID)
	ctx = hclog.WithContext(ctx, logger)

	err = v.unpublishVolume(ctx, volID, remoteID, allocID, usage)

	if err == nil || errors.Is(err, structs.ErrCSIClientRPCIgnorable) {
		canRelease := v.usageTracker.Free(allocID, volID, usage)
		if v.requiresStaging && canRelease {
			err = v.unstageVolume(ctx, volID, remoteID, usage)
		}
	}

	if errors.Is(err, structs.ErrCSIClientRPCIgnorable) {
		logger.Trace("unmounting volume failed with ignorable error", "error", err)
		err = nil
	}

	event := structs.NewNodeEvent().
		SetSubsystem(structs.NodeEventSubsystemStorage).
		SetMessage("Unmount volume").
		AddDetail("volume_id", volID)
	if err == nil {
		event.AddDetail("success", "true")
	} else {
		event.AddDetail("success", "false")
		event.AddDetail("error", err.Error())
	}

	v.eventer(event)

	return err
}

func (v *volumeManager) ExternalID() string {
	return v.externalNodeID
}

func (v *volumeManager) HasMount(_ context.Context, mountInfo *MountInfo) (bool, error) {
	m := mount.New()
	isNotMount, err := m.IsNotAMountPoint(mountInfo.Source)
	return !isNotMount, err
}
