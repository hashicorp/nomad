// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package csi

import (
	"context"
	"errors"
	"fmt"

	csipbv1 "github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
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

	// ControllerCreateVolume is used to create a remote volume in the
	// external storage provider
	ControllerCreateVolume(ctx context.Context, req *ControllerCreateVolumeRequest, opts ...grpc.CallOption) (*ControllerCreateVolumeResponse, error)

	// ControllerDeleteVolume is used to delete a remote volume in the
	// external storage provider
	ControllerDeleteVolume(ctx context.Context, req *ControllerDeleteVolumeRequest, opts ...grpc.CallOption) error

	// ControllerListVolumes is used to list all volumes available in the
	// external storage provider
	ControllerListVolumes(ctx context.Context, req *ControllerListVolumesRequest, opts ...grpc.CallOption) (*ControllerListVolumesResponse, error)

	// ControllerExpandVolume is used to expand a volume's size
	ControllerExpandVolume(ctx context.Context, req *ControllerExpandVolumeRequest, opts ...grpc.CallOption) (*ControllerExpandVolumeResponse, error)

	// ControllerCreateSnapshot is used to create a volume snapshot in the
	// external storage provider
	ControllerCreateSnapshot(ctx context.Context, req *ControllerCreateSnapshotRequest, opts ...grpc.CallOption) (*ControllerCreateSnapshotResponse, error)

	// ControllerDeleteSnapshot is used to delete a volume snapshot from the
	// external storage provider
	ControllerDeleteSnapshot(ctx context.Context, req *ControllerDeleteSnapshotRequest, opts ...grpc.CallOption) error

	// ControllerListSnapshots is used to list all volume snapshots available
	// in the external storage provider
	ControllerListSnapshots(ctx context.Context, req *ControllerListSnapshotsRequest, opts ...grpc.CallOption) (*ControllerListSnapshotsResponse, error)

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

	// NodeExpandVolume is used to expand a volume. This MUST be called after
	// any ControllerExpandVolume is called, but only if that RPC indicates
	// that node expansion is required
	NodeExpandVolume(ctx context.Context, req *NodeExpandVolumeRequest, opts ...grpc.CallOption) (*NodeExpandVolumeResponse, error)

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

// HasToplogies indicates whether the volumes for this plugin are equally
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
	HasCreateDeleteVolume        bool
	HasPublishUnpublishVolume    bool
	HasListVolumes               bool
	HasGetCapacity               bool
	HasCreateDeleteSnapshot      bool
	HasListSnapshots             bool
	HasCloneVolume               bool
	HasPublishReadonly           bool
	HasExpandVolume              bool
	HasListVolumesPublishedNodes bool
	HasVolumeCondition           bool
	HasGetVolume                 bool
}

