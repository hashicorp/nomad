package base

import (
	"errors"
	"time"

	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers/base/proto"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"golang.org/x/net/context"
)

var _ DriverPlugin = &driverPluginClient{}

type driverPluginClient struct {
	*base.BasePluginClient

	client proto.DriverClient
}

func (d *driverPluginClient) TaskConfigSchema() *hclspec.Spec {
	panic("not implemented")
}

func (d *driverPluginClient) Capabilities() *Capabilities {
	panic("not implemented")
}

func (d *driverPluginClient) Fingerprint() chan *Fingerprint {
	return nil
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
	go func() {
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
	}()
	return ch
}

func (d *driverPluginClient) StopTask(taskID string, timeout time.Duration, signal string) error {
	return nil
}
func (d *driverPluginClient) DestroyTask(taskID string, force bool) error { return nil }
func (d *driverPluginClient) InspectTask(taskID string) (*TaskStatus, error) {
	return nil, nil
}
func (d *driverPluginClient) TaskStats(taskID string) (*TaskStats, error) {
	return nil, nil
}
func (d *driverPluginClient) TaskEvents() chan *TaskEvent {
	panic("not implemented")
}

func (d *driverPluginClient) SignalTask(taskID string, signal string) error {
	panic("not implemented")
}

func (d *driverPluginClient) ExecTask(taskID string, cmd []string, timeout time.Duration) (stdout []byte, stderr []byte, result *ExitResult) {
	panic("not implemented")
}
