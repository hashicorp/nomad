package logmon

import (
	"context"

	"github.com/hashicorp/nomad/client/logmon/proto"
	"github.com/hashicorp/nomad/helper/pluginutils/grpcutils"
)

type logmonClient struct {
	client proto.LogMonClient

	// doneCtx is closed when the plugin exits
	doneCtx context.Context
}

func (c *logmonClient) Start(cfg *LogConfig) error {
	req := &proto.StartRequest{
		LogDir:         cfg.LogDir,
		StdoutFileName: cfg.StdoutLogFile,
		StderrFileName: cfg.StderrLogFile,
		MaxFiles:       uint32(cfg.MaxFiles),
		MaxFileSizeMb:  uint32(cfg.MaxFileSizeMB),
		StdoutFifo:     cfg.StdoutFifo,
		StderrFifo:     cfg.StderrFifo,
		FileExtension:  cfg.FileExtension,
	}
	_, err := c.client.Start(context.Background(), req)
	return grpcutils.HandleGrpcErr(err, c.doneCtx)
}

func (c *logmonClient) Stop() error {
	req := &proto.StopRequest{}
	_, err := c.client.Stop(context.Background(), req)
	return grpcutils.HandleGrpcErr(err, c.doneCtx)
}
