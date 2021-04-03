package testing

import (
	"context"
	"fmt"

	csipbv1 "github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
)

// IdentityClient is a CSI identity client used for testing
type IdentityClient struct {
	NextErr                error
	NextPluginInfo         *csipbv1.GetPluginInfoResponse
	NextPluginCapabilities *csipbv1.GetPluginCapabilitiesResponse
	NextPluginProbe        *csipbv1.ProbeResponse
}

// NewIdentityClient returns a new IdentityClient
func NewIdentityClient() *IdentityClient {
	return &IdentityClient{}
}

func (f *IdentityClient) Reset() {
	f.NextErr = nil
	f.NextPluginInfo = nil
	f.NextPluginCapabilities = nil
	f.NextPluginProbe = nil
}

// GetPluginInfo returns plugin info
func (f *IdentityClient) GetPluginInfo(ctx context.Context, in *csipbv1.GetPluginInfoRequest, opts ...grpc.CallOption) (*csipbv1.GetPluginInfoResponse, error) {
	return f.NextPluginInfo, f.NextErr
}

// GetPluginCapabilities implements csi method
func (f *IdentityClient) GetPluginCapabilities(ctx context.Context, in *csipbv1.GetPluginCapabilitiesRequest, opts ...grpc.CallOption) (*csipbv1.GetPluginCapabilitiesResponse, error) {
	return f.NextPluginCapabilities, f.NextErr
}

// Probe implements csi method
func (f *IdentityClient) Probe(ctx context.Context, in *csipbv1.ProbeRequest, opts ...grpc.CallOption) (*csipbv1.ProbeResponse, error) {
	return f.NextPluginProbe, f.NextErr
}

// ControllerClient is a CSI controller client used for testing
type ControllerClient struct {
	NextErr                                error
	NextCapabilitiesResponse               *csipbv1.ControllerGetCapabilitiesResponse
	NextPublishVolumeResponse              *csipbv1.ControllerPublishVolumeResponse
	NextUnpublishVolumeResponse            *csipbv1.ControllerUnpublishVolumeResponse
	NextValidateVolumeCapabilitiesResponse *csipbv1.ValidateVolumeCapabilitiesResponse
	NextCreateVolumeResponse               *csipbv1.CreateVolumeResponse
	NextDeleteVolumeResponse               *csipbv1.DeleteVolumeResponse
	NextListVolumesResponse                *csipbv1.ListVolumesResponse
	NextCreateSnapshotResponse             *csipbv1.CreateSnapshotResponse
	NextDeleteSnapshotResponse             *csipbv1.DeleteSnapshotResponse
	NextListSnapshotsResponse              *csipbv1.ListSnapshotsResponse
}

// NewControllerClient returns a new ControllerClient
func NewControllerClient() *ControllerClient {
	return &ControllerClient{}
}

func (f *ControllerClient) Reset() {
	f.NextErr = nil
	f.NextCapabilitiesResponse = nil
	f.NextPublishVolumeResponse = nil
	f.NextUnpublishVolumeResponse = nil
	f.NextValidateVolumeCapabilitiesResponse = nil
	f.NextCreateVolumeResponse = nil
	f.NextDeleteVolumeResponse = nil
	f.NextListVolumesResponse = nil
	f.NextCreateSnapshotResponse = nil
	f.NextDeleteSnapshotResponse = nil
	f.NextListSnapshotsResponse = nil
}

func (c *ControllerClient) ControllerGetCapabilities(ctx context.Context, in *csipbv1.ControllerGetCapabilitiesRequest, opts ...grpc.CallOption) (*csipbv1.ControllerGetCapabilitiesResponse, error) {
	return c.NextCapabilitiesResponse, c.NextErr
}

func (c *ControllerClient) ControllerPublishVolume(ctx context.Context, in *csipbv1.ControllerPublishVolumeRequest, opts ...grpc.CallOption) (*csipbv1.ControllerPublishVolumeResponse, error) {
	return c.NextPublishVolumeResponse, c.NextErr
}

func (c *ControllerClient) ControllerUnpublishVolume(ctx context.Context, in *csipbv1.ControllerUnpublishVolumeRequest, opts ...grpc.CallOption) (*csipbv1.ControllerUnpublishVolumeResponse, error) {
	return c.NextUnpublishVolumeResponse, c.NextErr
}

func (c *ControllerClient) ValidateVolumeCapabilities(ctx context.Context, in *csipbv1.ValidateVolumeCapabilitiesRequest, opts ...grpc.CallOption) (*csipbv1.ValidateVolumeCapabilitiesResponse, error) {
	return c.NextValidateVolumeCapabilitiesResponse, c.NextErr
}

