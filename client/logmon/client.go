package logmon

import (
	"context"

	"github.com/hashicorp/nomad/client/logmon/proto"
)

type logmonClient struct {
	client proto.LogMonClient
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
	}
	_, err := c.client.Start(context.Background(), req)
	return err
}

func (c *logmonClient) Stop() error {
	req := &proto.StopRequest{}
	_, err := c.client.Stop(context.Background(), req)
	return err
}
