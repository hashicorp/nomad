package structs

import "github.com/hashicorp/nomad/plugins/csi"

type ClientCSIControllerPublishVolumeRequest struct {
	PluginName string

	// The ID of the volume to be used on a node.
	// This field is REQUIRED.
	VolumeID string

	// The ID of the node. This field is REQUIRED. This must match the NodeID that
	// is fingerprinted by the target node for this plugin name.
	NodeID string

	// TODO: Fingerprint the following correctly:

	// Volume capability describing how the CO intends to use this volume.
	// SP MUST ensure the CO can use the published volume as described.
	// Otherwise SP MUST return the appropriate gRPC error code.
	// This is a REQUIRED field.
	// VolumeCapability *VolumeCapability `protobuf:"bytes,3,opt,name=volume_capability,json=volumeCapability,proto3" json:"volume_capability,omitempty"`

	// Indicates SP MUST publish the volume in readonly mode.
	// CO MUST set this field to false if SP does not have the
	// PUBLISH_READONLY controller capability.
	// This is a REQUIRED field.
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
