package csi

import (
	"context"
	"errors"
	"fmt"

	csipbv1 "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"google.golang.org/grpc"
)

// CSIPlugin implements a lightweight abstraction layer around a CSI Plugin.
// It validates that responses from storage providers (SP's), correctly conform
// to the specification before returning response data or erroring.
type CSIPlugin interface {
	base.BasePlugin

	// PluginProbe is used to verify that the plugin is in a healthy state
	PluginProbe(ctx context.Context) (bool, error)

	// PluginGetInfo is used to return semantic data about the plugin.
	// Response:
	//  - string: name, the name of the plugin in domain notation format.
	//  - string: version, the vendor version of the plugin
	PluginGetInfo(ctx context.Context) (string, string, error)

	// PluginGetCapabilities is used to return the available capabilities from the
	// identity service. This currently only looks for the CONTROLLER_SERVICE and
	// Accessible Topology Support
	PluginGetCapabilities(ctx context.Context) (*PluginCapabilitySet, error)

	// GetControllerCapabilities is used to get controller-specific capabilities
	// for a plugin.
	ControllerGetCapabilities(ctx context.Context) (*ControllerCapabilitySet, error)

	// ControllerPublishVolume is used to attach a remote volume to a cluster node.
	ControllerPublishVolume(ctx context.Context, req *ControllerPublishVolumeRequest, opts ...grpc.CallOption) (*ControllerPublishVolumeResponse, error)

	// ControllerUnpublishVolume is used to deattach a remote volume from a cluster node.
	ControllerUnpublishVolume(ctx context.Context, req *ControllerUnpublishVolumeRequest, opts ...grpc.CallOption) (*ControllerUnpublishVolumeResponse, error)

	// ControllerValidateCapabilities is used to validate that a volume exists and
	// supports the requested capability.
	ControllerValidateCapabilities(ctx context.Context, req *ControllerValidateVolumeRequest, opts ...grpc.CallOption) error

	// NodeGetCapabilities is used to return the available capabilities from the
	// Node Service.
	NodeGetCapabilities(ctx context.Context) (*NodeCapabilitySet, error)

	// NodeGetInfo is used to return semantic data about the current node in
	// respect to the SP.
	NodeGetInfo(ctx context.Context) (*NodeGetInfoResponse, error)

	// NodeStageVolume is used when a plugin has the STAGE_UNSTAGE volume capability
	// to prepare a volume for usage on a host. If err == nil, the response should
	// be assumed to be successful.
	NodeStageVolume(ctx context.Context, req *NodeStageVolumeRequest, opts ...grpc.CallOption) error

	// NodeUnstageVolume is used when a plugin has the STAGE_UNSTAGE volume capability
	// to undo the work performed by NodeStageVolume. If a volume has been staged,
	// this RPC must be called before freeing the volume.
	//
	// If err == nil, the response should be assumed to be successful.
	NodeUnstageVolume(ctx context.Context, volumeID string, stagingTargetPath string, opts ...grpc.CallOption) error

	// NodePublishVolume is used to prepare a volume for use by an allocation.
	// if err == nil the response should be assumed to be successful.
	NodePublishVolume(ctx context.Context, req *NodePublishVolumeRequest, opts ...grpc.CallOption) error

	// NodeUnpublishVolume is used to cleanup usage of a volume for an alloc. This
	// MUST be called before calling NodeUnstageVolume or ControllerUnpublishVolume
	// for the given volume.
	NodeUnpublishVolume(ctx context.Context, volumeID, targetPath string, opts ...grpc.CallOption) error

	// Shutdown the client and ensure any connections are cleaned up.
	Close() error
}

type NodePublishVolumeRequest struct {
	// The external ID of the volume to publish.
	ExternalID string

	// If the volume was attached via a call to `ControllerPublishVolume` then
	// we need to provide the returned PublishContext here.
	PublishContext map[string]string

	// The path to which the volume was staged by `NodeStageVolume`.
	// It MUST be an absolute path in the root filesystem of the process
	// serving this request.
	// E.g {the plugins internal mount path}/staging/volumeid/...
	//
	// It MUST be set if the Node Plugin implements the
	// `STAGE_UNSTAGE_VOLUME` node capability.
	StagingTargetPath string

	// The path to which the volume will be published.
	// It MUST be an absolute path in the root filesystem of the process serving this
	// request.
	// E.g {the plugins internal mount path}/per-alloc/allocid/volumeid/...
	//
	// The CO SHALL ensure uniqueness of target_path per volume.
	// The CO SHALL ensure that the parent directory of this path exists
	// and that the process serving the request has `read` and `write`
	// permissions to that parent directory.
	TargetPath string

	// Volume capability describing how the CO intends to use this volume.
	VolumeCapability *VolumeCapability

	Readonly bool

	// Secrets required by plugins to complete the node publish volume
	// request. This field is OPTIONAL.
	Secrets structs.CSISecrets

	// Volume context as returned by SP in the CSI
	// CreateVolumeResponse.Volume.volume_context which we don't implement but
	// can be entered by hand in the volume spec.  This field is OPTIONAL.
	VolumeContext map[string]string
}

