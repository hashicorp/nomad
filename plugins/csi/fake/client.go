// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// fake is a package that includes fake implementations of public interfaces
// from the CSI package for testing.
package fake

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"

	"google.golang.org/grpc"

	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/csi"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

var _ csi.CSIPlugin = NewClient()

// Client is a mock implementation of the csi.CSIPlugin interface for use in testing
// external components
type Client struct {
	lock   sync.RWMutex
	counts map[string]int

	NextPluginInfoResponse *base.PluginInfoResponse
	NextPluginInfoErr      error

	NextPluginProbeResponse bool
	NextPluginProbeErr      error

	NextPluginGetInfoNameResponse    string
	NextPluginGetInfoVersionResponse string
	NextPluginGetInfoErr             error

	NextPluginGetCapabilitiesResponse *csi.PluginCapabilitySet
	NextPluginGetCapabilitiesErr      error

	NextControllerGetCapabilitiesResponse *csi.ControllerCapabilitySet
	NextControllerGetCapabilitiesErr      error

	NextControllerPublishVolumeResponse *csi.ControllerPublishVolumeResponse
	NextControllerPublishVolumeErr      error

	NextControllerUnpublishVolumeResponse *csi.ControllerUnpublishVolumeResponse
	NextControllerUnpublishVolumeErr      error

	NextControllerCreateVolumeResponse *csi.ControllerCreateVolumeResponse
	NextControllerCreateVolumeErr      error

	NextControllerDeleteVolumeErr error

	NextControllerListVolumesResponse *csi.ControllerListVolumesResponse
	NextControllerListVolumesErr      error

	NextControllerValidateVolumeErr error

	NextControllerCreateSnapshotResponse *csi.ControllerCreateSnapshotResponse
	NextControllerCreateSnapshotErr      error

	NextControllerDeleteSnapshotErr error

	NextControllerListSnapshotsResponse *csi.ControllerListSnapshotsResponse
	NextControllerListSnapshotsErr      error

	NextControllerExpandVolumeResponse *csi.ControllerExpandVolumeResponse
	NextControllerExpandVolumeErr      error

	NextNodeGetCapabilitiesResponse *csi.NodeCapabilitySet
	NextNodeGetCapabilitiesErr      error

	NextNodeGetInfoResponse *csi.NodeGetInfoResponse
	NextNodeGetInfoErr      error

	NextNodeStageVolumeErr error

	NextNodeUnstageVolumeErr error

	PrevVolumeCapability     *csi.VolumeCapability
	NextNodePublishVolumeErr error

	NextNodeUnpublishVolumeErr error

	NextNodeExpandVolumeResponse *csi.NodeExpandVolumeResponse
	NextNodeExpandVolumeErr      error
}

func NewClient() *Client {
	return &Client{
		counts: map[string]int{},
	}
}

// Counts returns a copy of the count tracking map
func (c *Client) Counts() map[string]int {
	c.lock.Lock()
	defer c.lock.Unlock()
	return maps.Clone(c.counts)
}

// Reset clears the RPC count tracking
func (c *Client) Reset() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.counts = map[string]int{}
}

// PluginInfo describes the type and version of a plugin.
func (c *Client) PluginInfo() (*base.PluginInfoResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.counts["PluginInfo"]++
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
	c.lock.Lock()
	defer c.lock.Unlock()

	c.counts["PluginProbe"]++
	return c.NextPluginProbeResponse, c.NextPluginProbeErr
}

// PluginGetInfo is used to return semantic data about the plugin.
// Response:
//   - string: name, the name of the plugin in domain notation format.
func (c *Client) PluginGetInfo(ctx context.Context) (string, string, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.counts["PluginGetInfo"]++
	return c.NextPluginGetInfoNameResponse, c.NextPluginGetInfoVersionResponse, c.NextPluginGetInfoErr
}

// PluginGetCapabilities is used to return the available capabilities from the
// identity service. This currently only looks for the CONTROLLER_SERVICE and
// Accessible Topology Support
func (c *Client) PluginGetCapabilities(ctx context.Context) (*csi.PluginCapabilitySet, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.counts["PluginGetCapabilities"]++
	return c.NextPluginGetCapabilitiesResponse, c.NextPluginGetCapabilitiesErr
}

func (c *Client) ControllerGetCapabilities(ctx context.Context) (*csi.ControllerCapabilitySet, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.counts["ControllerGetCapabilities"]++
	return c.NextControllerGetCapabilitiesResponse, c.NextControllerGetCapabilitiesErr
}

// ControllerPublishVolume is used to attach a remote volume to a node
func (c *Client) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest, opts ...grpc.CallOption) (*csi.ControllerPublishVolumeResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.counts["ControllerPublishVolume"]++
	return c.NextControllerPublishVolumeResponse, c.NextControllerPublishVolumeErr
}

// ControllerUnpublishVolume is used to attach a remote volume to a node
func (c *Client) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest, opts ...grpc.CallOption) (*csi.ControllerUnpublishVolumeResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.counts["ControllerUnpublishVolume"]++
	return c.NextControllerUnpublishVolumeResponse, c.NextControllerUnpublishVolumeErr
}

func (c *Client) ControllerValidateCapabilities(ctx context.Context, req *csi.ControllerValidateVolumeRequest, opts ...grpc.CallOption) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.counts["ControllerValidateVolume"]++
	return c.NextControllerValidateVolumeErr
}

