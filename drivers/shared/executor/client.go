package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/LK4D4/joincontext"
	"github.com/golang/protobuf/ptypes"
	hclog "github.com/hashicorp/go-hclog"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/drivers/shared/executor/proto"
	"github.com/hashicorp/nomad/helper/pluginutils/grpcutils"
	"github.com/hashicorp/nomad/plugins/drivers"
	dproto "github.com/hashicorp/nomad/plugins/drivers/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ Executor = (*grpcExecutorClient)(nil)

type grpcExecutorClient struct {
	client proto.ExecutorClient
	logger hclog.Logger

	// doneCtx is close when the plugin exits
	doneCtx context.Context
}

func (c *grpcExecutorClient) Launch(cmd *ExecCommand) (*ProcessState, error) {
	ctx := context.Background()
	req := &proto.LaunchRequest{
		Cmd:                cmd.Cmd,
		Args:               cmd.Args,
		Resources:          drivers.ResourcesToProto(cmd.Resources),
		StdoutPath:         cmd.StdoutPath,
		StderrPath:         cmd.StderrPath,
		Env:                cmd.Env,
		User:               cmd.User,
		TaskDir:            cmd.TaskDir,
		ResourceLimits:     cmd.ResourceLimits,
		BasicProcessCgroup: cmd.BasicProcessCgroup,
		NoPivotRoot:        cmd.NoPivotRoot,
		Mounts:             drivers.MountsToProto(cmd.Mounts),
		Devices:            drivers.DevicesToProto(cmd.Devices),
		NetworkIsolation:   drivers.NetworkIsolationSpecToProto(cmd.NetworkIsolation),
		DefaultPidMode:     cmd.ModePID,
		DefaultIpcMode:     cmd.ModeIPC,
		Capabilities:       cmd.Capabilities,
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

func (c *grpcExecutorClient) UpdateResources(r *drivers.Resources) error {
	ctx := context.Background()
	req := &proto.UpdateResourcesRequest{Resources: drivers.ResourcesToProto(r)}
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

func (c *grpcExecutorClient) Stats(ctx context.Context, interval time.Duration) (<-chan *cstructs.TaskResourceUsage, error) {
	stream, err := c.client.Stats(ctx, &proto.StatsRequest{
		Interval: int64(interval),
	})
	if err != nil {
		return nil, err
	}

	ch := make(chan *cstructs.TaskResourceUsage)
	go c.handleStats(ctx, stream, ch)
	return ch, nil
}

func (c *grpcExecutorClient) handleStats(ctx context.Context, stream proto.Executor_StatsClient, ch chan<- *cstructs.TaskResourceUsage) {
	defer close(ch)
	for {
		resp, err := stream.Recv()
		if ctx.Err() != nil {
			// Context canceled; exit gracefully
			return
		}

		if err == io.EOF ||
			status.Code(err) == codes.Unavailable ||
			status.Code(err) == codes.Canceled ||
			err == context.Canceled {
			c.logger.Trace("executor Stats stream closed", "msg", err)
			return
		} else if err != nil {
			c.logger.Warn("failed to receive Stats executor RPC stream, closing stream", "error", err)
			return
		}

		stats, err := drivers.TaskStatsFromProto(resp.Stats)
		if err != nil {
			c.logger.Error("failed to decode stats from RPC", "error", err, "stats", resp.Stats)
			continue
		}

		select {
		case ch <- stats:
		case <-ctx.Done():
			return
		}
	}
}

func (c *grpcExecutorClient) Signal(s os.Signal) error {
	ctx := context.Background()
	sig, ok := s.(syscall.Signal)
	if !ok {
		return fmt.Errorf("unsupported signal type: %q", s.String())
	}
	req := &proto.SignalRequest{
		Signal: int32(sig),
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

func (d *grpcExecutorClient) ExecStreaming(ctx context.Context,
	command []string,
	tty bool,
	execStream drivers.ExecTaskStream) error {

	err := d.execStreaming(ctx, command, tty, execStream)
	if err != nil {
		return grpcutils.HandleGrpcErr(err, d.doneCtx)
	}
	return nil
}

func (d *grpcExecutorClient) execStreaming(ctx context.Context,
	command []string,
	tty bool,
	execStream drivers.ExecTaskStream) error {

	stream, err := d.client.ExecStreaming(ctx)
	if err != nil {
		return err
	}

	err = stream.Send(&dproto.ExecTaskStreamingRequest{
		Setup: &dproto.ExecTaskStreamingRequest_Setup{
			Command: command,
			Tty:     tty,
		},
	})
	if err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		for {
			m, err := execStream.Recv()
			if err == io.EOF {
				return
			} else if err != nil {
				errCh <- err
				return
			}

			if err := stream.Send(m); err != nil {
				errCh <- err
				return
			}

		}
	}()

	for {
		select {
		case err := <-errCh:
			return err
		default:
		}

		m, err := stream.Recv()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		if err := execStream.Send(m); err != nil {
			return err
		}
	}
}
