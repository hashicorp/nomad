// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package logmon

import (
	"context"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/logmon/proto"
)

type logmonServer struct {
	broker *plugin.GRPCBroker
	impl   LogMon
}

func (s *logmonServer) Start(ctx context.Context, req *proto.StartRequest) (*proto.StartResponse, error) {
	cfg := &LogConfig{
		LogDir:        req.LogDir,
		StdoutLogFile: req.StdoutFileName,
		StderrLogFile: req.StderrFileName,
		MaxFiles:      int(req.MaxFiles),
		MaxFileSizeMB: int(req.MaxFileSizeMb),
		StdoutFifo:    req.StdoutFifo,
		StderrFifo:    req.StderrFifo,
	}

	err := s.impl.Start(cfg)
	if err != nil {
		return nil, err
	}
	resp := &proto.StartResponse{}
	return resp, nil
}

func (s *logmonServer) Stop(ctx context.Context, req *proto.StopRequest) (*proto.StopResponse, error) {
	return &proto.StopResponse{}, s.impl.Stop()
}
