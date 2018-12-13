package executor

import (
	"fmt"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/hashicorp/consul-template/signals"
	"github.com/hashicorp/nomad/drivers/shared/executor/proto"
	"github.com/hashicorp/nomad/plugins/drivers"
	"golang.org/x/net/context"
)

type grpcExecutorServer struct {
	impl Executor
}

func (s *grpcExecutorServer) Launch(ctx context.Context, req *proto.LaunchRequest) (*proto.LaunchResponse, error) {
	ps, err := s.impl.Launch(&ExecCommand{
		Cmd:                req.Cmd,
		Args:               req.Args,
		Resources:          drivers.ResourcesFromProto(req.Resources),
		StdoutPath:         req.StdoutPath,
		StderrPath:         req.StderrPath,
		Env:                req.Env,
		User:               req.User,
		TaskDir:            req.TaskDir,
		ResourceLimits:     req.ResourceLimits,
		BasicProcessCgroup: req.BasicProcessCgroup,
	})

	if err != nil {
		return nil, err
	}

	process, err := processStateToProto(ps)
	if err != nil {
		return nil, err
	}

	return &proto.LaunchResponse{
		Process: process,
	}, nil
}

func (s *grpcExecutorServer) Wait(ctx context.Context, req *proto.WaitRequest) (*proto.WaitResponse, error) {
	ps, err := s.impl.Wait(ctx)
	if err != nil {
		return nil, err
	}

	process, err := processStateToProto(ps)
	if err != nil {
		return nil, err
	}

	return &proto.WaitResponse{
		Process: process,
	}, nil
}

func (s *grpcExecutorServer) Shutdown(ctx context.Context, req *proto.ShutdownRequest) (*proto.ShutdownResponse, error) {
	if err := s.impl.Shutdown(req.Signal, time.Duration(req.GracePeriod)); err != nil {
		return nil, err
	}

	return &proto.ShutdownResponse{}, nil
}

func (s *grpcExecutorServer) UpdateResources(ctx context.Context, req *proto.UpdateResourcesRequest) (*proto.UpdateResourcesResponse, error) {
	if err := s.impl.UpdateResources(drivers.ResourcesFromProto(req.Resources)); err != nil {
		return nil, err
	}

	return &proto.UpdateResourcesResponse{}, nil
}

func (s *grpcExecutorServer) Version(context.Context, *proto.VersionRequest) (*proto.VersionResponse, error) {
	v, err := s.impl.Version()
	if err != nil {
		return nil, err
	}

	return &proto.VersionResponse{
		Version: v.Version,
	}, nil
}

func (s *grpcExecutorServer) Stats(context.Context, *proto.StatsRequest) (*proto.StatsResponse, error) {
	stats, err := s.impl.Stats()
	if err != nil {
		return nil, err
	}

	pbStats, err := drivers.TaskStatsToProto(stats)
	if err != nil {
		return nil, err
	}

	return &proto.StatsResponse{
		Stats: pbStats,
	}, nil
}

func (s *grpcExecutorServer) Signal(ctx context.Context, req *proto.SignalRequest) (*proto.SignalResponse, error) {
	if sig, ok := signals.SignalLookup[req.Signal]; ok {
		if err := s.impl.Signal(sig); err != nil {
			return nil, err
		}
		return &proto.SignalResponse{}, nil
	}
	return nil, fmt.Errorf("invalid signal sent by client")
}

func (s *grpcExecutorServer) Exec(ctx context.Context, req *proto.ExecRequest) (*proto.ExecResponse, error) {
	deadline, err := ptypes.Timestamp(req.Deadline)
	if err != nil {
		return nil, err
	}

	out, exit, err := s.impl.Exec(deadline, req.Cmd, req.Args)
	if err != nil {
		return nil, err
	}

	return &proto.ExecResponse{
		Output:   out,
		ExitCode: int32(exit),
	}, nil
}