func (r *NodePublishVolumeRequest) ToCSIRepresentation() *csipbv1.NodePublishVolumeRequest {
	if r == nil {
		return nil
	}

	return &csipbv1.NodePublishVolumeRequest{
		VolumeId:          r.ExternalID,
		PublishContext:    r.PublishContext,
		StagingTargetPath: r.StagingTargetPath,
		TargetPath:        r.TargetPath,
		VolumeCapability:  r.VolumeCapability.ToCSIRepresentation(),
		Readonly:          r.Readonly,
		Secrets:           r.Secrets,
		VolumeContext:     r.VolumeContext,
	}
}

func (r *NodePublishVolumeRequest) Validate() error {
	if r.ExternalID == "" {
		return errors.New("missing volume ID")
	}

	if r.TargetPath == "" {
		return errors.New("missing TargetPath")
	}

	if r.VolumeCapability == nil {
		return errors.New("missing VolumeCapabilities")
	}

	return nil
}

type NodeStageVolumeRequest struct {
	// The external ID of the volume to stage.
	ExternalID string

	// If the volume was attached via a call to `ControllerPublishVolume` then
	// we need to provide the returned PublishContext here.
	PublishContext map[string]string

	// The path to which the volume MAY be staged. It MUST be an
	// absolute path in the root filesystem of the process serving this
	// request, and MUST be a directory. The CO SHALL ensure that there
	// is only one `staging_target_path` per volume. The CO SHALL ensure
	// that the path is directory and that the process serving the
	// request has `read` and `write` permission to that directory. The
	// CO SHALL be responsible for creating the directory if it does not
	// exist.
	// This is a REQUIRED field.
	StagingTargetPath string

	// Volume capability describing how the CO intends to use this volume.
	VolumeCapability *VolumeCapability

	// Secrets required by plugins to complete the node stage volume
	// request. This field is OPTIONAL.
	Secrets structs.CSISecrets

	// Volume context as returned by SP in the CSI
	// CreateVolumeResponse.Volume.volume_context which we don't implement but
	// can be entered by hand in the volume spec.  This field is OPTIONAL.
	VolumeContext map[string]string
}

func (r *NodeStageVolumeRequest) ToCSIRepresentation() *csipbv1.NodeStageVolumeRequest {
	if r == nil {
		return nil
	}

	return &csipbv1.NodeStageVolumeRequest{
		VolumeId:          r.ExternalID,
		PublishContext:    r.PublishContext,
		StagingTargetPath: r.StagingTargetPath,
		VolumeCapability:  r.VolumeCapability.ToCSIRepresentation(),
		Secrets:           r.Secrets,
		VolumeContext:     r.VolumeContext,
	}
}

func (r *NodeStageVolumeRequest) Validate() error {
	if r.ExternalID == "" {
		return errors.New("missing volume ID")
	}

	if r.StagingTargetPath == "" {
		return errors.New("missing StagingTargetPath")
	}

	if r.VolumeCapability == nil {
		return errors.New("missing VolumeCapabilities")
	}

	return nil
}

type PluginCapabilitySet struct {
	hasControllerService bool
	hasTopologies        bool
}

func (p *PluginCapabilitySet) HasControllerService() bool {
	return p.hasControllerService
}

// HasTopologies indicates whether the volumes for this plugin are equally
// accessible by all nodes in the cluster.
// If true, we MUST use the topology information when scheduling workloads.
func (p *PluginCapabilitySet) HasToplogies() bool {
	return p.hasTopologies
}

func (p *PluginCapabilitySet) IsEqual(o *PluginCapabilitySet) bool {
	return p.hasControllerService == o.hasControllerService && p.hasTopologies == o.hasTopologies
}

func NewTestPluginCapabilitySet(topologies, controller bool) *PluginCapabilitySet {
	return &PluginCapabilitySet{
		hasTopologies:        topologies,
		hasControllerService: controller,
	}
}

func NewPluginCapabilitySet(capabilities *csipbv1.GetPluginCapabilitiesResponse) *PluginCapabilitySet {
	cs := &PluginCapabilitySet{}

	pluginCapabilities := capabilities.GetCapabilities()

	for _, pcap := range pluginCapabilities {
		if svcCap := pcap.GetService(); svcCap != nil {
			switch svcCap.Type {
			case csipbv1.PluginCapability_Service_UNKNOWN:
				continue
			case csipbv1.PluginCapability_Service_CONTROLLER_SERVICE:
				cs.hasControllerService = true
			case csipbv1.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS:
				cs.hasTopologies = true
			default:
				continue
			}
		}
	}

	return cs
}

