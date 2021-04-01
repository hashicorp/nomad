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

// CSIControllerRequest interface lets us set embedded CSIControllerQuery
// fields in the server
type CSIControllerRequest interface {
	SetControllerNodeID(string)
}

// CSIControllerQuery is used to specify various flags for queries against CSI
// Controllers
type CSIControllerQuery struct {
	// ControllerNodeID is the node that should be targeted by the request
	ControllerNodeID string

	// PluginID is the plugin that should be targeted on the given node.
	PluginID string
}

func (c *CSIControllerQuery) SetControllerNodeID(nodeID string) {
	c.ControllerNodeID = nodeID
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

// ClientCSIControllerCreateVolumeRequest the RPC made from the server to a
// Nomad client to tell a CSI controller plugin on that client to perform
// CreateVolume
type ClientCSIControllerCreateVolumeRequest struct {
	Name               string
	VolumeCapabilities []*structs.CSIVolumeCapability
	Parameters         map[string]string
	Secrets            structs.CSISecrets
	CapacityMin        int64
	CapacityMax        int64
	SnapshotID         string
	CloneID            string
	// TODO: topology is not yet supported
	// TopologyRequirement

	CSIControllerQuery
}

func (req *ClientCSIControllerCreateVolumeRequest) ToCSIRequest() (*csi.ControllerCreateVolumeRequest, error) {

	creq := &csi.ControllerCreateVolumeRequest{
		Name: req.Name,
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: req.CapacityMin,
			LimitBytes:    req.CapacityMax,
		},
		VolumeCapabilities: []*csi.VolumeCapability{},
		Parameters:         req.Parameters,
		Secrets:            req.Secrets,
		ContentSource: &csi.VolumeContentSource{
			CloneID:    req.CloneID,
			SnapshotID: req.SnapshotID,
		},
		// TODO: topology is not yet supported
		AccessibilityRequirements: &csi.TopologyRequirement{},
	}
	for _, cap := range req.VolumeCapabilities {
		ccap, err := csi.VolumeCapabilityFromStructs(cap.AttachmentMode, cap.AccessMode)
		if err != nil {
			return nil, err
		}
		creq.VolumeCapabilities = append(creq.VolumeCapabilities, ccap)
	}
	return creq, nil
}

type ClientCSIControllerCreateVolumeResponse struct {
	ExternalVolumeID string
	CapacityBytes    int64
	VolumeContext    map[string]string

	// TODO: topology is not yet supported
	// AccessibleTopology []*Topology
}

// ClientCSIControllerDeleteVolumeRequest the RPC made from the server to a
// Nomad client to tell a CSI controller plugin on that client to perform
// DeleteVolume
type ClientCSIControllerDeleteVolumeRequest struct {
	ExternalVolumeID string
	Secrets          structs.CSISecrets

	CSIControllerQuery
}

func (req *ClientCSIControllerDeleteVolumeRequest) ToCSIRequest() *csi.ControllerDeleteVolumeRequest {
	return &csi.ControllerDeleteVolumeRequest{
		ExternalVolumeID: req.ExternalVolumeID,
		Secrets:          req.Secrets,
	}
}

type ClientCSIControllerDeleteVolumeResponse struct{}

// ClientCSIControllerListVolumesVolumeRequest the RPC made from the server to
// a Nomad client to tell a CSI controller plugin on that client to perform
// ListVolumes
type ClientCSIControllerListVolumesRequest struct {
	// these pagination fields match the pagination fields of the plugins and
	// not Nomad's own fields, for clarity when mapping between the two RPCs
	MaxEntries    int32
	StartingToken string

	CSIControllerQuery
}

func (req *ClientCSIControllerListVolumesRequest) ToCSIRequest() *csi.ControllerListVolumesRequest {
	return &csi.ControllerListVolumesRequest{
		MaxEntries:    req.MaxEntries,
		StartingToken: req.StartingToken,
	}
}

type ClientCSIControllerListVolumesResponse struct {
	Entries   []*structs.CSIVolumeExternalStub
	NextToken string
}

// ClientCSIControllerCreateSnapshotRequest the RPC made from the server to a
// Nomad client to tell a CSI controller plugin on that client to perform
// CreateSnapshot
type ClientCSIControllerCreateSnapshotRequest struct {
	ExternalSourceVolumeID string
	Name                   string
	Secrets                structs.CSISecrets
	Parameters             map[string]string

	CSIControllerQuery
}

func (req *ClientCSIControllerCreateSnapshotRequest) ToCSIRequest() (*csi.ControllerCreateSnapshotRequest, error) {
	return &csi.ControllerCreateSnapshotRequest{
		VolumeID:   req.ExternalSourceVolumeID,
		Name:       req.Name,
		Secrets:    req.Secrets,
		Parameters: req.Parameters,
	}, nil
}

type ClientCSIControllerCreateSnapshotResponse struct {
	ID                     string
	ExternalSourceVolumeID string
	SizeBytes              int64
	CreateTime             int64
	IsReady                bool
}

// ClientCSIControllerDeleteSnapshotRequest the RPC made from the server to a
// Nomad client to tell a CSI controller plugin on that client to perform
// DeleteSnapshot
type ClientCSIControllerDeleteSnapshotRequest struct {
	ID      string
	Secrets structs.CSISecrets

	CSIControllerQuery
}

func (req *ClientCSIControllerDeleteSnapshotRequest) ToCSIRequest() *csi.ControllerDeleteSnapshotRequest {
	return &csi.ControllerDeleteSnapshotRequest{
		SnapshotID: req.ID,
		Secrets:    req.Secrets,
	}
}

type ClientCSIControllerDeleteSnapshotResponse struct{}

// ClientCSIControllerListSnapshotsRequest is the RPC made from the server to
// a Nomad client to tell a CSI controller plugin on that client to perform
// ListSnapshots
type ClientCSIControllerListSnapshotsRequest struct {
	// these pagination fields match the pagination fields of the plugins and
	// not Nomad's own fields, for clarity when mapping between the two RPCs
	MaxEntries    int32
	StartingToken string

	CSIControllerQuery
}

func (req *ClientCSIControllerListSnapshotsRequest) ToCSIRequest() *csi.ControllerListSnapshotsRequest {
	return &csi.ControllerListSnapshotsRequest{
		MaxEntries:    req.MaxEntries,
		StartingToken: req.StartingToken,
	}
}

type ClientCSIControllerListSnapshotsResponse struct {
	Entries   []*structs.CSISnapshot
	NextToken string
}

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
