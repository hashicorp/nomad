package csi

import (
	"context"
	"fmt"
	"net"
	"time"

	csipbv1 "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/grpc-middleware/logging"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"google.golang.org/grpc"
)

type NodeGetInfoResponse struct {
	NodeID             string
	MaxVolumes         int64
	AccessibleTopology *Topology
}

// Topology is a map of topological domains to topological segments.
// A topological domain is a sub-division of a cluster, like "region",
// "zone", "rack", etc.
//
// According to CSI, there are a few requirements for the keys within this map:
// - Valid keys have two segments: an OPTIONAL prefix and name, separated
//   by a slash (/), for example: "com.company.example/zone".
// - The key name segment is REQUIRED. The prefix is OPTIONAL.
// - The key name MUST be 63 characters or less, begin and end with an
//   alphanumeric character ([a-z0-9A-Z]), and contain only dashes (-),
//   underscores (_), dots (.), or alphanumerics in between, for example
//   "zone".
// - The key prefix MUST be 63 characters or less, begin and end with a
//   lower-case alphanumeric character ([a-z0-9]), contain only
//   dashes (-), dots (.), or lower-case alphanumerics in between, and
//   follow domain name notation format
//   (https://tools.ietf.org/html/rfc1035#section-2.3.1).
// - The key prefix SHOULD include the plugin's host company name and/or
//   the plugin name, to minimize the possibility of collisions with keys
//   from other plugins.
// - If a key prefix is specified, it MUST be identical across all
//   topology keys returned by the SP (across all RPCs).
// - Keys MUST be case-insensitive. Meaning the keys "Zone" and "zone"
//   MUST not both exist.
// - Each value (topological segment) MUST contain 1 or more strings.
// - Each string MUST be 63 characters or less and begin and end with an
//   alphanumeric character with '-', '_', '.', or alphanumerics in
//   between.
type Topology struct {
	Segments map[string]string
}

// CSIControllerClient defines the minimal CSI Controller Plugin interface used
// by nomad to simplify the interface required for testing.
type CSIControllerClient interface {
	ControllerGetCapabilities(ctx context.Context, in *csipbv1.ControllerGetCapabilitiesRequest, opts ...grpc.CallOption) (*csipbv1.ControllerGetCapabilitiesResponse, error)
	ControllerPublishVolume(ctx context.Context, in *csipbv1.ControllerPublishVolumeRequest, opts ...grpc.CallOption) (*csipbv1.ControllerPublishVolumeResponse, error)
	ControllerUnpublishVolume(ctx context.Context, in *csipbv1.ControllerUnpublishVolumeRequest, opts ...grpc.CallOption) (*csipbv1.ControllerUnpublishVolumeResponse, error)
	ValidateVolumeCapabilities(ctx context.Context, in *csipbv1.ValidateVolumeCapabilitiesRequest, opts ...grpc.CallOption) (*csipbv1.ValidateVolumeCapabilitiesResponse, error)
}

// CSINodeClient defines the minimal CSI Node Plugin interface used
// by nomad to simplify the interface required for testing.
type CSINodeClient interface {
	NodeGetCapabilities(ctx context.Context, in *csipbv1.NodeGetCapabilitiesRequest, opts ...grpc.CallOption) (*csipbv1.NodeGetCapabilitiesResponse, error)
	NodeGetInfo(ctx context.Context, in *csipbv1.NodeGetInfoRequest, opts ...grpc.CallOption) (*csipbv1.NodeGetInfoResponse, error)
	NodeStageVolume(ctx context.Context, in *csipbv1.NodeStageVolumeRequest, opts ...grpc.CallOption) (*csipbv1.NodeStageVolumeResponse, error)
	NodeUnstageVolume(ctx context.Context, in *csipbv1.NodeUnstageVolumeRequest, opts ...grpc.CallOption) (*csipbv1.NodeUnstageVolumeResponse, error)
	NodePublishVolume(ctx context.Context, in *csipbv1.NodePublishVolumeRequest, opts ...grpc.CallOption) (*csipbv1.NodePublishVolumeResponse, error)
	NodeUnpublishVolume(ctx context.Context, in *csipbv1.NodeUnpublishVolumeRequest, opts ...grpc.CallOption) (*csipbv1.NodeUnpublishVolumeResponse, error)
}

type client struct {
	conn             *grpc.ClientConn
	identityClient   csipbv1.IdentityClient
	controllerClient CSIControllerClient
	nodeClient       CSINodeClient
}