type ControllerCapabilitySet struct {
	HasPublishUnpublishVolume    bool
	HasPublishReadonly           bool
	HasListVolumes               bool
	HasListVolumesPublishedNodes bool
}

func NewControllerCapabilitySet(resp *csipbv1.ControllerGetCapabilitiesResponse) *ControllerCapabilitySet {
	cs := &ControllerCapabilitySet{}

	pluginCapabilities := resp.GetCapabilities()
	for _, pcap := range pluginCapabilities {
		if c := pcap.GetRpc(); c != nil {
			switch c.Type {
			case csipbv1.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME:
				cs.HasPublishUnpublishVolume = true
			case csipbv1.ControllerServiceCapability_RPC_PUBLISH_READONLY:
				cs.HasPublishReadonly = true
			case csipbv1.ControllerServiceCapability_RPC_LIST_VOLUMES:
				cs.HasListVolumes = true
			case csipbv1.ControllerServiceCapability_RPC_LIST_VOLUMES_PUBLISHED_NODES:
				cs.HasListVolumesPublishedNodes = true
			default:
				continue
			}
		}
	}

	return cs
}

type ControllerValidateVolumeRequest struct {
	ExternalID   string
	Secrets      structs.CSISecrets
	Capabilities *VolumeCapability
	Parameters   map[string]string
	Context      map[string]string
}

func (r *ControllerValidateVolumeRequest) ToCSIRepresentation() *csipbv1.ValidateVolumeCapabilitiesRequest {
	if r == nil {
		return nil
	}

	return &csipbv1.ValidateVolumeCapabilitiesRequest{
		VolumeId:      r.ExternalID,
		VolumeContext: r.Context,
		VolumeCapabilities: []*csipbv1.VolumeCapability{
			r.Capabilities.ToCSIRepresentation(),
		},
		Parameters: r.Parameters,
		Secrets:    r.Secrets,
	}
}

type ControllerPublishVolumeRequest struct {
	ExternalID       string
	NodeID           string
	ReadOnly         bool
	VolumeCapability *VolumeCapability
	Secrets          structs.CSISecrets
	VolumeContext    map[string]string
}

func (r *ControllerPublishVolumeRequest) ToCSIRepresentation() *csipbv1.ControllerPublishVolumeRequest {
	if r == nil {
		return nil
	}

	return &csipbv1.ControllerPublishVolumeRequest{
		VolumeId:         r.ExternalID,
		NodeId:           r.NodeID,
		Readonly:         r.ReadOnly,
		VolumeCapability: r.VolumeCapability.ToCSIRepresentation(),
		Secrets:          r.Secrets,
		VolumeContext:    r.VolumeContext,
	}
}

func (r *ControllerPublishVolumeRequest) Validate() error {
	if r.ExternalID == "" {
		return errors.New("missing volume ID")
	}
	if r.NodeID == "" {
		return errors.New("missing NodeID")
	}
	return nil
}

type ControllerPublishVolumeResponse struct {
	PublishContext map[string]string
}

type ControllerUnpublishVolumeRequest struct {
	ExternalID string
	NodeID     string
	Secrets    structs.CSISecrets
}

func (r *ControllerUnpublishVolumeRequest) ToCSIRepresentation() *csipbv1.ControllerUnpublishVolumeRequest {
	if r == nil {
		return nil
	}

	return &csipbv1.ControllerUnpublishVolumeRequest{
		VolumeId: r.ExternalID,
		NodeId:   r.NodeID,
		Secrets:  r.Secrets,
	}
}

func (r *ControllerUnpublishVolumeRequest) Validate() error {
	if r.ExternalID == "" {
		return errors.New("missing ExternalID")
	}
	if r.NodeID == "" {
		// the spec allows this but it would unpublish the
		// volume from all nodes
		return errors.New("missing NodeID")
	}
	return nil
}

type ControllerUnpublishVolumeResponse struct{}

type NodeCapabilitySet struct {
	HasStageUnstageVolume bool
}

func NewNodeCapabilitySet(resp *csipbv1.NodeGetCapabilitiesResponse) *NodeCapabilitySet {
	cs := &NodeCapabilitySet{}
	pluginCapabilities := resp.GetCapabilities()
	for _, pcap := range pluginCapabilities {
		if c := pcap.GetRpc(); c != nil {
			switch c.Type {
			case csipbv1.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME:
				cs.HasStageUnstageVolume = true
			default:
				continue
			}
		}
	}

	return cs
}

// VolumeAccessMode represents the desired access mode of the CSI Volume
type VolumeAccessMode csipbv1.VolumeCapability_AccessMode_Mode

var _ fmt.Stringer = VolumeAccessModeUnknown

