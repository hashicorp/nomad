// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package executor

import (
	"context"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/drivers/shared/executor/proto"
	"google.golang.org/grpc"
)

type ExecutorPlugin struct {
	// TODO: support backwards compatibility with pre 0.9 NetRPC plugin
	plugin.NetRPCUnsupportedPlugin
	logger      hclog.Logger
	fsIsolation bool
}

func (p *ExecutorPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	if p.fsIsolation {
		proto.RegisterExecutorServer(s, &grpcExecutorServer{impl: NewExecutorWithIsolation(p.logger)})
	} else {
		proto.RegisterExecutorServer(s, &grpcExecutorServer{impl: NewExecutor(p.logger)})
	}
	return nil
}

func (p *ExecutorPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &grpcExecutorClient{
		client:  proto.NewExecutorClient(c),
		doneCtx: ctx,
		logger:  p.logger,
	}, nil
}