func (c *client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func NewClient(addr string, logger hclog.Logger) (CSIPlugin, error) {
	if addr == "" {
		return nil, fmt.Errorf("address is empty")
	}

	conn, err := newGrpcConn(addr, logger)
	if err != nil {
		return nil, err
	}

	return &client{
		conn:             conn,
		identityClient:   csipbv1.NewIdentityClient(conn),
		controllerClient: csipbv1.NewControllerClient(conn),
		nodeClient:       csipbv1.NewNodeClient(conn),
	}, nil
}

func newGrpcConn(addr string, logger hclog.Logger) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(
		addr,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(logging.UnaryClientInterceptor(logger)),
		grpc.WithStreamInterceptor(logging.StreamClientInterceptor(logger)),
		grpc.WithDialer(func(target string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", target, timeout)
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to open grpc connection to addr: %s, err: %v", addr, err)
	}

	return conn, nil
}

// PluginInfo describes the type and version of a plugin as required by the nomad
// base.BasePlugin interface.
func (c *client) PluginInfo() (*base.PluginInfoResponse, error) {
	name, err := c.PluginGetInfo(context.TODO())
	if err != nil {
		return nil, err
	}

	return &base.PluginInfoResponse{
		Type:              "csi",
		PluginApiVersions: []string{"1.0.0"}, // TODO: fingerprint csi version
		PluginVersion:     "1.0.0",           // TODO: get plugin version from somewhere?!
		Name:              name,
	}, nil
}

// ConfigSchema returns the schema for parsing the plugins configuration as
// required by the base.BasePlugin interface. It will always return nil.
func (c *client) ConfigSchema() (*hclspec.Spec, error) {
	return nil, nil
}

// SetConfig is used to set the configuration by passing a MessagePack
// encoding of it.
func (c *client) SetConfig(_ *base.Config) error {
	return fmt.Errorf("unsupported")
}

func (c *client) PluginProbe(ctx context.Context) (bool, error) {
	req, err := c.identityClient.Probe(ctx, &csipbv1.ProbeRequest{})
	if err != nil {
		return false, err
	}

	wrapper := req.GetReady()

	// wrapper.GetValue() protects against wrapper being `nil`, and returns false.
	ready := wrapper.GetValue()

	if wrapper == nil {
		// If the plugin returns a nil value for ready, then it should be
		// interpreted as the plugin is ready for compatibility with plugins that
		// do not do health checks.
		ready = true
	}

	return ready, nil
}

func (c *client) PluginGetInfo(ctx context.Context) (string, error) {
	if c == nil {
		return "", fmt.Errorf("Client not initialized")
	}
	if c.identityClient == nil {
		return "", fmt.Errorf("Client not initialized")
	}

	req, err := c.identityClient.GetPluginInfo(ctx, &csipbv1.GetPluginInfoRequest{})
	if err != nil {
		return "", err
	}

	name := req.GetName()
	if name == "" {
		return "", fmt.Errorf("PluginGetInfo: plugin returned empty name field")
	}

	return name, nil
}

func (c *client) PluginGetCapabilities(ctx context.Context) (*PluginCapabilitySet, error) {
	if c == nil {
		return nil, fmt.Errorf("Client not initialized")
	}
	if c.identityClient == nil {
		return nil, fmt.Errorf("Client not initialized")
	}

	resp, err := c.identityClient.GetPluginCapabilities(ctx, &csipbv1.GetPluginCapabilitiesRequest{})
	if err != nil {
		return nil, err
	}

	return NewPluginCapabilitySet(resp), nil
}

//
// Controller Endpoints
//

func (c *client) ControllerGetCapabilities(ctx context.Context) (*ControllerCapabilitySet, error) {
	if c == nil {
		return nil, fmt.Errorf("Client not initialized")
	}
	if c.controllerClient == nil {
		return nil, fmt.Errorf("controllerClient not initialized")
	}

	resp, err := c.controllerClient.ControllerGetCapabilities(ctx, &csipbv1.ControllerGetCapabilitiesRequest{})
	if err != nil {
		return nil, err
	}

	return NewControllerCapabilitySet(resp), nil
}

func (c *client) ControllerPublishVolume(ctx context.Context, req *ControllerPublishVolumeRequest) (*ControllerPublishVolumeResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("Client not initialized")
	}
	if c.controllerClient == nil {
		return nil, fmt.Errorf("controllerClient not initialized")
	}

	err := req.Validate()
	if err != nil {
		return nil, err
	}

	pbrequest := req.ToCSIRepresentation()
	resp, err := c.controllerClient.ControllerPublishVolume(ctx, pbrequest)
	if err != nil {
		return nil, err
	}

	return &ControllerPublishVolumeResponse{
		PublishContext: helper.CopyMapStringString(resp.PublishContext),
	}, nil
}

func (c *client) ControllerUnpublishVolume(ctx context.Context, req *ControllerUnpublishVolumeRequest) (*ControllerUnpublishVolumeResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("Client not initialized")
	}
	if c.controllerClient == nil {
		return nil, fmt.Errorf("controllerClient not initialized")
	}
	err := req.Validate()
	if err != nil {
		return nil, err
	}

	upbrequest := req.ToCSIRepresentation()
	_, err = c.controllerClient.ControllerUnpublishVolume(ctx, upbrequest)
	if err != nil {
		return nil, err
	}

	return &ControllerUnpublishVolumeResponse{}, nil
}

