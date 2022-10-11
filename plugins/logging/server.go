package logging

import (
	"context"

	"github.com/hashicorp/go-plugin"

	loglib "github.com/hashicorp/nomad/plugins/logging/lib"
	"github.com/hashicorp/nomad/plugins/logging/proto"
)

type LoggingPluginServer struct {
	broker *plugin.GRPCBroker
	impl   LoggingPlugin
}

func NewLoggingPluginServer(broker *plugin.GRPCBroker, impl LoggingPlugin) *LoggingPluginServer {
	return &LoggingPluginServer{
		broker: broker,
		impl:   impl,
	}
}

func (s *LoggingPluginServer) Start(ctx context.Context, req *proto.StartRequest) (*proto.StartResponse, error) {
	cfg := &loglib.LogConfig{
		JobID:         req.JobId,
		AllocID:       req.AllocId,
		GroupName:     req.GroupName,
		TaskName:      req.TaskName,
		LogDir:        req.LogDir,
		StdoutLogFile: req.StdoutFileName,
		StderrLogFile: req.StderrFileName,
		StdoutFifo:    req.StdoutFifo,
		StderrFifo:    req.StderrFifo,
		MaxFiles:      int(req.MaxFiles),
		MaxFileSizeMB: int(req.MaxFileSizeMb),
	}

	err := s.impl.Start(cfg)
	if err != nil {
		return nil, err
	}
	resp := &proto.StartResponse{}
	return resp, nil
}

func (s *LoggingPluginServer) Stop(ctx context.Context, req *proto.StopRequest) (*proto.StopResponse, error) {
	cfg := &loglib.LogConfig{
		JobID:         req.JobId,
		AllocID:       req.AllocId,
		GroupName:     req.GroupName,
		TaskName:      req.TaskName,
		LogDir:        req.LogDir,
		StdoutLogFile: req.StdoutFileName,
		StderrLogFile: req.StderrFileName,
		StdoutFifo:    req.StdoutFifo,
		StderrFifo:    req.StderrFifo,
	}
	return &proto.StopResponse{}, s.impl.Stop(cfg)
}

func (s *LoggingPluginServer) Fingerprint(req *proto.FingerprintRequest, stream proto.LoggingPlugin_FingerprintServer) error {
	ctx := stream.Context()
	outCh, err := s.impl.Fingerprint(ctx)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case resp, ok := <-outCh:
			if !ok {
				return nil // output channel closed, end stream
			}
			if resp.Error != nil {
				return resp.Error
			}

			// TODO: once we have some kind of capabilities info, map
			// the capabilities to the response
			presp := &proto.FingerprintResponse{}

			// Send the devices
			if err := stream.Send(presp); err != nil {
				return err
			}
		}
	}
}
