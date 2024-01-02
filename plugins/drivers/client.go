// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package drivers

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/LK4D4/joincontext"
	"github.com/golang/protobuf/ptypes"
	hclog "github.com/hashicorp/go-hclog"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/pluginutils/grpcutils"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers/proto"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	sproto "github.com/hashicorp/nomad/plugins/shared/structs/proto"
	"google.golang.org/grpc/status"
)

var _ DriverPlugin = &driverPluginClient{}

type driverPluginClient struct {
	*base.BasePluginClient

	client proto.DriverClient
	logger hclog.Logger

	// doneCtx is closed when the plugin exits
	doneCtx context.Context
}

func (d *driverPluginClient) TaskConfigSchema() (*hclspec.Spec, error) {
	req := &proto.TaskConfigSchemaRequest{}

	resp, err := d.client.TaskConfigSchema(d.doneCtx, req)
	if err != nil {
		return nil, grpcutils.HandleGrpcErr(err, d.doneCtx)
	}

	return resp.Spec, nil
}

func (d *driverPluginClient) Capabilities() (*Capabilities, error) {
	req := &proto.CapabilitiesRequest{}

	resp, err := d.client.Capabilities(d.doneCtx, req)
	if err != nil {
		return nil, grpcutils.HandleGrpcErr(err, d.doneCtx)
	}

	caps := &Capabilities{}
	if resp.Capabilities != nil {
		caps.SendSignals = resp.Capabilities.SendSignals
		caps.Exec = resp.Capabilities.Exec
		caps.MustInitiateNetwork = resp.Capabilities.MustCreateNetwork

		for _, mode := range resp.Capabilities.NetworkIsolationModes {
			caps.NetIsolationModes = append(caps.NetIsolationModes, netIsolationModeFromProto(mode))
		}

		switch resp.Capabilities.FsIsolation {
		case proto.DriverCapabilities_NONE:
			caps.FSIsolation = FSIsolationNone
		case proto.DriverCapabilities_CHROOT:
			caps.FSIsolation = FSIsolationChroot
		case proto.DriverCapabilities_IMAGE:
			caps.FSIsolation = FSIsolationImage
		default:
			caps.FSIsolation = FSIsolationNone
		}

		caps.MountConfigs = MountConfigSupport(resp.Capabilities.MountConfigs)
		caps.RemoteTasks = resp.Capabilities.RemoteTasks
	}

	return caps, nil
}

// Fingerprint the driver, return a chan that will be pushed to periodically and on changes to health
func (d *driverPluginClient) Fingerprint(ctx context.Context) (<-chan *Fingerprint, error) {
	req := &proto.FingerprintRequest{}

	// Join the passed context and the shutdown context
	joinedCtx, _ := joincontext.Join(ctx, d.doneCtx)

	stream, err := d.client.Fingerprint(joinedCtx, req)
	if err != nil {
		return nil, grpcutils.HandleReqCtxGrpcErr(err, ctx, d.doneCtx)
	}

	ch := make(chan *Fingerprint, 1)
	go d.handleFingerprint(ctx, ch, stream)

	return ch, nil
}

func (d *driverPluginClient) handleFingerprint(reqCtx context.Context, ch chan *Fingerprint, stream proto.Driver_FingerprintClient) {
	defer close(ch)
	for {
		pb, err := stream.Recv()
		if err != nil {
			if err != io.EOF {
				ch <- &Fingerprint{
					Err: grpcutils.HandleReqCtxGrpcErr(err, reqCtx, d.doneCtx),
				}
			}

			// End the stream
			return
		}

		f := &Fingerprint{
			Attributes:        pstructs.ConvertProtoAttributeMap(pb.Attributes),
			Health:            healthStateFromProto(pb.Health),
			HealthDescription: pb.HealthDescription,
		}

		select {
		case <-reqCtx.Done():
			return
		case ch <- f:
		}
	}
}