func (c *Client) ControllerCreateVolume(ctx context.Context, in *csi.ControllerCreateVolumeRequest, opts ...grpc.CallOption) (*csi.ControllerCreateVolumeResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.counts["ControllerCreateVolume"]++
	return c.NextControllerCreateVolumeResponse, c.NextControllerCreateVolumeErr
}

func (c *Client) ControllerDeleteVolume(ctx context.Context, req *csi.ControllerDeleteVolumeRequest, opts ...grpc.CallOption) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.counts["ControllerDeleteVolume"]++
	return c.NextControllerDeleteVolumeErr
}

func (c *Client) ControllerListVolumes(ctx context.Context, req *csi.ControllerListVolumesRequest, opts ...grpc.CallOption) (*csi.ControllerListVolumesResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.counts["ControllerListVolumes"]++
	return c.NextControllerListVolumesResponse, c.NextControllerListVolumesErr
}

func (c *Client) ControllerCreateSnapshot(ctx context.Context, req *csi.ControllerCreateSnapshotRequest, opts ...grpc.CallOption) (*csi.ControllerCreateSnapshotResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.counts["ControllerCreateSnapshot"]++
	return c.NextControllerCreateSnapshotResponse, c.NextControllerCreateSnapshotErr
}

func (c *Client) ControllerDeleteSnapshot(ctx context.Context, req *csi.ControllerDeleteSnapshotRequest, opts ...grpc.CallOption) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.counts["ControllerDeleteSnapshot"]++
	return c.NextControllerDeleteSnapshotErr
}

func (c *Client) ControllerListSnapshots(ctx context.Context, req *csi.ControllerListSnapshotsRequest, opts ...grpc.CallOption) (*csi.ControllerListSnapshotsResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.counts["ControllerListSnapshots"]++
	return c.NextControllerListSnapshotsResponse, c.NextControllerListSnapshotsErr
}

func (c *Client) ControllerExpandVolume(ctx context.Context, in *csi.ControllerExpandVolumeRequest, opts ...grpc.CallOption) (*csi.ControllerExpandVolumeResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.counts["ControllerExpandVolume"]++
	return c.NextControllerExpandVolumeResponse, c.NextControllerExpandVolumeErr
}

func (c *Client) NodeGetCapabilities(ctx context.Context) (*csi.NodeCapabilitySet, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.counts["NodeGetCapabilities"]++
	return c.NextNodeGetCapabilitiesResponse, c.NextNodeGetCapabilitiesErr
}

// NodeGetInfo is used to return semantic data about the current node in
// respect to the SP.
func (c *Client) NodeGetInfo(ctx context.Context) (*csi.NodeGetInfoResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.counts["NodeGetInfo"]++
	return c.NextNodeGetInfoResponse, c.NextNodeGetInfoErr
}

// NodeStageVolume is used when a plugin has the STAGE_UNSTAGE volume capability
// to prepare a volume for usage on a host. If err == nil, the response should
// be assumed to be successful.
func (c *Client) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest, opts ...grpc.CallOption) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.counts["NodeStageVolume"]++
	return c.NextNodeStageVolumeErr
}

// NodeUnstageVolume is used when a plugin has the STAGE_UNSTAGE volume capability
// to undo the work performed by NodeStageVolume. If a volume has been staged,
// this RPC must be called before freeing the volume.
//
// If err == nil, the response should be assumed to be successful.
func (c *Client) NodeUnstageVolume(ctx context.Context, volumeID string, stagingTargetPath string, opts ...grpc.CallOption) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.counts["NodeUnstageVolume"]++
	return c.NextNodeUnstageVolumeErr
}

func (c *Client) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest, opts ...grpc.CallOption) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.PrevVolumeCapability = req.VolumeCapability
	c.counts["NodePublishVolume"]++
	return c.NextNodePublishVolumeErr
}

func (c *Client) NodeUnpublishVolume(ctx context.Context, volumeID, targetPath string, opts ...grpc.CallOption) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.counts["NodeUnpublishVolume"]++
	return c.NextNodeUnpublishVolumeErr
}

func (c *Client) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest, opts ...grpc.CallOption) (*csi.NodeExpandVolumeResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.counts["NodeExpandVolume"]++
	return c.NextNodeExpandVolumeResponse, c.NextNodeExpandVolumeErr
}

// Close the client and ensure any connections are cleaned up.
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

	c.NextControllerExpandVolumeResponse = nil
	c.NextControllerExpandVolumeErr = fmt.Errorf("closed client")

	c.NextControllerValidateVolumeErr = fmt.Errorf("closed client")

	c.NextNodeGetCapabilitiesResponse = nil
	c.NextNodeGetCapabilitiesErr = fmt.Errorf("closed client")

	c.NextNodeGetInfoResponse = nil
	c.NextNodeGetInfoErr = fmt.Errorf("closed client")

	c.NextNodeStageVolumeErr = fmt.Errorf("closed client")

	c.NextNodeUnstageVolumeErr = fmt.Errorf("closed client")

	c.NextNodePublishVolumeErr = fmt.Errorf("closed client")

	c.NextNodeUnpublishVolumeErr = fmt.Errorf("closed client")

	c.NextNodeExpandVolumeResponse = nil
	c.NextNodeExpandVolumeErr = fmt.Errorf("closed client")

	return nil
}
