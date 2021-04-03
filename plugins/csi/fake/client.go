// fake is a package that includes fake implementations of public interfaces
// from the CSI package for testing.
package fake

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/csi"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"google.golang.org/grpc"
)

var _ csi.CSIPlugin = &Client{}

// Client is a mock implementation of the csi.CSIPlugin interface for use in testing
// external components
type Client struct {
	Mu sync.RWMutex

	NextPluginInfoResponse *base.PluginInfoResponse
	NextPluginInfoErr      error
	PluginInfoCallCount    int64

	NextPluginProbeResponse bool
	NextPluginProbeErr      error
	PluginProbeCallCount    int64

	NextPluginGetInfoNameResponse    string
	NextPluginGetInfoVersionResponse string
	NextPluginGetInfoErr             error
	PluginGetInfoCallCount           int64

	NextPluginGetCapabilitiesResponse *csi.PluginCapabilitySet
	NextPluginGetCapabilitiesErr      error
	PluginGetCapabilitiesCallCount    int64

	NextControllerGetCapabilitiesResponse *csi.ControllerCapabilitySet
	NextControllerGetCapabilitiesErr      error
	ControllerGetCapabilitiesCallCount    int64

	NextControllerPublishVolumeResponse *csi.ControllerPublishVolumeResponse
	NextControllerPublishVolumeErr      error
	ControllerPublishVolumeCallCount    int64

	NextControllerUnpublishVolumeResponse *csi.ControllerUnpublishVolumeResponse
	NextControllerUnpublishVolumeErr      error
	ControllerUnpublishVolumeCallCount    int64

	NextControllerCreateVolumeResponse *csi.ControllerCreateVolumeResponse
	NextControllerCreateVolumeErr      error
	ControllerCreateVolumeCallCount    int64

	NextControllerDeleteVolumeErr   error
	ControllerDeleteVolumeCallCount int64

	NextControllerListVolumesResponse *csi.ControllerListVolumesResponse
	NextControllerListVolumesErr      error
	ControllerListVolumesCallCount    int64

	NextControllerValidateVolumeErr   error
	ControllerValidateVolumeCallCount int64

	NextControllerCreateSnapshotResponse *csi.ControllerCreateSnapshotResponse
	NextControllerCreateSnapshotErr      error
	ControllerCreateSnapshotCallCount    int64

	NextControllerDeleteSnapshotErr   error
	ControllerDeleteSnapshotCallCount int64

	NextControllerListSnapshotsResponse *csi.ControllerListSnapshotsResponse
	NextControllerListSnapshotsErr      error
	ControllerListSnapshotsCallCount    int64

	NextNodeGetCapabilitiesResponse *csi.NodeCapabilitySet
	NextNodeGetCapabilitiesErr      error
	NodeGetCapabilitiesCallCount    int64

	NextNodeGetInfoResponse *csi.NodeGetInfoResponse
	NextNodeGetInfoErr      error
	NodeGetInfoCallCount    int64

	NextNodeStageVolumeErr   error
	NodeStageVolumeCallCount int64

	NextNodeUnstageVolumeErr   error
	NodeUnstageVolumeCallCount int64

	PrevVolumeCapability       *csi.VolumeCapability
	NextNodePublishVolumeErr   error
	NodePublishVolumeCallCount int64

	NextNodeUnpublishVolumeErr   error
	NodeUnpublishVolumeCallCount int64
}

// PluginInfo describes the type and version of a plugin.
func (c *Client) PluginInfo() (*base.PluginInfoResponse, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	c.PluginInfoCallCount++

	return c.NextPluginInfoResponse, c.NextPluginInfoErr
}

// ConfigSchema returns the schema for parsing the plugins configuration.
func (c *Client) ConfigSchema() (*hclspec.Spec, error) {
	return nil, errors.New("Unsupported")
}

// SetConfig is used to set the configuration by passing a MessagePack
// encoding of it.
func (c *Client) SetConfig(a *base.Config) error {
	return errors.New("Unsupported")
}