// RecoverTask does internal state recovery to be able to control the task of
// the given TaskHandle
func (d *driverPluginClient) RecoverTask(h *TaskHandle) error {
	req := &proto.RecoverTaskRequest{Handle: taskHandleToProto(h)}

	_, err := d.client.RecoverTask(d.doneCtx, req)
	return grpcutils.HandleGrpcErr(err, d.doneCtx)
}

// StartTask starts execution of a task with the given TaskConfig. A TaskHandle
// is returned to the caller that can be used to recover state of the task,
// should the driver crash or exit prematurely.
func (d *driverPluginClient) StartTask(c *TaskConfig) (*TaskHandle, *DriverNetwork, error) {
	req := &proto.StartTaskRequest{
		Task: taskConfigToProto(c),
	}

	resp, err := d.client.StartTask(d.doneCtx, req)
	if err != nil {
		st := status.Convert(err)
		if len(st.Details()) > 0 {
			if rec, ok := st.Details()[0].(*sproto.RecoverableError); ok {
				return nil, nil, structs.NewRecoverableError(err, rec.Recoverable)
			}
		}
		return nil, nil, grpcutils.HandleGrpcErr(err, d.doneCtx)
	}

	var net *DriverNetwork
	if resp.NetworkOverride != nil {
		net = &DriverNetwork{
			PortMap:       map[string]int{},
			IP:            resp.NetworkOverride.Addr,
			AutoAdvertise: resp.NetworkOverride.AutoAdvertise,
		}
		for k, v := range resp.NetworkOverride.PortMap {
			net.PortMap[k] = int(v)
		}
	}

	return taskHandleFromProto(resp.Handle), net, nil
}

// WaitTask returns a channel that will have an ExitResult pushed to it once when the task
// exits on its own or is killed. If WaitTask is called after the task has exited, the channel
// will immedialy return the ExitResult. WaitTask can be called multiple times for
// the same task without issue.
func (d *driverPluginClient) WaitTask(ctx context.Context, id string) (<-chan *ExitResult, error) {
	ch := make(chan *ExitResult)
	go d.handleWaitTask(ctx, id, ch)
	return ch, nil
}

func (d *driverPluginClient) handleWaitTask(ctx context.Context, id string, ch chan *ExitResult) {
	defer close(ch)
	var result ExitResult
	req := &proto.WaitTaskRequest{
		TaskId: id,
	}

	// Join the passed context and the shutdown context
	joinedCtx, joinedCtxCancel := joincontext.Join(ctx, d.doneCtx)
	defer joinedCtxCancel()

	resp, err := d.client.WaitTask(joinedCtx, req)
	if err != nil {
		result.Err = grpcutils.HandleReqCtxGrpcErr(err, ctx, d.doneCtx)
	} else {
		result.ExitCode = int(resp.Result.ExitCode)
		result.Signal = int(resp.Result.Signal)
		result.OOMKilled = resp.Result.OomKilled
		if len(resp.Err) > 0 {
			result.Err = errors.New(resp.Err)
		}
	}
	ch <- &result
}

// StopTask stops the task with the given taskID. A timeout and signal can be
// given to control a graceful termination of the task. The driver will send the
// given signal to the task and wait for the given timeout for it to exit. If the
// task does not exit within the timeout it will be forcefully killed.
func (d *driverPluginClient) StopTask(taskID string, timeout time.Duration, signal string) error {
	req := &proto.StopTaskRequest{
		TaskId:  taskID,
		Timeout: ptypes.DurationProto(timeout),
		Signal:  signal,
	}

	_, err := d.client.StopTask(d.doneCtx, req)
	return grpcutils.HandleGrpcErr(err, d.doneCtx)
}

// DestroyTask removes the task from the driver's in memory state. The task
// cannot be running unless force is set to true. If force is set to true the
// driver will forcefully terminate the task before removing it.
func (d *driverPluginClient) DestroyTask(taskID string, force bool) error {
	req := &proto.DestroyTaskRequest{
		TaskId: taskID,
		Force:  force,
	}

	_, err := d.client.DestroyTask(d.doneCtx, req)
	return grpcutils.HandleGrpcErr(err, d.doneCtx)
}

