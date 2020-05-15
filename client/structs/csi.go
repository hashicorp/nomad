package structs

import (
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/csi"
)

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

// CSIControllerQuery is used to specify various flags for queries against CSI
// Controllers
type CSIControllerQuery struct {
	// ControllerNodeID is the node that should be targeted by the request
	ControllerNodeID string

	// PluginID is the plugin that should be targeted on the given node.
	PluginID string
}

type ClientCSIControllerValidateVolumeRequest struct {
	VolumeID string // note: this is the external ID

	AttachmentMode structs.CSIVolumeAttachmentMode
	AccessMode     structs.CSIVolumeAccessMode
	Secrets        structs.CSISecrets

	// Parameters as returned by storage provider in CreateVolumeResponse.
	// This field is optional.
	Parameters map[string]string

	// Volume context as returned by storage provider in CreateVolumeResponse.
	// This field is optional.
	Context map[string]string

	CSIControllerQuery
}

func (c *ClientCSIControllerValidateVolumeRequest) ToCSIRequest() (*csi.ControllerValidateVolumeRequest, error) {
	if c == nil {
		return &csi.ControllerValidateVolumeRequest{}, nil
	}

	caps, err := csi.VolumeCapabilityFromStructs(c.AttachmentMode, c.AccessMode)
	if err != nil {
		return nil, err
	}

	return &csi.ControllerValidateVolumeRequest{
		ExternalID:   c.VolumeID,
		Secrets:      c.Secrets,
		Capabilities: caps,
		Parameters:   c.Parameters,
		Context:      c.Context,
	}, nil
}

type ClientCSIControllerValidateVolumeResponse struct {
}

type ClientCSIControllerAttachVolumeRequest struct {
	// The external ID of the volume to be used on a node.
	// This field is REQUIRED.
	VolumeID string

	// The ID of the node. This field is REQUIRED. This must match the NodeID that
	// is fingerprinted by the target node for this plugin name.
	ClientCSINodeID string

	// AttachmentMode indicates how the volume should be attached and mounted into
	// a task.
	AttachmentMode structs.CSIVolumeAttachmentMode

	// AccessMode indicates the desired concurrent access model for the volume
	AccessMode structs.CSIVolumeAccessMode

	// MountOptions is an optional field that contains additional configuration
	// when providing an AttachmentMode of CSIVolumeAttachmentModeFilesystem
	MountOptions *CSIVolumeMountOptions

	// ReadOnly indicates that the volume will be used in a readonly fashion. This
	// only works when the Controller has the PublishReadonly capability.
	ReadOnly bool

	// Secrets required by plugin to complete the controller publish
	// volume request. This field is OPTIONAL.
	Secrets structs.CSISecrets

	// Volume context as returned by storage provider in CreateVolumeResponse.
	// This field is optional.
	VolumeContext map[string]string

	CSIControllerQuery
}

func (c *ClientCSIControllerAttachVolumeRequest) ToCSIRequest() (*csi.ControllerPublishVolumeRequest, error) {
	if c == nil {
		return &csi.ControllerPublishVolumeRequest{}, nil
	}

	caps, err := csi.VolumeCapabilityFromStructs(c.AttachmentMode, c.AccessMode)
	if err != nil {
		return nil, err
	}

	return &csi.ControllerPublishVolumeRequest{
		ExternalID:       c.VolumeID,
		NodeID:           c.ClientCSINodeID,
		VolumeCapability: caps,
		ReadOnly:         c.ReadOnly,
		Secrets:          c.Secrets,
		VolumeContext:    c.VolumeContext,
	}, nil
}

// ClientCSIControllerDetachVolumeRequest is the RPC made from the server to
// a Nomad client to tell a CSI controller plugin on that client to perform
// ControllerUnpublish for a volume on a specific client.
type ClientCSIControllerAttachVolumeResponse struct {
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

type ClientCSIControllerDetachVolumeRequest struct {
	// The external ID of the volume to be unpublished for the node
	// This field is REQUIRED.
	VolumeID string

	// The CSI Node ID for the Node that the volume should be detached from.
	// This field is REQUIRED. This must match the NodeID that is fingerprinted
	// by the target node for this plugin name.
	ClientCSINodeID string

	// Secrets required by plugin to complete the controller unpublish
	// volume request. This field is OPTIONAL.
	Secrets structs.CSISecrets

	CSIControllerQuery
}

func (c *ClientCSIControllerDetachVolumeRequest) ToCSIRequest() *csi.ControllerUnpublishVolumeRequest {
	if c == nil {
		return &csi.ControllerUnpublishVolumeRequest{}
	}

	return &csi.ControllerUnpublishVolumeRequest{
		ExternalID: c.VolumeID,
		NodeID:     c.ClientCSINodeID,
	}
}

type ClientCSIControllerDetachVolumeResponse struct{}

// ClientCSINodeDetachVolumeRequest is the RPC made from the server to
// a Nomad client to tell a CSI node plugin on that client to perform
// NodeUnpublish and NodeUnstage.
type ClientCSINodeDetachVolumeRequest struct {
	PluginID   string // ID of the plugin that manages the volume (required)
	VolumeID   string // ID of the volume to be unpublished (required)
	AllocID    string // ID of the allocation we're unpublishing for (required)
	NodeID     string // ID of the Nomad client targeted
	ExternalID string // External ID of the volume to be unpublished (required)

	// These fields should match the original volume request so that
	// we can find the mount points on the client
	AttachmentMode structs.CSIVolumeAttachmentMode
	AccessMode     structs.CSIVolumeAccessMode
	ReadOnly       bool
}

type ClientCSINodeDetachVolumeResponse struct{}
