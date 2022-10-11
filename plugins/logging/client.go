package logging

import (
	"context"
	"time"

	"github.com/hashicorp/nomad/helper/pluginutils/grpcutils"
	loglib "github.com/hashicorp/nomad/plugins/logging/lib"
	"github.com/hashicorp/nomad/plugins/logging/proto"
)

type LoggingPluginClient struct {
	client proto.LoggingPluginClient

	// doneCtx is closed when the plugin exits
	doneCtx context.Context
}

const loggingRPCTimeout = 1 * time.Minute

func (c *LoggingPluginClient) Start(cfg *loglib.LogConfig) error {
	req := &proto.StartRequest{
		JobId:          cfg.JobID,
		AllocId:        cfg.AllocID,
		GroupName:      cfg.GroupName,
		TaskName:       cfg.TaskName,
		LogDir:         cfg.LogDir,
		StdoutFileName: cfg.StdoutLogFile,
		StderrFileName: cfg.StderrLogFile,
		StdoutFifo:     cfg.StdoutFifo,
		StderrFifo:     cfg.StderrFifo,
		MaxFiles:       uint32(cfg.MaxFiles),
		MaxFileSizeMb:  uint32(cfg.MaxFileSizeMB),
	}
	ctx, cancel := context.WithTimeout(context.Background(), loggingRPCTimeout)
	defer cancel()

	_, err := c.client.Start(ctx, req)
	return grpcutils.HandleGrpcErr(err, c.doneCtx)
}

func (c *LoggingPluginClient) Stop(cfg *loglib.LogConfig) error {
	req := &proto.StopRequest{
		JobId:          cfg.JobID,
		AllocId:        cfg.AllocID,
		GroupName:      cfg.GroupName,
		TaskName:       cfg.TaskName,
		LogDir:         cfg.LogDir,
		StdoutFileName: cfg.StdoutLogFile,
		StderrFileName: cfg.StderrLogFile,
		StdoutFifo:     cfg.StdoutFifo,
		StderrFifo:     cfg.StderrFifo,
	}
	ctx, cancel := context.WithTimeout(context.Background(), loggingRPCTimeout)
	defer cancel()

	_, err := c.client.Stop(ctx, req)
	return grpcutils.HandleGrpcErr(err, c.doneCtx)
}

func (c *LoggingPluginClient) Fingerprint() (*FingerprintResponse, error) {
	req := &proto.FingerprintRequest{}
	ctx, cancel := context.WithTimeout(context.Background(), loggingRPCTimeout)
	defer cancel()

	_, err := c.client.Fingerprint(ctx, req)
	return &FingerprintResponse{}, err
}
