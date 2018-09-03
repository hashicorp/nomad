package base

import (
	"errors"
	"time"

	"github.com/hashicorp/nomad/plugins/drivers/base/proto"
	"golang.org/x/net/context"
)

type DriverClient interface {
	Fingerprint() chan *Fingerprint
	RecoverTask(*TaskHandle) error
	StartTask(*TaskConfig) (*TaskHandle, error)
	WaitTask(ctx context.Context, taskID string) chan *TaskResult
}

type driverClient struct {
	client proto.DriverClient
}

func (d *driverClient) Fingerprint() chan *Fingerprint {
	return nil
}

func (d *driverClient) RecoverTask(h *TaskHandle) error {
	_, err := d.client.RecoverTask(context.Background(),
		&proto.RecoverTaskRequest{Handle: taskHandleToProto(h)})
	return err
}

func (d *driverClient) StartTask(c *TaskConfig) (*TaskHandle, error) {
	resp, err := d.client.StartTask(context.Background(),
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

func (d *driverClient) StopTask(taskID string, timeout time.Duration, signal string) error {
	return nil
}
func (d *driverClient) DestroyTask(taskID string) {}
func (d *driverClient) ListTasks(q *ListTasksQuery) ([]*TaskSummary, error) {
	return nil, nil
}
func (d *driverClient) InspectTask(taskID string) (*TaskStatus, error) {
	return nil, nil
}
func (d *driverClient) TaskStats(taskID string) (*TaskStats, error) {
	return nil, nil
}