// InspectTask returns status information for a task
func (d *driverPluginClient) InspectTask(taskID string) (*TaskStatus, error) {
	req := &proto.InspectTaskRequest{TaskId: taskID}

	resp, err := d.client.InspectTask(d.doneCtx, req)
	if err != nil {
		return nil, grpcutils.HandleGrpcErr(err, d.doneCtx)
	}

	status, err := taskStatusFromProto(resp.Task)
	if err != nil {
		return nil, err
	}

	if resp.Driver != nil {
		status.DriverAttributes = resp.Driver.Attributes
	}
	if resp.NetworkOverride != nil {
		status.NetworkOverride = &DriverNetwork{
			PortMap:       map[string]int{},
			IP:            resp.NetworkOverride.Addr,
			AutoAdvertise: resp.NetworkOverride.AutoAdvertise,
		}
		for k, v := range resp.NetworkOverride.PortMap {
			status.NetworkOverride.PortMap[k] = int(v)
		}
	}

	return status, nil
}

// TaskStats returns resource usage statistics for the task
func (d *driverPluginClient) TaskStats(ctx context.Context, taskID string, interval time.Duration) (<-chan *cstructs.TaskResourceUsage, error) {
	req := &proto.TaskStatsRequest{
		TaskId:             taskID,
		CollectionInterval: ptypes.DurationProto(interval),
	}
	ctx, _ = joincontext.Join(ctx, d.doneCtx)
	stream, err := d.client.TaskStats(ctx, req)
	if err != nil {
		st := status.Convert(err)
		if len(st.Details()) > 0 {
			if rec, ok := st.Details()[0].(*sproto.RecoverableError); ok {
				return nil, structs.NewRecoverableError(err, rec.Recoverable)
			}
		}
		return nil, grpcutils.HandleGrpcErr(err, d.doneCtx)
	}

	ch := make(chan *cstructs.TaskResourceUsage, 1)
	go d.handleStats(ctx, ch, stream)

	return ch, nil
}

func (d *driverPluginClient) handleStats(ctx context.Context, ch chan<- *cstructs.TaskResourceUsage, stream proto.Driver_TaskStatsClient) {
	defer close(ch)
	for {
		resp, err := stream.Recv()
		if ctx.Err() != nil {
			// Context canceled; exit gracefully
			return
		}

		if err != nil {
			if err != io.EOF {
				d.logger.Error("error receiving stream from TaskStats driver RPC, closing stream", "error", err)
			}

			// End of stream
			return
		}

		stats, err := TaskStatsFromProto(resp.Stats)
		if err != nil {
			d.logger.Error("failed to decode stats from RPC", "error", err, "stats", resp.Stats)
			continue
		}

		select {
		case ch <- stats:
		case <-ctx.Done():
			return
		}
	}
}

// TaskEvents returns a channel that will receive events from the driver about all
// tasks such as lifecycle events, terminal errors, etc.
func (d *driverPluginClient) TaskEvents(ctx context.Context) (<-chan *TaskEvent, error) {
	req := &proto.TaskEventsRequest{}

	// Join the passed context and the shutdown context
	joinedCtx, _ := joincontext.Join(ctx, d.doneCtx)

	stream, err := d.client.TaskEvents(joinedCtx, req)
	if err != nil {
		return nil, grpcutils.HandleReqCtxGrpcErr(err, ctx, d.doneCtx)
	}

	ch := make(chan *TaskEvent, 1)
	go d.handleTaskEvents(ctx, ch, stream)
	return ch, nil
}

