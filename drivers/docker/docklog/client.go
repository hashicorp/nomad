package docklog

import (
	"context"

	"github.com/hashicorp/nomad/drivers/docker/docklog/proto"
)

// docklogClient implements the Docklog interface for client side requests
type docklogClient struct {
	client proto.DocklogClient
}

// Start proxies the Start client side func to the protobuf interface
func (c *docklogClient) Start(opts *StartOpts) error {
	req := &proto.StartRequest{
		Endpoint:    opts.Endpoint,
		ContainerId: opts.ContainerID,
		StdoutFifo:  opts.Stdout,
		StderrFifo:  opts.Stderr,

		TlsCert: opts.TLSCert,
		TlsKey:  opts.TLSKey,
		TlsCa:   opts.TLSCA,
	}
	_, err := c.client.Start(context.Background(), req)
	return err
}

// Stop proxies the Stop client side func to the protobuf interface
func (c *docklogClient) Stop() error {
	req := &proto.StopRequest{}
	_, err := c.client.Stop(context.Background(), req)
	return err
}