// PluginProbe is used to verify that the plugin is in a healthy state
func (c *Client) PluginProbe(ctx context.Context) (bool, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	c.PluginProbeCallCount++

	return c.NextPluginProbeResponse, c.NextPluginProbeErr
}

// PluginGetInfo is used to return semantic data about the plugin.
// Response:
//  - string: name, the name of the plugin in domain notation format.
func (c *Client) PluginGetInfo(ctx context.Context) (string, string, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	c.PluginGetInfoCallCount++

	return c.NextPluginGetInfoNameResponse, c.NextPluginGetInfoVersionResponse, c.NextPluginGetInfoErr
}

// PluginGetCapabilities is used to return the available capabilities from the
// identity service. This currently only looks for the CONTROLLER_SERVICE and
// Accessible Topology Support
func (c *Client) PluginGetCapabilities(ctx context.Context) (*csi.PluginCapabilitySet, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	c.PluginGetCapabilitiesCallCount++

	return c.NextPluginGetCapabilitiesResponse, c.NextPluginGetCapabilitiesErr
}

func (c *Client) ControllerGetCapabilities(ctx context.Context) (*csi.ControllerCapabilitySet, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	c.ControllerGetCapabilitiesCallCount++

	return c.NextControllerGetCapabilitiesResponse, c.NextControllerGetCapabilitiesErr
}

// ControllerPublishVolume is used to attach a remote volume to a node
func (c *Client) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest, opts ...grpc.CallOption) (*csi.ControllerPublishVolumeResponse, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	c.ControllerPublishVolumeCallCount++

	return c.NextControllerPublishVolumeResponse, c.NextControllerPublishVolumeErr
}

// ControllerUnpublishVolume is used to attach a remote volume to a node
func (c *Client) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest, opts ...grpc.CallOption) (*csi.ControllerUnpublishVolumeResponse, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	c.ControllerUnpublishVolumeCallCount++

	return c.NextControllerUnpublishVolumeResponse, c.NextControllerUnpublishVolumeErr
}

func (c *Client) ControllerValidateCapabilities(ctx context.Context, req *csi.ControllerValidateVolumeRequest, opts ...grpc.CallOption) error {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	c.ControllerValidateVolumeCallCount++

	return c.NextControllerValidateVolumeErr
}

func (c *Client) ControllerCreateVolume(ctx context.Context, in *csi.ControllerCreateVolumeRequest, opts ...grpc.CallOption) (*csi.ControllerCreateVolumeResponse, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	c.ControllerCreateVolumeCallCount++
	return c.NextControllerCreateVolumeResponse, c.NextControllerCreateVolumeErr
}

func (c *Client) ControllerDeleteVolume(ctx context.Context, req *csi.ControllerDeleteVolumeRequest, opts ...grpc.CallOption) error {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	c.ControllerDeleteVolumeCallCount++
	return c.NextControllerDeleteVolumeErr
}

func (c *Client) ControllerListVolumes(ctx context.Context, req *csi.ControllerListVolumesRequest, opts ...grpc.CallOption) (*csi.ControllerListVolumesResponse, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	c.ControllerListVolumesCallCount++
	return c.NextControllerListVolumesResponse, c.NextControllerListVolumesErr
}

func (c *Client) ControllerCreateSnapshot(ctx context.Context, req *csi.ControllerCreateSnapshotRequest, opts ...grpc.CallOption) (*csi.ControllerCreateSnapshotResponse, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	c.ControllerCreateSnapshotCallCount++
	return c.NextControllerCreateSnapshotResponse, c.NextControllerCreateSnapshotErr
}

func (c *Client) ControllerDeleteSnapshot(ctx context.Context, req *csi.ControllerDeleteSnapshotRequest, opts ...grpc.CallOption) error {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	c.ControllerDeleteSnapshotCallCount++
	return c.NextControllerDeleteSnapshotErr
}

