package structs

import "github.com/hashicorp/nomad/plugins/csi"

// CSIVolumeAttachmentMode chooses the type of storage api that will be used to
// interact with the device.
type CSIVolumeAttachmentMode string

const (
	CSIVolumeAttachmentModeUnknown     CSIVolumeAttachmentMode = ""
	CSIVolumeAttachmentModeBlockDevice CSIVolumeAttachmentMode = "block-device"
	CSIVolumeAttachmentModeFilesystem  CSIVolumeAttachmentMode = "file-system"
)

func ValidCSIVolumeAttachmentMode(attachmentMode CSIVolumeAttachmentMode) bool {
	switch attachmentMode {
	case CSIVolumeAttachmentModeBlockDevice, CSIVolumeAttachmentModeFilesystem:
		return true
	default:
		return false
	}
}

// CSIVolumeAccessMode indicates how a volume should be used in a storage topology
// e.g whether the provider should make the volume available concurrently.
type CSIVolumeAccessMode string

const (
	CSIVolumeAccessModeUnknown CSIVolumeAccessMode = ""

	CSIVolumeAccessModeSingleNodeReader CSIVolumeAccessMode = "single-node-reader-only"
	CSIVolumeAccessModeSingleNodeWriter CSIVolumeAccessMode = "single-node-writer"

	CSIVolumeAccessModeMultiNodeReader       CSIVolumeAccessMode = "multi-node-reader-only"
	CSIVolumeAccessModeMultiNodeSingleWriter CSIVolumeAccessMode = "multi-node-single-writer"
	CSIVolumeAccessModeMultiNodeMultiWriter  CSIVolumeAccessMode = "multi-node-multi-writer"
)

// ValidCSIVolumeAccessMode checks to see that the provided access mode is a valid,
// non-empty access mode.
func ValidCSIVolumeAccessMode(accessMode CSIVolumeAccessMode) bool {
	switch accessMode {
	case CSIVolumeAccessModeSingleNodeReader, CSIVolumeAccessModeSingleNodeWriter,
		CSIVolumeAccessModeMultiNodeReader, CSIVolumeAccessModeMultiNodeSingleWriter,
		CSIVolumeAccessModeMultiNodeMultiWriter:
		return true
	default:
		return false
	}
}

// CSIVolumeMountOptions contains the mount options that should be provided when
// attaching and mounting a volume with the CSIVolumeAttachmentModeFilesystem
// attachment mode.
type CSIVolumeMountOptions struct {
	// Filesystem is the desired filesystem type that should be used by the volume
	// (e.g ext4, aufs, zfs). This field is optional.
	Filesystem string

	// MountFlags contain the mount options that should be used for the volume.
	// These may contain _sensitive_ data and should not be leaked to logs or
	// returned in debugging data.
	// The total size of this field must be under 4KiB.
	MountFlags []string
}

type ClientCSIControllerPublishVolumeRequest struct {
	PluginName string

	// The ID of the volume to be used on a node.
	// This field is REQUIRED.
	VolumeID string

	// The ID of the node. This field is REQUIRED. This must match the NodeID that
	// is fingerprinted by the target node for this plugin name.
	NodeID string

	// AttachmentMode indicates how the volume should be attached and mounted into
	// a task.
	AttachmentMode CSIVolumeAttachmentMode

	// AccessMode indicates the desired concurrent access model for the volume
	AccessMode CSIVolumeAccessMode

	// MountOptions is an optional field that contains additional configuration
	// when providing an AttachmentMode of CSIVolumeAttachmentModeFilesystem
	MountOptions *CSIVolumeMountOptions

	// ReadOnly indicates that the volume will be used in a readonly fashion. This
	// only works when the Controller has the PublishReadonly capability.
	ReadOnly bool
}

func (c *ClientCSIControllerPublishVolumeRequest) ToCSIRequest() *csi.ControllerPublishVolumeRequest {
	if c == nil {
		return &csi.ControllerPublishVolumeRequest{}
	}

	return &csi.ControllerPublishVolumeRequest{
		VolumeID: c.VolumeID,
		NodeID:   c.NodeID,
		ReadOnly: c.ReadOnly,
	}
}

type ClientCSIControllerPublishVolumeResponse struct {
	// Opaque static publish properties of the volume. SP MAY use this
	// field to ensure subsequent `NodeStageVolume` or `NodePublishVolume`
	// calls calls have contextual information.
	// The contents of this field SHALL be opaque to nomad.
	// The contents of this field SHALL NOT be mutable.
	// The contents of this field SHALL be safe for the nomad to cache.
	// The contents of this field SHOULD NOT contain sensitive
	// information.
	// The contents of this field SHOULD NOT be used for uniquely
	// identifying a volume. The `volume_id` alone SHOULD be sufficient to
	// identify the volume.
	// This field is OPTIONAL and when present MUST be passed to
	// subsequent `NodeStageVolume` or `NodePublishVolume` calls
	PublishContext map[string]string
}