var (
	VolumeAccessModeUnknown               = VolumeAccessMode(csipbv1.VolumeCapability_AccessMode_UNKNOWN)
	VolumeAccessModeSingleNodeWriter      = VolumeAccessMode(csipbv1.VolumeCapability_AccessMode_SINGLE_NODE_WRITER)
	VolumeAccessModeSingleNodeReaderOnly  = VolumeAccessMode(csipbv1.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY)
	VolumeAccessModeMultiNodeReaderOnly   = VolumeAccessMode(csipbv1.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY)
	VolumeAccessModeMultiNodeSingleWriter = VolumeAccessMode(csipbv1.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER)
	VolumeAccessModeMultiNodeMultiWriter  = VolumeAccessMode(csipbv1.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER)
)

func (a VolumeAccessMode) String() string {
	return a.ToCSIRepresentation().String()
}

func (a VolumeAccessMode) ToCSIRepresentation() csipbv1.VolumeCapability_AccessMode_Mode {
	return csipbv1.VolumeCapability_AccessMode_Mode(a)
}

// VolumeAccessType represents the filesystem apis that the user intends to use
// with the volume. E.g whether it will be used as a block device or if they wish
// to have a mounted filesystem.
type VolumeAccessType int32

var _ fmt.Stringer = VolumeAccessTypeBlock

var (
	VolumeAccessTypeBlock VolumeAccessType = 1
	VolumeAccessTypeMount VolumeAccessType = 2
)

func (v VolumeAccessType) String() string {
	if v == VolumeAccessTypeBlock {
		return "VolumeAccessType.Block"
	} else if v == VolumeAccessTypeMount {
		return "VolumeAccessType.Mount"
	} else {
		return "VolumeAccessType.Unspecified"
	}
}

// VolumeCapability describes the overall usage requirements for a given CSI Volume
type VolumeCapability struct {
	AccessType VolumeAccessType
	AccessMode VolumeAccessMode

	// Indicate that the volume will be accessed via the filesystem API.
	MountVolume *structs.CSIMountOptions
}

func VolumeCapabilityFromStructs(sAccessType structs.CSIVolumeAttachmentMode, sAccessMode structs.CSIVolumeAccessMode) (*VolumeCapability, error) {
	var accessType VolumeAccessType
	switch sAccessType {
	case structs.CSIVolumeAttachmentModeBlockDevice:
		accessType = VolumeAccessTypeBlock
	case structs.CSIVolumeAttachmentModeFilesystem:
		accessType = VolumeAccessTypeMount
	default:
		// These fields are validated during job submission, but here we perform a
		// final check during transformation into the requisite CSI Data type to
		// defend against development bugs and corrupted state - and incompatible
		// nomad versions in the future.
		return nil, fmt.Errorf("Unknown volume attachment mode: %s", sAccessType)
	}

	var accessMode VolumeAccessMode
	switch sAccessMode {
	case structs.CSIVolumeAccessModeSingleNodeReader:
		accessMode = VolumeAccessModeSingleNodeReaderOnly
	case structs.CSIVolumeAccessModeSingleNodeWriter:
		accessMode = VolumeAccessModeSingleNodeWriter
	case structs.CSIVolumeAccessModeMultiNodeMultiWriter:
		accessMode = VolumeAccessModeMultiNodeMultiWriter
	case structs.CSIVolumeAccessModeMultiNodeSingleWriter:
		accessMode = VolumeAccessModeMultiNodeSingleWriter
	case structs.CSIVolumeAccessModeMultiNodeReader:
		accessMode = VolumeAccessModeMultiNodeReaderOnly
	default:
		// These fields are validated during job submission, but here we perform a
		// final check during transformation into the requisite CSI Data type to
		// defend against development bugs and corrupted state - and incompatible
		// nomad versions in the future.
		return nil, fmt.Errorf("Unknown volume access mode: %v", sAccessMode)
	}

	return &VolumeCapability{
		AccessType: accessType,
		AccessMode: accessMode,
	}, nil
}

func (c *VolumeCapability) ToCSIRepresentation() *csipbv1.VolumeCapability {
	if c == nil {
		return nil
	}

	vc := &csipbv1.VolumeCapability{
		AccessMode: &csipbv1.VolumeCapability_AccessMode{
			Mode: c.AccessMode.ToCSIRepresentation(),
		},
	}

	if c.AccessType == VolumeAccessTypeMount {
		opts := &csipbv1.VolumeCapability_MountVolume{}
		if c.MountVolume != nil {
			opts.FsType = c.MountVolume.FSType
			opts.MountFlags = c.MountVolume.MountFlags
		}
		vc.AccessType = &csipbv1.VolumeCapability_Mount{Mount: opts}
	} else {
		vc.AccessType = &csipbv1.VolumeCapability_Block{Block: &csipbv1.VolumeCapability_BlockVolume{}}
	}

	return vc
}