func (d *driverPluginClient) handleTaskEvents(reqCtx context.Context, ch chan *TaskEvent, stream proto.Driver_TaskEventsClient) {
	defer close(ch)
	for {
		ev, err := stream.Recv()
		if err != nil {
			if err != io.EOF {
				ch <- &TaskEvent{
					Err: grpcutils.HandleReqCtxGrpcErr(err, reqCtx, d.doneCtx),
				}
			}

			// End the stream
			return
		}

		timestamp, _ := ptypes.Timestamp(ev.Timestamp)
		event := &TaskEvent{
			TaskID:      ev.TaskId,
			AllocID:     ev.AllocId,
			TaskName:    ev.TaskName,
			Annotations: ev.Annotations,
			Message:     ev.Message,
			Timestamp:   timestamp,
		}
		select {
		case <-reqCtx.Done():
			return
		case ch <- event:
		}
	}
}

// SignalTask will send the given signal to the specified task
func (d *driverPluginClient) SignalTask(taskID string, signal string) error {
	req := &proto.SignalTaskRequest{
		TaskId: taskID,
		Signal: signal,
	}
	_, err := d.client.SignalTask(d.doneCtx, req)
	return grpcutils.HandleGrpcErr(err, d.doneCtx)
}

// ExecTask will run the given command within the execution context of the task.
// The driver will wait for the given timeout for the command to complete before
// terminating it. The stdout and stderr of the command will be return to the caller,
// along with other exit information such as exit code.
func (d *driverPluginClient) ExecTask(taskID string, cmd []string, timeout time.Duration) (*ExecTaskResult, error) {
	req := &proto.ExecTaskRequest{
		TaskId:  taskID,
		Command: cmd,
		Timeout: ptypes.DurationProto(timeout),
	}

	resp, err := d.client.ExecTask(d.doneCtx, req)
	if err != nil {
		return nil, grpcutils.HandleGrpcErr(err, d.doneCtx)
	}

	result := &ExecTaskResult{
		Stdout:     resp.Stdout,
		Stderr:     resp.Stderr,
		ExitResult: exitResultFromProto(resp.Result),
	}

	return result, nil
}

var _ ExecTaskStreamingRawDriver = (*driverPluginClient)(nil)

func (d *driverPluginClient) ExecTaskStreamingRaw(ctx context.Context,
	taskID string,
	command []string,
	tty bool,
	execStream ExecTaskStream) error {

	stream, err := d.client.ExecTaskStreaming(ctx)
	if err != nil {
		return grpcutils.HandleGrpcErr(err, d.doneCtx)
	}

	err = stream.Send(&proto.ExecTaskStreamingRequest{
		Setup: &proto.ExecTaskStreamingRequest_Setup{
			TaskId:  taskID,
			Command: command,
			Tty:     tty,
		},
	})
	if err != nil {
		return grpcutils.HandleGrpcErr(err, d.doneCtx)
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
			// Once we get to the end of stream successfully, we can ignore errCh:
			// e.g. input write failures after process terminates shouldn't cause method to fail
			return nil
		} else if err != nil {
			return err
		}

		if err := execStream.Send(m); err != nil {
			return err
		}
	}
}

var _ DriverNetworkManager = (*driverPluginClient)(nil)

func (d *driverPluginClient) CreateNetwork(allocID string, _ *NetworkCreateRequest) (*NetworkIsolationSpec, bool, error) {
	req := &proto.CreateNetworkRequest{
		AllocId: allocID,
	}

	resp, err := d.client.CreateNetwork(d.doneCtx, req)
	if err != nil {
		return nil, false, grpcutils.HandleGrpcErr(err, d.doneCtx)
	}

	return NetworkIsolationSpecFromProto(resp.IsolationSpec), resp.Created, nil
}

func (d *driverPluginClient) DestroyNetwork(allocID string, spec *NetworkIsolationSpec) error {
	if spec == nil {
		return nil
	}

	req := &proto.DestroyNetworkRequest{
		AllocId:       allocID,
		IsolationSpec: NetworkIsolationSpecToProto(spec),
	}

	_, err := d.client.DestroyNetwork(d.doneCtx, req)
	if err != nil {
		return grpcutils.HandleGrpcErr(err, d.doneCtx)
	}

	return nil
}
