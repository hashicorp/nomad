// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docklog

import (
	"context"

	"github.com/hashicorp/nomad/drivers/docker/docklog/proto"
)

// dockerLoggerClient implements the dockerLogger interface for client side requests
type dockerLoggerClient struct {
	client proto.DockerLoggerClient
}

// Start proxies the Start client side func to the protobuf interface
func (c *dockerLoggerClient) Start(opts *StartOpts) error {
	req := &proto.StartRequest{
		Endpoint:    opts.Endpoint,
		ContainerId: opts.ContainerID,
		StdoutFifo:  opts.Stdout,
		StderrFifo:  opts.Stderr,
		Tty:         opts.TTY,

		TlsCert: opts.TLSCert,
		TlsKey:  opts.TLSKey,
		TlsCa:   opts.TLSCA,
	}
	_, err := c.client.Start(context.Background(), req)
	return err
}

// Stop proxies the Stop client side func to the protobuf interface
func (c *dockerLoggerClient) Stop() error {
	req := &proto.StopRequest{}
	_, err := c.client.Stop(context.Background(), req)
	return err
}