func (c *ControllerClient) CreateVolume(ctx context.Context, in *csipbv1.CreateVolumeRequest, opts ...grpc.CallOption) (*csipbv1.CreateVolumeResponse, error) {
	if in.VolumeContentSource != nil {
		if in.VolumeContentSource.Type == nil || (in.VolumeContentSource.Type ==
			&csipbv1.VolumeContentSource_Volume{
				Volume: &csipbv1.VolumeContentSource_VolumeSource{VolumeId: ""},
			}) || (in.VolumeContentSource.Type ==
			&csipbv1.VolumeContentSource_Snapshot{
				Snapshot: &csipbv1.VolumeContentSource_SnapshotSource{SnapshotId: ""},
			}) {
			return nil, fmt.Errorf("empty content source should be nil")
		}
	}
	return c.NextCreateVolumeResponse, c.NextErr
}

func (c *ControllerClient) DeleteVolume(ctx context.Context, in *csipbv1.DeleteVolumeRequest, opts ...grpc.CallOption) (*csipbv1.DeleteVolumeResponse, error) {
	return c.NextDeleteVolumeResponse, c.NextErr
}

func (c *ControllerClient) ListVolumes(ctx context.Context, in *csipbv1.ListVolumesRequest, opts ...grpc.CallOption) (*csipbv1.ListVolumesResponse, error) {
	return c.NextListVolumesResponse, c.NextErr
}

func (c *ControllerClient) CreateSnapshot(ctx context.Context, in *csipbv1.CreateSnapshotRequest, opts ...grpc.CallOption) (*csipbv1.CreateSnapshotResponse, error) {
	return c.NextCreateSnapshotResponse, c.NextErr
}

func (c *ControllerClient) DeleteSnapshot(ctx context.Context, in *csipbv1.DeleteSnapshotRequest, opts ...grpc.CallOption) (*csipbv1.DeleteSnapshotResponse, error) {
	return c.NextDeleteSnapshotResponse, c.NextErr
}

func (c *ControllerClient) ListSnapshots(ctx context.Context, in *csipbv1.ListSnapshotsRequest, opts ...grpc.CallOption) (*csipbv1.ListSnapshotsResponse, error) {
	return c.NextListSnapshotsResponse, c.NextErr
}

// NodeClient is a CSI Node client used for testing
type NodeClient struct {
	NextErr                     error
	NextCapabilitiesResponse    *csipbv1.NodeGetCapabilitiesResponse
	NextGetInfoResponse         *csipbv1.NodeGetInfoResponse
	NextStageVolumeResponse     *csipbv1.NodeStageVolumeResponse
	NextUnstageVolumeResponse   *csipbv1.NodeUnstageVolumeResponse
	NextPublishVolumeResponse   *csipbv1.NodePublishVolumeResponse
	NextUnpublishVolumeResponse *csipbv1.NodeUnpublishVolumeResponse
}

// NewNodeClient returns a new stub NodeClient
func NewNodeClient() *NodeClient {
	return &NodeClient{}
}

func (f *NodeClient) Reset() {
	f.NextErr = nil
	f.NextCapabilitiesResponse = nil
	f.NextGetInfoResponse = nil
	f.NextStageVolumeResponse = nil
	f.NextUnstageVolumeResponse = nil
	f.NextPublishVolumeResponse = nil
	f.NextUnpublishVolumeResponse = nil
}

func (c *NodeClient) NodeGetCapabilities(ctx context.Context, in *csipbv1.NodeGetCapabilitiesRequest, opts ...grpc.CallOption) (*csipbv1.NodeGetCapabilitiesResponse, error) {
	return c.NextCapabilitiesResponse, c.NextErr
}

func (c *NodeClient) NodeGetInfo(ctx context.Context, in *csipbv1.NodeGetInfoRequest, opts ...grpc.CallOption) (*csipbv1.NodeGetInfoResponse, error) {
	return c.NextGetInfoResponse, c.NextErr
}

func (c *NodeClient) NodeStageVolume(ctx context.Context, in *csipbv1.NodeStageVolumeRequest, opts ...grpc.CallOption) (*csipbv1.NodeStageVolumeResponse, error) {
	return c.NextStageVolumeResponse, c.NextErr
}

func (c *NodeClient) NodeUnstageVolume(ctx context.Context, in *csipbv1.NodeUnstageVolumeRequest, opts ...grpc.CallOption) (*csipbv1.NodeUnstageVolumeResponse, error) {
	return c.NextUnstageVolumeResponse, c.NextErr
}

func (c *NodeClient) NodePublishVolume(ctx context.Context, in *csipbv1.NodePublishVolumeRequest, opts ...grpc.CallOption) (*csipbv1.NodePublishVolumeResponse, error) {
	return c.NextPublishVolumeResponse, c.NextErr
}

func (c *NodeClient) NodeUnpublishVolume(ctx context.Context, in *csipbv1.NodeUnpublishVolumeRequest, opts ...grpc.CallOption) (*csipbv1.NodeUnpublishVolumeResponse, error) {
	return c.NextUnpublishVolumeResponse, c.NextErr
}
