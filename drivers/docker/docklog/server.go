// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docklog

import (
	"context"

	"github.com/hashicorp/go-plugin"

	"github.com/hashicorp/nomad/drivers/docker/docklog/proto"
)

// dockerLoggerServer is the server side translation between the protobuf and native interfaces
type dockerLoggerServer struct {
	broker *plugin.GRPCBroker
	impl   DockerLogger
}

// Start proxies the protobuf Start RPC to the Start fun of the DockerLogger interface
func (s *dockerLoggerServer) Start(ctx context.Context, req *proto.StartRequest) (*proto.StartResponse, error) {
	opts := &StartOpts{
		Endpoint:    req.Endpoint,
		ContainerID: req.ContainerId,
		Stdout:      req.StdoutFifo,
		Stderr:      req.StderrFifo,
		TTY:         req.Tty,

		TLSCert: req.TlsCert,
		TLSKey:  req.TlsKey,
		TLSCA:   req.TlsCa,
	}
	err := s.impl.Start(opts)
	if err != nil {
		return nil, err
	}
	resp := &proto.StartResponse{}
	return resp, nil
}

// Stop proxies the protobuf Stop RPC to the Stop fun of the DockerLogger interface
func (s *dockerLoggerServer) Stop(ctx context.Context, req *proto.StopRequest) (*proto.StopResponse, error) {
	return &proto.StopResponse{}, s.impl.Stop()
}