//
// Node Endpoints
//

func (c *client) NodeGetCapabilities(ctx context.Context) (*NodeCapabilitySet, error) {
	if c == nil {
		return nil, fmt.Errorf("Client not initialized")
	}
	if c.nodeClient == nil {
		return nil, fmt.Errorf("Client not initialized")
	}

	resp, err := c.nodeClient.NodeGetCapabilities(ctx, &csipbv1.NodeGetCapabilitiesRequest{})
	if err != nil {
		return nil, err
	}

	return NewNodeCapabilitySet(resp), nil
}

func (c *client) NodeGetInfo(ctx context.Context) (*NodeGetInfoResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("Client not initialized")
	}
	if c.nodeClient == nil {
		return nil, fmt.Errorf("Client not initialized")
	}

	result := &NodeGetInfoResponse{}

	resp, err := c.nodeClient.NodeGetInfo(ctx, &csipbv1.NodeGetInfoRequest{})
	if err != nil {
		return nil, err
	}

	if resp.GetNodeId() == "" {
		return nil, fmt.Errorf("plugin failed to return nodeid")
	}

	result.NodeID = resp.GetNodeId()
	result.MaxVolumes = resp.GetMaxVolumesPerNode()

	return result, nil
}

func (c *client) NodeStageVolume(ctx context.Context, volumeID string, publishContext map[string]string, stagingTargetPath string, capabilities *VolumeCapability, opts ...grpc.CallOption) error {
	if c == nil {
		return fmt.Errorf("Client not initialized")
	}
	if c.nodeClient == nil {
		return fmt.Errorf("Client not initialized")
	}

	// These errors should not be returned during production use but exist as aids
	// during Nomad Development
	if volumeID == "" {
		return fmt.Errorf("missing volumeID")
	}
	if stagingTargetPath == "" {
		return fmt.Errorf("missing stagingTargetPath")
	}

	req := &csipbv1.NodeStageVolumeRequest{
		VolumeId:          volumeID,
		PublishContext:    publishContext,
		StagingTargetPath: stagingTargetPath,
		VolumeCapability:  capabilities.ToCSIRepresentation(),
	}

	// NodeStageVolume's response contains no extra data. If err == nil, we were
	// successful.
	_, err := c.nodeClient.NodeStageVolume(ctx, req, opts...)
	return err
}

func (c *client) NodeUnstageVolume(ctx context.Context, volumeID string, stagingTargetPath string, opts ...grpc.CallOption) error {
	if c == nil {
		return fmt.Errorf("Client not initialized")
	}
	if c.nodeClient == nil {
		return fmt.Errorf("Client not initialized")
	}
	// These errors should not be returned during production use but exist as aids
	// during Nomad Development
	if volumeID == "" {
		return fmt.Errorf("missing volumeID")
	}
	if stagingTargetPath == "" {
		return fmt.Errorf("missing stagingTargetPath")
	}

	req := &csipbv1.NodeUnstageVolumeRequest{
		VolumeId:          volumeID,
		StagingTargetPath: stagingTargetPath,
	}

	// NodeUnstageVolume's response contains no extra data. If err == nil, we were
	// successful.
	_, err := c.nodeClient.NodeUnstageVolume(ctx, req, opts...)
	return err
}

func (c *client) NodePublishVolume(ctx context.Context, req *NodePublishVolumeRequest, opts ...grpc.CallOption) error {
	if c == nil {
		return fmt.Errorf("Client not initialized")
	}
	if c.nodeClient == nil {
		return fmt.Errorf("Client not initialized")
	}

	if err := req.Validate(); err != nil {
		return fmt.Errorf("validation error: %v", err)
	}

	// NodePublishVolume's response contains no extra data. If err == nil, we were
	// successful.
	_, err := c.nodeClient.NodePublishVolume(ctx, req.ToCSIRepresentation(), opts...)
	return err
}

func (c *client) NodeUnpublishVolume(ctx context.Context, volumeID, targetPath string, opts ...grpc.CallOption) error {
	if c == nil {
		return fmt.Errorf("Client not initialized")
	}
	if c.nodeClient == nil {
		return fmt.Errorf("Client not initialized")
	}

	if volumeID == "" {
		return fmt.Errorf("missing VolumeID")
	}

	if targetPath == "" {
		return fmt.Errorf("missing TargetPath")
	}

	req := &csipbv1.NodeUnpublishVolumeRequest{
		VolumeId:   volumeID,
		TargetPath: targetPath,
	}

	// NodeUnpublishVolume's response contains no extra data. If err == nil, we were
	// successful.
	_, err := c.nodeClient.NodeUnpublishVolume(ctx, req, opts...)
	return err
}
