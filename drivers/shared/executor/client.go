package executor

import (
	"context"
	"os"
	"time"

	"github.com/LK4D4/joincontext"
	"github.com/golang/protobuf/ptypes"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/drivers/shared/executor/proto"
	"github.com/hashicorp/nomad/plugins/drivers"
)

var _ Executor = (*grpcExecutorClient)(nil)

type grpcExecutorClient struct {
	client proto.ExecutorClient

	// doneCtx is close when the plugin exits
	doneCtx context.Context
}

func (c *grpcExecutorClient) Launch(cmd *ExecCommand) (*ProcessState, error) {
	ctx := context.Background()
	req := &proto.LaunchRequest{
		Cmd:                cmd.Cmd,
		Args:               cmd.Args,
		Resources:          resourcesToProto(cmd.Resources),
		StdoutPath:         cmd.StdoutPath,
		StderrPath:         cmd.StderrPath,
		Env:                cmd.Env,
		User:               cmd.User,
		TaskDir:            cmd.TaskDir,
		ResourceLimits:     cmd.ResourceLimits,
		BasicProcessCgroup: cmd.BasicProcessCgroup,
	}
	resp, err := c.client.Launch(ctx, req)
	if err != nil {
		return nil, err
	}

	ps, err := processStateFromProto(resp.Process)
	if err != nil {
		return nil, err
	}
	return ps, nil
}

func (c *grpcExecutorClient) Wait(ctx context.Context) (*ProcessState, error) {
	// Join the passed context and the shutdown context
	ctx, _ = joincontext.Join(ctx, c.doneCtx)

	resp, err := c.client.Wait(ctx, &proto.WaitRequest{})
	if err != nil {
		return nil, err
	}

	ps, err := processStateFromProto(resp.Process)
	if err != nil {
		return nil, err
	}

	return ps, nil
}

func (c *grpcExecutorClient) Shutdown(signal string, gracePeriod time.Duration) error {
	ctx := context.Background()
	req := &proto.ShutdownRequest{
		Signal:      signal,
		GracePeriod: gracePeriod.Nanoseconds(),
	}
	if _, err := c.client.Shutdown(ctx, req); err != nil {
		return err
	}

	return nil
}

func (c *grpcExecutorClient) UpdateResources(r *Resources) error {
	ctx := context.Background()
	req := &proto.UpdateResourcesRequest{Resources: resourcesToProto(r)}
	if _, err := c.client.UpdateResources(ctx, req); err != nil {
		return err
	}

	return nil
}

func (c *grpcExecutorClient) Version() (*ExecutorVersion, error) {
	ctx := context.Background()
	resp, err := c.client.Version(ctx, &proto.VersionRequest{})
	if err != nil {
		return nil, err
	}
	return &ExecutorVersion{Version: resp.Version}, nil
}

func (c *grpcExecutorClient) Stats() (*cstructs.TaskResourceUsage, error) {
	ctx := context.Background()
	resp, err := c.client.Stats(ctx, &proto.StatsRequest{})
	if err != nil {
		return nil, err
	}

	stats, err := drivers.TaskStatsFromProto(resp.Stats)
	if err != nil {
		return nil, err
	}
	return stats, nil

}

func (c *grpcExecutorClient) Signal(s os.Signal) error {
	ctx := context.Background()
	req := &proto.SignalRequest{
		Signal: s.String(),
	}
	if _, err := c.client.Signal(ctx, req); err != nil {
		return err
	}

	return nil
}

func (c *grpcExecutorClient) Exec(deadline time.Time, cmd string, args []string) ([]byte, int, error) {
	ctx := context.Background()
	pbDeadline, err := ptypes.TimestampProto(deadline)
	if err != nil {
		return nil, 0, err
	}
	req := &proto.ExecRequest{
		Deadline: pbDeadline,
		Cmd:      cmd,
		Args:     args,
	}

	resp, err := c.client.Exec(ctx, req)
	if err != nil {
		return nil, 0, err
	}

	return resp.Output, int(resp.ExitCode), nil
}
