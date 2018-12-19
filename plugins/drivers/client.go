package drivers

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/LK4D4/joincontext"
	"github.com/golang/protobuf/ptypes"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers/proto"
	"github.com/hashicorp/nomad/plugins/shared"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	sproto "github.com/hashicorp/nomad/plugins/shared/structs/proto"
	"google.golang.org/grpc/status"
)

var _ DriverPlugin = &driverPluginClient{}

type driverPluginClient struct {
	*base.BasePluginClient

	client proto.DriverClient

	// doneCtx is closed when the plugin exits
	doneCtx context.Context
}

func (d *driverPluginClient) TaskConfigSchema() (*hclspec.Spec, error) {
	req := &proto.TaskConfigSchemaRequest{}

	resp, err := d.client.TaskConfigSchema(d.doneCtx, req)
	if err != nil {
		return nil, err
	}

	return resp.Spec, nil
}

func (d *driverPluginClient) Capabilities() (*Capabilities, error) {
	req := &proto.CapabilitiesRequest{}

	resp, err := d.client.Capabilities(d.doneCtx, req)
	if err != nil {
		return nil, err
	}

	caps := &Capabilities{}
	if resp.Capabilities != nil {
		caps.SendSignals = resp.Capabilities.SendSignals
		caps.Exec = resp.Capabilities.Exec

		switch resp.Capabilities.FsIsolation {
		case proto.DriverCapabilities_NONE:
			caps.FSIsolation = cstructs.FSIsolationNone
		case proto.DriverCapabilities_CHROOT:
			caps.FSIsolation = cstructs.FSIsolationChroot
		case proto.DriverCapabilities_IMAGE:
			caps.FSIsolation = cstructs.FSIsolationImage
		default:
			caps.FSIsolation = cstructs.FSIsolationNone
		}
	}

	return caps, nil
}

// Fingerprint the driver, return a chan that will be pushed to periodically and on changes to health
func (d *driverPluginClient) Fingerprint(ctx context.Context) (<-chan *Fingerprint, error) {
	req := &proto.FingerprintRequest{}

	// Join the passed context and the shutdown context
	ctx, _ = joincontext.Join(ctx, d.doneCtx)

	stream, err := d.client.Fingerprint(ctx, req)
	if err != nil {
		return nil, err
	}

	ch := make(chan *Fingerprint, 1)
	go d.handleFingerprint(ctx, ch, stream)

	return ch, nil
}

func (d *driverPluginClient) handleFingerprint(ctx context.Context, ch chan *Fingerprint, stream proto.Driver_FingerprintClient) {
	defer close(ch)
	for {
		pb, err := stream.Recv()
		if err != nil {
			if err != io.EOF {
				ch <- &Fingerprint{
					Err: shared.HandleStreamErr(err, ctx, d.doneCtx),
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
		case <-ctx.Done():
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
	return err
}

// StartTask starts execution of a task with the given TaskConfig. A TaskHandle
// is returned to the caller that can be used to recover state of the task,
// should the driver crash or exit prematurely.
func (d *driverPluginClient) StartTask(c *TaskConfig) (*TaskHandle, *cstructs.DriverNetwork, error) {
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
		return nil, nil, err
	}

	var net *cstructs.DriverNetwork
	if resp.NetworkOverride != nil {
		net = &cstructs.DriverNetwork{
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

	// Join the passed context and the shutdown context
	ctx, _ = joincontext.Join(ctx, d.doneCtx)

	go d.handleWaitTask(ctx, id, ch)
	return ch, nil
}

func (d *driverPluginClient) handleWaitTask(ctx context.Context, id string, ch chan *ExitResult) {
	defer close(ch)
	var result ExitResult
	req := &proto.WaitTaskRequest{
		TaskId: id,
	}

	resp, err := d.client.WaitTask(ctx, req)
	if err != nil {
		result.Err = err
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
	return err
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
	return err
}

// InspectTask returns status information for a task
func (d *driverPluginClient) InspectTask(taskID string) (*TaskStatus, error) {
	req := &proto.InspectTaskRequest{TaskId: taskID}

	resp, err := d.client.InspectTask(d.doneCtx, req)
	if err != nil {
		return nil, err
	}

	status, err := taskStatusFromProto(resp.Task)
	if err != nil {
		return nil, err
	}

	if resp.Driver != nil {
		status.DriverAttributes = resp.Driver.Attributes
	}
	if resp.NetworkOverride != nil {
		status.NetworkOverride = &cstructs.DriverNetwork{
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
func (d *driverPluginClient) TaskStats(taskID string) (*cstructs.TaskResourceUsage, error) {
	req := &proto.TaskStatsRequest{TaskId: taskID}

	resp, err := d.client.TaskStats(d.doneCtx, req)
	if err != nil {
		return nil, err
	}

	stats, err := TaskStatsFromProto(resp.Stats)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// TaskEvents returns a channel that will receive events from the driver about all
// tasks such as lifecycle events, terminal errors, etc.
func (d *driverPluginClient) TaskEvents(ctx context.Context) (<-chan *TaskEvent, error) {
	req := &proto.TaskEventsRequest{}

	// Join the passed context and the shutdown context
	ctx, _ = joincontext.Join(ctx, d.doneCtx)

	stream, err := d.client.TaskEvents(ctx, req)
	if err != nil {
		return nil, err
	}

	ch := make(chan *TaskEvent, 1)
	go d.handleTaskEvents(ctx, ch, stream)
	return ch, nil
}

func (d *driverPluginClient) handleTaskEvents(ctx context.Context, ch chan *TaskEvent, stream proto.Driver_TaskEventsClient) {
	defer close(ch)
	for {
		ev, err := stream.Recv()
		if err != nil {
			if err != io.EOF {
				ch <- &TaskEvent{
					Err: shared.HandleStreamErr(err, ctx, d.doneCtx),
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
		case <-ctx.Done():
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
	return err
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
		return nil, err
	}

	result := &ExecTaskResult{
		Stdout:     resp.Stdout,
		Stderr:     resp.Stderr,
		ExitResult: exitResultFromProto(resp.Result),
	}

	return result, nil

}