func (c *Client) ControllerListSnapshots(ctx context.Context, req *csi.ControllerListSnapshotsRequest, opts ...grpc.CallOption) (*csi.ControllerListSnapshotsResponse, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	c.ControllerListSnapshotsCallCount++
	return c.NextControllerListSnapshotsResponse, c.NextControllerListSnapshotsErr
}

func (c *Client) NodeGetCapabilities(ctx context.Context) (*csi.NodeCapabilitySet, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	c.NodeGetCapabilitiesCallCount++

	return c.NextNodeGetCapabilitiesResponse, c.NextNodeGetCapabilitiesErr
}

// NodeGetInfo is used to return semantic data about the current node in
// respect to the SP.
func (c *Client) NodeGetInfo(ctx context.Context) (*csi.NodeGetInfoResponse, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	c.NodeGetInfoCallCount++

	return c.NextNodeGetInfoResponse, c.NextNodeGetInfoErr
}

// NodeStageVolume is used when a plugin has the STAGE_UNSTAGE volume capability
// to prepare a volume for usage on a host. If err == nil, the response should
// be assumed to be successful.
func (c *Client) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest, opts ...grpc.CallOption) error {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	c.NodeStageVolumeCallCount++

	return c.NextNodeStageVolumeErr
}

// NodeUnstageVolume is used when a plugin has the STAGE_UNSTAGE volume capability
// to undo the work performed by NodeStageVolume. If a volume has been staged,
// this RPC must be called before freeing the volume.
//
// If err == nil, the response should be assumed to be successful.
func (c *Client) NodeUnstageVolume(ctx context.Context, volumeID string, stagingTargetPath string, opts ...grpc.CallOption) error {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	c.NodeUnstageVolumeCallCount++

	return c.NextNodeUnstageVolumeErr
}

func (c *Client) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest, opts ...grpc.CallOption) error {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	c.PrevVolumeCapability = req.VolumeCapability
	c.NodePublishVolumeCallCount++

	return c.NextNodePublishVolumeErr
}

func (c *Client) NodeUnpublishVolume(ctx context.Context, volumeID, targetPath string, opts ...grpc.CallOption) error {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	c.NodeUnpublishVolumeCallCount++

	return c.NextNodeUnpublishVolumeErr
}

// Shutdown the client and ensure any connections are cleaned up.
func (c *Client) Close() error {

	c.NextPluginInfoResponse = nil
	c.NextPluginInfoErr = fmt.Errorf("closed client")

	c.NextPluginProbeResponse = false
	c.NextPluginProbeErr = fmt.Errorf("closed client")

	c.NextPluginGetInfoNameResponse = ""
	c.NextPluginGetInfoVersionResponse = ""
	c.NextPluginGetInfoErr = fmt.Errorf("closed client")

	c.NextPluginGetCapabilitiesResponse = nil
	c.NextPluginGetCapabilitiesErr = fmt.Errorf("closed client")

	c.NextControllerGetCapabilitiesResponse = nil
	c.NextControllerGetCapabilitiesErr = fmt.Errorf("closed client")

	c.NextControllerPublishVolumeResponse = nil
	c.NextControllerPublishVolumeErr = fmt.Errorf("closed client")

	c.NextControllerUnpublishVolumeResponse = nil
	c.NextControllerUnpublishVolumeErr = fmt.Errorf("closed client")

	c.NextControllerValidateVolumeErr = fmt.Errorf("closed client")

	c.NextNodeGetCapabilitiesResponse = nil
	c.NextNodeGetCapabilitiesErr = fmt.Errorf("closed client")

	c.NextNodeGetInfoResponse = nil
	c.NextNodeGetInfoErr = fmt.Errorf("closed client")

	c.NextNodeStageVolumeErr = fmt.Errorf("closed client")

	c.NextNodeUnstageVolumeErr = fmt.Errorf("closed client")

	c.NextNodePublishVolumeErr = fmt.Errorf("closed client")

	c.NextNodeUnpublishVolumeErr = fmt.Errorf("closed client")

	return nil
}
