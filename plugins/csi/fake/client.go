// fake is a package that includes fake implementations of public interfaces
// from the CSI package for testing.
package fake

import (
	"context"
	"errors"
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

	NextPluginGetInfoResponse string
	NextPluginGetInfoErr      error
	PluginGetInfoCallCount    int64

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
func (c *Client) PluginGetInfo(ctx context.Context) (string, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	c.PluginGetInfoCallCount++

	return c.NextPluginGetInfoResponse, c.NextPluginGetInfoErr
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
func (c *Client) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	c.ControllerPublishVolumeCallCount++

	return c.NextControllerPublishVolumeResponse, c.NextControllerPublishVolumeErr
}

// ControllerUnpublishVolume is used to attach a remote volume to a node
func (c *Client) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	c.ControllerUnpublishVolumeCallCount++

	return c.NextControllerUnpublishVolumeResponse, c.NextControllerUnpublishVolumeErr
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
func (c *Client) NodeStageVolume(ctx context.Context, volumeID string, publishContext map[string]string, stagingTargetPath string, capabilities *csi.VolumeCapability, opts ...grpc.CallOption) error {
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
	return nil
}
