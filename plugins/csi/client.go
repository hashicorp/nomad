package csi

import (
	"context"
	"fmt"
	"net"
	"time"

	csipbv1 "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"google.golang.org/grpc"
)

type NodeGetInfoResponse struct {
	NodeID     string
	MaxVolumes int64
}

type client struct {
	conn             *grpc.ClientConn
	identityClient   csipbv1.IdentityClient
	controllerClient csipbv1.ControllerClient
	nodeClient       csipbv1.NodeClient
}

func (c *client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func NewClient(addr string) (CSIPlugin, error) {
	if addr == "" {
		return nil, fmt.Errorf("address is empty")
	}

	conn, err := newGrpcConn(addr)
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

func newGrpcConn(addr string) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(
		addr,
		grpc.WithInsecure(),
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