func NewControllerCapabilitySet(resp *csipbv1.ControllerGetCapabilitiesResponse) *ControllerCapabilitySet {
	cs := &ControllerCapabilitySet{}

	pluginCapabilities := resp.GetCapabilities()
	for _, pcap := range pluginCapabilities {
		if c := pcap.GetRpc(); c != nil {
			switch c.Type {
			case csipbv1.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME:
				cs.HasCreateDeleteVolume = true
			case csipbv1.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME:
				cs.HasPublishUnpublishVolume = true
			case csipbv1.ControllerServiceCapability_RPC_LIST_VOLUMES:
				cs.HasListVolumes = true
			case csipbv1.ControllerServiceCapability_RPC_GET_CAPACITY:
				cs.HasGetCapacity = true
			case csipbv1.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT:
				cs.HasCreateDeleteSnapshot = true
			case csipbv1.ControllerServiceCapability_RPC_LIST_SNAPSHOTS:
				cs.HasListSnapshots = true
			case csipbv1.ControllerServiceCapability_RPC_CLONE_VOLUME:
				cs.HasCloneVolume = true
			case csipbv1.ControllerServiceCapability_RPC_PUBLISH_READONLY:
				cs.HasPublishReadonly = true
			case csipbv1.ControllerServiceCapability_RPC_EXPAND_VOLUME:
				cs.HasExpandVolume = true
			case csipbv1.ControllerServiceCapability_RPC_LIST_VOLUMES_PUBLISHED_NODES:
				cs.HasListVolumesPublishedNodes = true
			case csipbv1.ControllerServiceCapability_RPC_VOLUME_CONDITION:
				cs.HasVolumeCondition = true
			case csipbv1.ControllerServiceCapability_RPC_GET_VOLUME:
				cs.HasGetVolume = true
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
	Capabilities []*VolumeCapability
	Parameters   map[string]string
	Context      map[string]string
}

func (r *ControllerValidateVolumeRequest) ToCSIRepresentation() *csipbv1.ValidateVolumeCapabilitiesRequest {
	if r == nil {
		return nil
	}

	caps := make([]*csipbv1.VolumeCapability, 0, len(r.Capabilities))
	for _, cap := range r.Capabilities {
		caps = append(caps, cap.ToCSIRepresentation())
	}

	return &csipbv1.ValidateVolumeCapabilitiesRequest{
		VolumeId:           r.ExternalID,
		VolumeContext:      r.Context,
		VolumeCapabilities: caps,
		Parameters:         r.Parameters,
		Secrets:            r.Secrets,
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

type ControllerCreateVolumeRequest struct {
	// note that Name is intentionally differentiated from both CSIVolume.ID
	// and ExternalVolumeID. This name is only a recommendation for the
	// storage provider, and many will discard this suggestion
	Name                      string
	CapacityRange             *CapacityRange
	VolumeCapabilities        []*VolumeCapability
	Parameters                map[string]string
	Secrets                   structs.CSISecrets
	ContentSource             *VolumeContentSource
	AccessibilityRequirements *TopologyRequirement
}

func (r *ControllerCreateVolumeRequest) ToCSIRepresentation() *csipbv1.CreateVolumeRequest {
	if r == nil {
		return nil
	}
	caps := make([]*csipbv1.VolumeCapability, 0, len(r.VolumeCapabilities))
	for _, cap := range r.VolumeCapabilities {
		caps = append(caps, cap.ToCSIRepresentation())
	}
	req := &csipbv1.CreateVolumeRequest{
		Name:                      r.Name,
		CapacityRange:             r.CapacityRange.ToCSIRepresentation(),
		VolumeCapabilities:        caps,
		Parameters:                r.Parameters,
		Secrets:                   r.Secrets,
		VolumeContentSource:       r.ContentSource.ToCSIRepresentation(),
		AccessibilityRequirements: r.AccessibilityRequirements.ToCSIRepresentation(),
	}

	return req
}

func (r *ControllerCreateVolumeRequest) Validate() error {
	if r.Name == "" {
		return errors.New("missing Name")
	}
	if r.VolumeCapabilities == nil {
		return errors.New("missing VolumeCapabilities")
	}
	if r.CapacityRange != nil {
		if r.CapacityRange.LimitBytes == 0 && r.CapacityRange.RequiredBytes == 0 {
			return errors.New(
				"one of LimitBytes or RequiredBytes must be set if CapacityRange is set")
		}
		if r.CapacityRange.LimitBytes > 0 &&
			r.CapacityRange.LimitBytes < r.CapacityRange.RequiredBytes {
			return errors.New("LimitBytes cannot be less than RequiredBytes")
		}
	}
	if r.ContentSource != nil {
		if r.ContentSource.CloneID != "" && r.ContentSource.SnapshotID != "" {
			return errors.New(
				"one of SnapshotID or CloneID must be set if ContentSource is set")
		}
	}
	return nil
}

// VolumeContentSource is snapshot or volume that the plugin will use to
// create the new volume. At most one of these fields can be set, but nil (and
// not an empty struct) is expected by CSI plugins if neither field is set.
type VolumeContentSource struct {
	SnapshotID string
	CloneID    string
}

func (vcr *VolumeContentSource) ToCSIRepresentation() *csipbv1.VolumeContentSource {
	if vcr == nil {
		return nil
	}
	if vcr.CloneID != "" {
		return &csipbv1.VolumeContentSource{
			Type: &csipbv1.VolumeContentSource_Volume{
				Volume: &csipbv1.VolumeContentSource_VolumeSource{
					VolumeId: vcr.CloneID,
				},
			},
		}
	} else if vcr.SnapshotID != "" {
		return &csipbv1.VolumeContentSource{
			Type: &csipbv1.VolumeContentSource_Snapshot{
				Snapshot: &csipbv1.VolumeContentSource_SnapshotSource{
					SnapshotId: vcr.SnapshotID,
				},
			},
		}
	}
	// Nomad's RPCs will hand us an empty struct, not nil
	return nil
}

func newVolumeContentSource(src *csipbv1.VolumeContentSource) *VolumeContentSource {
	return &VolumeContentSource{
		SnapshotID: src.GetSnapshot().GetSnapshotId(),
		CloneID:    src.GetVolume().GetVolumeId(),
	}
}

type TopologyRequirement struct {
	Requisite []*Topology
	Preferred []*Topology
}

func (tr *TopologyRequirement) ToCSIRepresentation() *csipbv1.TopologyRequirement {
	if tr == nil {
		return nil
	}
	result := &csipbv1.TopologyRequirement{
		Requisite: []*csipbv1.Topology{},
		Preferred: []*csipbv1.Topology{},
	}
	for _, topo := range tr.Requisite {
		result.Requisite = append(result.Requisite,
			&csipbv1.Topology{Segments: topo.Segments})
	}
	for _, topo := range tr.Preferred {
		result.Preferred = append(result.Preferred,
			&csipbv1.Topology{Segments: topo.Segments})
	}
	return result
}

func newTopologies(src []*csipbv1.Topology) []*Topology {
	t := []*Topology{}
	for _, topo := range src {
		t = append(t, &Topology{Segments: topo.Segments})
	}
	return t
}

type ControllerCreateVolumeResponse struct {
	Volume *Volume
}

func NewCreateVolumeResponse(resp *csipbv1.CreateVolumeResponse) *ControllerCreateVolumeResponse {
	vol := resp.GetVolume()
	return &ControllerCreateVolumeResponse{Volume: &Volume{
		CapacityBytes:      vol.GetCapacityBytes(),
		ExternalVolumeID:   vol.GetVolumeId(),
		VolumeContext:      vol.GetVolumeContext(),
		ContentSource:      newVolumeContentSource(vol.GetContentSource()),
		AccessibleTopology: newTopologies(vol.GetAccessibleTopology()),
	}}
}

type Volume struct {
	CapacityBytes int64

	// this is differentiated from VolumeID so as not to create confusion
	// between the Nomad CSIVolume.ID and the storage provider's ID.
	ExternalVolumeID   string
	VolumeContext      map[string]string
	ContentSource      *VolumeContentSource
	AccessibleTopology []*Topology
}

type ControllerDeleteVolumeRequest struct {
	ExternalVolumeID string
	Secrets          structs.CSISecrets
}

func (r *ControllerDeleteVolumeRequest) ToCSIRepresentation() *csipbv1.DeleteVolumeRequest {
	if r == nil {
		return nil
	}
	return &csipbv1.DeleteVolumeRequest{
		VolumeId: r.ExternalVolumeID,
		Secrets:  r.Secrets,
	}
}

func (r *ControllerDeleteVolumeRequest) Validate() error {
	if r.ExternalVolumeID == "" {
		return errors.New("missing ExternalVolumeID")
	}
	return nil
}

type ControllerExpandVolumeRequest struct {
	ExternalVolumeID string
	RequiredBytes    int64
	LimitBytes       int64
	Capability       *VolumeCapability
	Secrets          structs.CSISecrets
}

func (r *ControllerExpandVolumeRequest) Validate() error {
	if r.ExternalVolumeID == "" {
		return errors.New("missing ExternalVolumeID")
	}
	if r.LimitBytes == 0 && r.RequiredBytes == 0 {
		return errors.New("one of LimitBytes or RequiredBytes must be set")
	}
	// per the spec: "A value of 0 is equal to an unspecified field value."
	// so in this case, only error if both are set.
	if r.LimitBytes > 0 && (r.LimitBytes < r.RequiredBytes) {
		return errors.New("LimitBytes cannot be less than RequiredBytes")
	}
	return nil
}

func (r *ControllerExpandVolumeRequest) ToCSIRepresentation() *csipbv1.ControllerExpandVolumeRequest {
	if r == nil {
		return nil
	}
	return &csipbv1.ControllerExpandVolumeRequest{
		VolumeId: r.ExternalVolumeID,
		CapacityRange: &csipbv1.CapacityRange{
			RequiredBytes: r.RequiredBytes,
			LimitBytes:    r.LimitBytes,
		},
		Secrets:          r.Secrets,
		VolumeCapability: r.Capability.ToCSIRepresentation(),
	}
}

type ControllerExpandVolumeResponse struct {
	CapacityBytes         int64
	NodeExpansionRequired bool
}

type ControllerListVolumesRequest struct {
	MaxEntries    int32
	StartingToken string
}

func (r *ControllerListVolumesRequest) ToCSIRepresentation() *csipbv1.ListVolumesRequest {
	if r == nil {
		return nil
	}
	return &csipbv1.ListVolumesRequest{
		MaxEntries:    r.MaxEntries,
		StartingToken: r.StartingToken,
	}
}

func (r *ControllerListVolumesRequest) Validate() error {
	if r.MaxEntries < 0 {
		return errors.New("MaxEntries cannot be negative")
	}
	return nil
}

type ControllerListVolumesResponse struct {
	Entries   []*ListVolumesResponse_Entry
	NextToken string
}

func NewListVolumesResponse(resp *csipbv1.ListVolumesResponse) *ControllerListVolumesResponse {
	if resp == nil {
		return &ControllerListVolumesResponse{}
	}
	entries := []*ListVolumesResponse_Entry{}
	if resp.Entries != nil {
		for _, entry := range resp.Entries {
			vol := entry.GetVolume()
			status := entry.GetStatus()
			entries = append(entries, &ListVolumesResponse_Entry{
				Volume: &Volume{
					CapacityBytes:      vol.CapacityBytes,
					ExternalVolumeID:   vol.VolumeId,
					VolumeContext:      vol.VolumeContext,
					ContentSource:      newVolumeContentSource(vol.ContentSource),
					AccessibleTopology: newTopologies(vol.AccessibleTopology),
				},
				Status: &ListVolumesResponse_VolumeStatus{
					PublishedNodeIds: status.GetPublishedNodeIds(),
					VolumeCondition: &VolumeCondition{
						Abnormal: status.GetVolumeCondition().GetAbnormal(),
						Message:  status.GetVolumeCondition().GetMessage(),
					},
				},
			})
		}
	}
	return &ControllerListVolumesResponse{
		Entries:   entries,
		NextToken: resp.NextToken,
	}
}

type ListVolumesResponse_Entry struct {
	Volume *Volume
	Status *ListVolumesResponse_VolumeStatus
}

type ListVolumesResponse_VolumeStatus struct {
	PublishedNodeIds []string
	VolumeCondition  *VolumeCondition
}

type VolumeCondition struct {
	Abnormal bool
	Message  string
}

type ControllerCreateSnapshotRequest struct {
	VolumeID   string
	Name       string
	Secrets    structs.CSISecrets
	Parameters map[string]string
}

func (r *ControllerCreateSnapshotRequest) ToCSIRepresentation() *csipbv1.CreateSnapshotRequest {
	return &csipbv1.CreateSnapshotRequest{
		SourceVolumeId: r.VolumeID,
		Name:           r.Name,
		Secrets:        r.Secrets,
		Parameters:     r.Parameters,
	}
}

func (r *ControllerCreateSnapshotRequest) Validate() error {
	if r.VolumeID == "" {
		return errors.New("missing VolumeID")
	}
	if r.Name == "" {
		return errors.New("missing Name")
	}
	return nil
}

type ControllerCreateSnapshotResponse struct {
	Snapshot *Snapshot
}

type Snapshot struct {
	ID             string
	SourceVolumeID string
	SizeBytes      int64
	CreateTime     int64
	IsReady        bool
}

type ControllerDeleteSnapshotRequest struct {
	SnapshotID string
	Secrets    structs.CSISecrets
}

func (r *ControllerDeleteSnapshotRequest) ToCSIRepresentation() *csipbv1.DeleteSnapshotRequest {
	return &csipbv1.DeleteSnapshotRequest{
		SnapshotId: r.SnapshotID,
		Secrets:    r.Secrets,
	}
}

func (r *ControllerDeleteSnapshotRequest) Validate() error {
	if r.SnapshotID == "" {
		return errors.New("missing SnapshotID")
	}
	return nil
}

type ControllerListSnapshotsRequest struct {
	MaxEntries    int32
	StartingToken string
	Secrets       structs.CSISecrets
}

func (r *ControllerListSnapshotsRequest) ToCSIRepresentation() *csipbv1.ListSnapshotsRequest {
	return &csipbv1.ListSnapshotsRequest{
		MaxEntries:    r.MaxEntries,
		StartingToken: r.StartingToken,
		Secrets:       r.Secrets,
	}
}

func (r *ControllerListSnapshotsRequest) Validate() error {
	if r.MaxEntries < 0 {
		return errors.New("MaxEntries cannot be negative")
	}
	return nil
}

func NewListSnapshotsResponse(resp *csipbv1.ListSnapshotsResponse) *ControllerListSnapshotsResponse {
	if resp == nil {
		return &ControllerListSnapshotsResponse{}
	}
	entries := []*ListSnapshotsResponse_Entry{}
	if resp.Entries != nil {
		for _, entry := range resp.Entries {
			snap := entry.GetSnapshot()
			entries = append(entries, &ListSnapshotsResponse_Entry{
				Snapshot: &Snapshot{
					SizeBytes:      snap.GetSizeBytes(),
					ID:             snap.GetSnapshotId(),
					SourceVolumeID: snap.GetSourceVolumeId(),
					CreateTime:     snap.GetCreationTime().GetSeconds(),
					IsReady:        snap.GetReadyToUse(),
				},
			})
		}
	}
	return &ControllerListSnapshotsResponse{
		Entries:   entries,
		NextToken: resp.NextToken,
	}
}

type ControllerListSnapshotsResponse struct {
	Entries   []*ListSnapshotsResponse_Entry
	NextToken string
}

type ListSnapshotsResponse_Entry struct {
	Snapshot *Snapshot
}

type NodeCapabilitySet struct {
	HasStageUnstageVolume bool
	HasGetVolumeStats     bool
	HasExpandVolume       bool
	HasVolumeCondition    bool
}

func NewNodeCapabilitySet(resp *csipbv1.NodeGetCapabilitiesResponse) *NodeCapabilitySet {
	cs := &NodeCapabilitySet{}
	pluginCapabilities := resp.GetCapabilities()
	for _, pcap := range pluginCapabilities {
		if c := pcap.GetRpc(); c != nil {
			switch c.Type {
			case csipbv1.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME:
				cs.HasStageUnstageVolume = true
			case csipbv1.NodeServiceCapability_RPC_GET_VOLUME_STATS:
				cs.HasGetVolumeStats = true
			case csipbv1.NodeServiceCapability_RPC_EXPAND_VOLUME:
				cs.HasExpandVolume = true
			case csipbv1.NodeServiceCapability_RPC_VOLUME_CONDITION:
				cs.HasVolumeCondition = true
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

func VolumeCapabilityFromStructs(sAccessType structs.CSIVolumeAttachmentMode, sAccessMode structs.CSIVolumeAccessMode, sMountOptions *structs.CSIMountOptions) (*VolumeCapability, error) {
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
		return nil, fmt.Errorf("unknown volume attachment mode: %s", sAccessType)
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
		return nil, fmt.Errorf("unknown volume access mode: %v", sAccessMode)
	}

	return &VolumeCapability{
		AccessType:  accessType,
		AccessMode:  accessMode,
		MountVolume: sMountOptions,
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

type CapacityRange struct {
	RequiredBytes int64
	LimitBytes    int64
}

func (c *CapacityRange) ToCSIRepresentation() *csipbv1.CapacityRange {
	if c == nil {
		return nil
	}
	return &csipbv1.CapacityRange{
		RequiredBytes: c.RequiredBytes,
		LimitBytes:    c.LimitBytes,
	}
}

type NodeExpandVolumeRequest struct {
	ExternalVolumeID string
	RequiredBytes    int64
	LimitBytes       int64
	TargetPath       string
	StagingPath      string
	Capability       *VolumeCapability
}

func (r *NodeExpandVolumeRequest) ToCSIRepresentation() *csipbv1.NodeExpandVolumeRequest {
	if r == nil {
		return nil
	}
	return &csipbv1.NodeExpandVolumeRequest{
		VolumeId:   r.ExternalVolumeID,
		VolumePath: r.TargetPath,
		CapacityRange: &csipbv1.CapacityRange{
			RequiredBytes: r.RequiredBytes,
			LimitBytes:    r.LimitBytes,
		},
		StagingTargetPath: r.StagingPath,
		VolumeCapability:  r.Capability.ToCSIRepresentation(),
	}
}

type NodeExpandVolumeResponse struct {
	CapacityBytes int64
}
