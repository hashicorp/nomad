package base

import (
	"errors"
	"time"

	"github.com/hashicorp/nomad/plugins/drivers/base/proto"
	"golang.org/x/net/context"
)

type DriverClient interface {
	Fingerprint(context.Context) *Fingerprint
	RecoverTask(context.Context, *TaskHandle) error
	StartTask(context.Context, *TaskConfig) (*TaskHandle, error)
	WaitTask(ctx context.Context, taskID string) chan *TaskResult
}

type driverClient struct {
	client proto.DriverClient
}

func (d *driverClient) Fingerprint(ctx context.Context) *Fingerprint {
	return nil
}

func (d *driverClient) RecoverTask(ctx context.Context, h *TaskHandle) error {
	_, err := d.client.RecoverTask(ctx,
		&proto.RecoverTaskRequest{Handle: taskHandleToProto(h)})
	return err
}

func (d *driverClient) StartTask(ctx context.Context, c *TaskConfig) (*TaskHandle, error) {
	resp, err := d.client.StartTask(ctx,
		&proto.StartTaskRequest{
			Task: taskConfigToProto(c),
		})
	if err != nil {
		return nil, err
	}

	return taskHandleFromProto(resp.Handle), nil
}

func (d *driverClient) WaitTask(ctx context.Context, id string) chan *TaskResult {
	ch := make(chan *TaskResult)
	go func() {
		defer close(ch)
		var result TaskResult
		resp, err := d.client.WaitTask(ctx,
			&proto.WaitTaskRequest{
				TaskId: id,
			})
		if err != nil {
			result.Err = err
		} else {
			result.ExitCode = int(resp.ExitCode)
			result.Signal = int(resp.Signal)
			result.Err = errors.New(resp.Err)
		}
		ch <- &result
	}()
	return ch
}

func (d *driverClient) StopTask(ctx context.Context, taskID string, timeout time.Duration, signal string) error {
	return nil
}
func (d *driverClient) DestroyTask(ctx context.Context, taskID string) {}
func (d *driverClient) ListTasks(ctx context.Context, q *ListTasksQuery) ([]*TaskSummary, error) {
	return nil, nil
}
func (d *driverClient) InspectTask(ctx context.Context, taskID string) (*TaskStatus, error) {
	return nil, nil
}
func (d *driverClient) TaskStats(ctx context.Context, taskID string) (*TaskStats, error) {
	return nil, nil
}
