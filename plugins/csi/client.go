package csi

import (
	"context"
	"fmt"
	"math"
	"net"
	"time"

	csipbv1 "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/grpc-middleware/logging"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PluginTypeCSI implements the CSI plugin interface
const PluginTypeCSI = "csi"

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
	logger           hclog.Logger
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
		logger:           logger,
	}, nil
}

func newGrpcConn(addr string, logger hclog.Logger) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()
	conn, err := grpc.DialContext(
		ctx,
		addr,
		grpc.WithBlock(),
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
	// note: no grpc retries needed here, as this is called in
	// fingerprinting and will get retried by the caller.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	name, version, err := c.PluginGetInfo(ctx)
	if err != nil {
		return nil, err
	}

	return &base.PluginInfoResponse{
		Type:              PluginTypeCSI,     // note: this isn't a Nomad go-plugin type
		PluginApiVersions: []string{"1.0.0"}, // TODO(tgross): we want to fingerprint spec version, but this isn't included as a field from the plugins
		PluginVersion:     version,
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
	// note: no grpc retries should be done here
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

func (c *client) PluginGetInfo(ctx context.Context) (string, string, error) {
	if c == nil {
		return "", "", fmt.Errorf("Client not initialized")
	}
	if c.identityClient == nil {
		return "", "", fmt.Errorf("Client not initialized")
	}

	resp, err := c.identityClient.GetPluginInfo(ctx, &csipbv1.GetPluginInfoRequest{})
	if err != nil {
		return "", "", err
	}

	name := resp.GetName()
	if name == "" {
		return "", "", fmt.Errorf("PluginGetInfo: plugin returned empty name field")
	}
	version := resp.GetVendorVersion()

	return name, version, nil
}

func (c *client) PluginGetCapabilities(ctx context.Context) (*PluginCapabilitySet, error) {
	if c == nil {
		return nil, fmt.Errorf("Client not initialized")
	}
	if c.identityClient == nil {
		return nil, fmt.Errorf("Client not initialized")
	}

	// note: no grpc retries needed here, as this is called in
	// fingerprinting and will get retried by the caller
	resp, err := c.identityClient.GetPluginCapabilities(ctx,
		&csipbv1.GetPluginCapabilitiesRequest{})
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

	// note: no grpc retries needed here, as this is called in
	// fingerprinting and will get retried by the caller
	resp, err := c.controllerClient.ControllerGetCapabilities(ctx,
		&csipbv1.ControllerGetCapabilitiesRequest{})
	if err != nil {
		return nil, err
	}

	return NewControllerCapabilitySet(resp), nil
}

func (c *client) ControllerPublishVolume(ctx context.Context, req *ControllerPublishVolumeRequest, opts ...grpc.CallOption) (*ControllerPublishVolumeResponse, error) {
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
	resp, err := c.controllerClient.ControllerPublishVolume(ctx, pbrequest, opts...)
	if err != nil {
		code := status.Code(err)
		switch code {
		case codes.NotFound:
			err = fmt.Errorf("volume %q or node %q could not be found: %v",
				req.ExternalID, req.NodeID, err)
		case codes.AlreadyExists:
			err = fmt.Errorf(
				"volume %q is already published at node %q but with capabilities or a read_only setting incompatible with this request: %v",
				req.ExternalID, req.NodeID, err)
		case codes.ResourceExhausted:
			err = fmt.Errorf("node %q has reached the maximum allowable number of attached volumes: %v",
				req.NodeID, err)
		case codes.FailedPrecondition:
			err = fmt.Errorf("volume %q is already published on another node and does not have MULTI_NODE volume capability: %v",
				req.ExternalID, err)
		case codes.Internal:
			err = fmt.Errorf("controller plugin returned an internal error, check the plugin allocation logs for more information: %v", err)
		}
		return nil, err
	}

	return &ControllerPublishVolumeResponse{
		PublishContext: helper.CopyMapStringString(resp.PublishContext),
	}, nil
}

func (c *client) ControllerUnpublishVolume(ctx context.Context, req *ControllerUnpublishVolumeRequest, opts ...grpc.CallOption) (*ControllerUnpublishVolumeResponse, error) {
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
	_, err = c.controllerClient.ControllerUnpublishVolume(ctx, upbrequest, opts...)
	if err != nil {
		code := status.Code(err)
		switch code {
		case codes.NotFound:
			// we'll have validated the volume and node *should* exist at the
			// server, so if we get a not-found here it's because we've previously
			// checkpointed. we'll return an error so the caller can log it for
			// diagnostic purposes.
			err = fmt.Errorf("%w: volume %q or node %q could not be found: %v",
				structs.ErrCSIClientRPCIgnorable, req.ExternalID, req.NodeID, err)
		case codes.Internal:
			err = fmt.Errorf("controller plugin returned an internal error, check the plugin allocation logs for more information: %v", err)
		}
		return nil, err
	}

	return &ControllerUnpublishVolumeResponse{}, nil
}

func (c *client) ControllerValidateCapabilities(ctx context.Context, req *ControllerValidateVolumeRequest, opts ...grpc.CallOption) error {
	if c == nil {
		return fmt.Errorf("Client not initialized")
	}
	if c.controllerClient == nil {
		return fmt.Errorf("controllerClient not initialized")
	}

	if req.ExternalID == "" {
		return fmt.Errorf("missing volume ID")
	}

	if req.Capabilities == nil {
		return fmt.Errorf("missing Capabilities")
	}

	creq := req.ToCSIRepresentation()
	resp, err := c.controllerClient.ValidateVolumeCapabilities(ctx, creq, opts...)
	if err != nil {
		code := status.Code(err)
		switch code {
		case codes.NotFound:
			err = fmt.Errorf("volume %q could not be found: %v", req.ExternalID, err)
		case codes.Internal:
			err = fmt.Errorf("controller plugin returned an internal error, check the plugin allocation logs for more information: %v", err)
		}
		return err
	}

	if resp.Message != "" {
		// this should only ever be set if Confirmed isn't set, but
		// it's not a validation failure.
		c.logger.Debug(resp.Message)
	}

	// The protobuf accessors below safely handle nil pointers.
	// The CSI spec says we can only assert the plugin has
	// confirmed the volume capabilities, not that it hasn't
	// confirmed them, so if the field is nil we have to assume
	// the volume is ok.
	confirmedCaps := resp.GetConfirmed().GetVolumeCapabilities()
	if confirmedCaps != nil {
		for _, requestedCap := range creq.VolumeCapabilities {
			err := compareCapabilities(requestedCap, confirmedCaps)
			if err != nil {
				return fmt.Errorf("volume capability validation failed: %v", err)
			}
		}
	}

	return nil
}

// compareCapabilities returns an error if the 'got' capabilities does not
// contain the 'expected' capability
func compareCapabilities(expected *csipbv1.VolumeCapability, got []*csipbv1.VolumeCapability) error {
	var err multierror.Error
NEXT_CAP:
	for _, cap := range got {

		expectedMode := expected.GetAccessMode().GetMode()
		capMode := cap.GetAccessMode().GetMode()

		if expectedMode != capMode {
			multierror.Append(&err,
				fmt.Errorf("requested AccessMode %v, got %v", expectedMode, capMode))
			continue NEXT_CAP
		}

		// AccessType Block is an empty struct even if set, so the
		// only way to test for it is to check that the AccessType
		// isn't Mount.
		expectedMount := expected.GetMount()
		capMount := cap.GetMount()

		if expectedMount == nil {
			if capMount == nil {
				return nil
			}
			multierror.Append(&err, fmt.Errorf(
				"requested AccessType Block but got AccessType Mount"))
			continue NEXT_CAP
		}

		if capMount == nil {
			multierror.Append(&err, fmt.Errorf(
				"requested AccessType Mount but got AccessType Block"))
			continue NEXT_CAP
		}

		if expectedMount.FsType != capMount.FsType {
			multierror.Append(&err, fmt.Errorf(
				"requested AccessType mount filesystem type %v, got %v",
				expectedMount.FsType, capMount.FsType))
			continue NEXT_CAP
		}

		for _, expectedFlag := range expectedMount.MountFlags {
			var ok bool
			for _, flag := range capMount.MountFlags {
				if expectedFlag == flag {
					ok = true
					break
				}
			}
			if !ok {
				// mount flags can contain sensitive data, so we can't log details
				multierror.Append(&err, fmt.Errorf(
					"requested mount flags did not match available capabilities"))
				continue NEXT_CAP
			}
		}
		return nil
	}
	return err.ErrorOrNil()
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

	// note: no grpc retries needed here, as this is called in
	// fingerprinting and will get retried by the caller
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

	// note: no grpc retries needed here, as this is called in
	// fingerprinting and will get retried by the caller
	resp, err := c.nodeClient.NodeGetInfo(ctx, &csipbv1.NodeGetInfoRequest{})
	if err != nil {
		return nil, err
	}

	if resp.GetNodeId() == "" {
		return nil, fmt.Errorf("plugin failed to return nodeid")
	}

	result.NodeID = resp.GetNodeId()
	result.MaxVolumes = resp.GetMaxVolumesPerNode()
	if result.MaxVolumes == 0 {
		// set safe default so that scheduler ignores this constraint when not set
		result.MaxVolumes = math.MaxInt64
	}

	return result, nil
}

func (c *client) NodeStageVolume(ctx context.Context, req *NodeStageVolumeRequest, opts ...grpc.CallOption) error {
	if c == nil {
		return fmt.Errorf("Client not initialized")
	}
	if c.nodeClient == nil {
		return fmt.Errorf("Client not initialized")
	}
	err := req.Validate()
	if err != nil {
		return err
	}

	// NodeStageVolume's response contains no extra data. If err == nil, we were
	// successful.
	_, err = c.nodeClient.NodeStageVolume(ctx, req.ToCSIRepresentation(), opts...)
	if err != nil {
		code := status.Code(err)
		switch code {
		case codes.NotFound:
			err = fmt.Errorf("volume %q could not be found: %v", req.ExternalID, err)
		case codes.AlreadyExists:
			err = fmt.Errorf(
				"volume %q is already staged to %q but with incompatible capabilities for this request: %v",
				req.ExternalID, req.StagingTargetPath, err)
		case codes.FailedPrecondition:
			err = fmt.Errorf("volume %q is already published on another node and does not have MULTI_NODE volume capability: %v",
				req.ExternalID, err)
		case codes.Internal:
			err = fmt.Errorf("node plugin returned an internal error, check the plugin allocation logs for more information: %v", err)
		}
	}

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
	// during Nomad development
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
	if err != nil {
		code := status.Code(err)
		switch code {
		case codes.NotFound:
			err = fmt.Errorf("%w: volume %q could not be found: %v",
				structs.ErrCSIClientRPCIgnorable, volumeID, err)
		case codes.Internal:
			err = fmt.Errorf("node plugin returned an internal error, check the plugin allocation logs for more information: %v", err)
		}
	}

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
	if err != nil {
		code := status.Code(err)
		switch code {
		case codes.NotFound:
			err = fmt.Errorf("volume %q could not be found: %v", req.ExternalID, err)
		case codes.AlreadyExists:
			err = fmt.Errorf(
				"volume %q is already published at target path %q but with capabilities or a read_only setting incompatible with this request: %v",
				req.ExternalID, req.TargetPath, err)
		case codes.FailedPrecondition:
			err = fmt.Errorf("volume %q is already published on another node and does not have MULTI_NODE volume capability: %v",
				req.ExternalID, err)
		case codes.Internal:
			err = fmt.Errorf("node plugin returned an internal error, check the plugin allocation logs for more information: %v", err)
		}
	}
	return err
}

func (c *client) NodeUnpublishVolume(ctx context.Context, volumeID, targetPath string, opts ...grpc.CallOption) error {
	if c == nil {
		return fmt.Errorf("Client not initialized")
	}
	if c.nodeClient == nil {
		return fmt.Errorf("Client not initialized")
	}

	// These errors should not be returned during production use but exist as aids
	// during Nomad development
	if volumeID == "" {
		return fmt.Errorf("missing volumeID")
	}
	if targetPath == "" {
		return fmt.Errorf("missing targetPath")
	}

	req := &csipbv1.NodeUnpublishVolumeRequest{
		VolumeId:   volumeID,
		TargetPath: targetPath,
	}

	// NodeUnpublishVolume's response contains no extra data. If err == nil, we were
	// successful.
	_, err := c.nodeClient.NodeUnpublishVolume(ctx, req, opts...)
	if err != nil {
		code := status.Code(err)
		switch code {
		case codes.NotFound:
			err = fmt.Errorf("%w: volume %q could not be found: %v",
				structs.ErrCSIClientRPCIgnorable, volumeID, err)
		case codes.Internal:
			err = fmt.Errorf("node plugin returned an internal error, check the plugin allocation logs for more information: %v", err)
		}
	}

	return err
}
