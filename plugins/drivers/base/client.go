package base

import (
	"errors"
	"io"
	"time"

	"github.com/golang/protobuf/ptypes"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers/base/proto"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"golang.org/x/net/context"
)

var _ DriverPlugin = &driverPluginClient{}

type driverPluginClient struct {
	base.BasePluginClient

	client proto.DriverClient
	logger hclog.Logger
}

func (d *driverPluginClient) TaskConfigSchema() (*hclspec.Spec, error) {
	req := &proto.TaskConfigSchemaRequest{}

	resp, err := d.client.TaskConfigSchema(context.Background(), req)
	if err != nil {
		return nil, err
	}

	return resp.Spec, nil
}

func (d *driverPluginClient) Capabilities() (*Capabilities, error) {
	req := &proto.CapabilitiesRequest{}

	resp, err := d.client.Capabilities(context.Background(), req)
	if err != nil {
		return nil, err
	}

	caps := &Capabilities{}
	if resp.Capabilities != nil {
		caps.SendSignals = resp.Capabilities.SendSignals
		caps.Exec = resp.Capabilities.Exec

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
	}

	return caps, nil
}

func (d *driverPluginClient) Fingerprint() (chan *Fingerprint, error) {
	req := &proto.FingerprintRequest{}

	stream, err := d.client.Fingerprint(context.Background(), req)
	if err != nil {
		return nil, err
	}

	ch := make(chan *Fingerprint)
	go d.handleFingerprint(ch, stream)

	return ch, nil
}

func (d *driverPluginClient) handleFingerprint(ch chan *Fingerprint, stream proto.Driver_FingerprintClient) {
	defer close(ch)
	for {
		pb, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			d.logger.Error("error recieving stream from Fingerprint driver RPC", "error", err)
			break
		}
		f := &Fingerprint{
			Attributes:        pb.Attributes,
			Health:            healthStateFromProto(pb.Health),
			HealthDescription: pb.HealthDescription,
		}
		ch <- f
	}

}

func (d *driverPluginClient) RecoverTask(h *TaskHandle) error {
	_, err := d.client.RecoverTask(context.Background(),
		&proto.RecoverTaskRequest{Handle: taskHandleToProto(h)})
	return err
}

func (d *driverPluginClient) StartTask(c *TaskConfig) (*TaskHandle, error) {
	resp, err := d.client.StartTask(context.Background(),
		&proto.StartTaskRequest{
			Task: taskConfigToProto(c),
		})
	if err != nil {
		return nil, err
	}

	return taskHandleFromProto(resp.Handle), nil
}

func (d *driverPluginClient) WaitTask(ctx context.Context, id string) chan *ExitResult {
	ch := make(chan *ExitResult)
	go d.handleWaitTask(ctx, id, ch)
	return ch
}

func (d *driverPluginClient) handleWaitTask(ctx context.Context, id string, ch chan *ExitResult) {
	defer close(ch)
	var result ExitResult
	resp, err := d.client.WaitTask(ctx,
		&proto.WaitTaskRequest{
			TaskId: id,
		})
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

func (d *driverPluginClient) StopTask(taskID string, timeout time.Duration, signal string) error {
	req := &proto.StopTaskRequest{
		TaskId:  taskID,
		Timeout: ptypes.DurationProto(timeout),
		Signal:  signal,
	}

	_, err := d.client.StopTask(context.Background(), req)
	return err
}

func (d *driverPluginClient) DestroyTask(taskID string, force bool) error {
	req := &proto.DestroyTaskRequest{
		TaskId: taskID,
		Force:  force,
	}

	_, err := d.client.DestroyTask(context.Background(), req)
	return err
}

func (d *driverPluginClient) InspectTask(taskID string) (*TaskStatus, error) {
	req := &proto.InspectTaskRequest{TaskId: taskID}

	resp, err := d.client.InspectTask(context.Background(), req)
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
		status.NetworkOverride = &NetworkOverride{
			PortMap:       resp.NetworkOverride.PortMap,
			Addr:          resp.NetworkOverride.Addr,
			AutoAdvertise: resp.NetworkOverride.AutoAdvertise,
		}
	}

	return status, nil
}

func (d *driverPluginClient) TaskStats(taskID string) (*TaskStats, error) {
	req := &proto.TaskStatsRequest{TaskId: taskID}

	resp, err := d.client.TaskStats(context.Background(), req)
	if err != nil {
		return nil, err
	}

	stats, err := taskStatsFromProto(resp.Stats)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func (d *driverPluginClient) TaskEvents() (chan *TaskEvent, error) {
	req := &proto.TaskEventsRequest{}
	stream, err := d.client.TaskEvents(context.Background(), req)
	if err != nil {
		return nil, err
	}

	ch := make(chan *TaskEvent)
	go d.handleTaskEvents(ch, stream)
	return ch, nil
}

func (d *driverPluginClient) handleTaskEvents(ch chan *TaskEvent, stream proto.Driver_TaskEventsClient) {
	defer close(ch)
	for {
		ev, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			d.logger.Error("error recieving stream from TaskEvents driver RPC", "error", err)
			break
		}
		timestamp, _ := ptypes.Timestamp(ev.Timestamp)
		event := &TaskEvent{
			TaskID:      ev.TaskId,
			Annotations: ev.Annotations,
			Message:     ev.Message,
			Timestamp:   timestamp,
		}
		ch <- event
	}
}

func (d *driverPluginClient) SignalTask(taskID string, signal string) error {
	req := &proto.SignalTaskRequest{
		TaskId: taskID,
		Signal: signal,
	}
	_, err := d.client.SignalTask(context.Background(), req)
	return err
}

func (d *driverPluginClient) ExecTask(taskID string, cmd []string, timeout time.Duration) (*ExecTaskResult, error) {
	req := &proto.ExecTaskRequest{
		TaskId:  taskID,
		Command: cmd,
		Timeout: ptypes.DurationProto(timeout),
	}

	resp, err := d.client.ExecTask(context.Background(), req)
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
