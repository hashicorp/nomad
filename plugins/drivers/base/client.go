package base

import (
	"errors"
	"time"

	"github.com/hashicorp/nomad/plugins/drivers/base/proto"
	"golang.org/x/net/context"
)

type DriverClient struct {
	client proto.DriverClient
}

func (d *DriverClient) Fingerprint() *Fingerprint {
	return nil
}

func (d *DriverClient) RecoverTask(h *TaskHandle) error {
	_, err := d.client.RecoverTask(context.TODO(),
		&proto.RecoverTaskRequest{Handle: taskHandleToProto(h)})
	return err
}

func (d *DriverClient) StartTask(c *TaskConfig) (*TaskHandle, error) {
	resp, err := d.client.StartTask(context.TODO(),
		&proto.StartTaskRequest{
			Task: taskConfigToProto(c),
		})
	if err != nil {
		return nil, err
	}

	return taskHandleFromProto(resp.Handle), nil
}

func (d *DriverClient) WaitTask(id string) chan *TaskResult {
	ch := make(chan *TaskResult)
	go func() {
		defer close(ch)
		var result TaskResult
		resp, err := d.client.WaitTask(context.TODO(),
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

func (d *DriverClient) StopTask(taskID string, timeout time.Duration, signal string) error { return nil }
func (d *DriverClient) DestroyTask(taskID string)                                          {}
func (d *DriverClient) ListTasks(*ListTasksQuery) ([]*TaskSummary, error)                  { return nil, nil }
func (d *DriverClient) InspectTask(taskID string) (*TaskStatus, error)                     { return nil, nil }
func (d *DriverClient) TaskStats(taskID string) (*TaskStats, error)                        { return nil, nil }
