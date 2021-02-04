package executor

import (
	"fmt"
	"syscall"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/hashicorp/nomad/drivers/shared/executor/proto"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	sproto "github.com/hashicorp/nomad/plugins/shared/structs/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
		NoPivotRoot:        req.NoPivotRoot,
		Mounts:             drivers.MountsFromProto(req.Mounts),
		Devices:            drivers.DevicesFromProto(req.Devices),
		NetworkIsolation:   drivers.NetworkIsolationSpecFromProto(req.NetworkIsolation),
		DefaultModePID:     req.DefaultPidMode,
		DefaultModeIPC:     req.DefaultIpcMode,
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

func (s *grpcExecutorServer) Stats(req *proto.StatsRequest, stream proto.Executor_StatsServer) error {
	interval := time.Duration(req.Interval)
	if interval == 0 {
		interval = time.Second
	}

	outCh, err := s.impl.Stats(stream.Context(), interval)
	if err != nil {
		if rec, ok := err.(structs.Recoverable); ok {
			st := status.New(codes.FailedPrecondition, rec.Error())
			st, err := st.WithDetails(&sproto.RecoverableError{Recoverable: rec.IsRecoverable()})
			if err != nil {
				// If this error, it will always error
				panic(err)
			}
			return st.Err()
		}
		return err
	}

	for resp := range outCh {
		pbStats, err := drivers.TaskStatsToProto(resp)
		if err != nil {
			return err
		}

		presp := &proto.StatsResponse{
			Stats: pbStats,
		}

		// Send the stats
		if err := stream.Send(presp); err != nil {
			return err
		}
	}

	return nil
}

func (s *grpcExecutorServer) Signal(ctx context.Context, req *proto.SignalRequest) (*proto.SignalResponse, error) {
	sig := syscall.Signal(req.Signal)
	if err := s.impl.Signal(sig); err != nil {
		return nil, err
	}
	return &proto.SignalResponse{}, nil
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

func (s *grpcExecutorServer) ExecStreaming(server proto.Executor_ExecStreamingServer) error {
	msg, err := server.Recv()
	if err != nil {
		return fmt.Errorf("failed to receive initial message: %v", err)
	}

	if msg.Setup == nil {
		return fmt.Errorf("first message should always be setup")
	}

	return s.impl.ExecStreaming(server.Context(),
		msg.Setup.Command, msg.Setup.Tty,
		server)
}
