package taskrunner

import (
	"context"
	"time"

	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// NewDriverHandle returns a handle for task operations on a specific task
func NewDriverHandle(driver drivers.DriverPlugin, taskID string, task *structs.Task, net *drivers.DriverNetwork) *DriverHandle {
	return &DriverHandle{
		driver: driver,
		net:    net,
		taskID: taskID,
		task:   task,
	}
}

// DriverHandle encapsulates a driver plugin client and task identifier and exposes
// an api to perform driver operations on the task
type DriverHandle struct {
	driver drivers.DriverPlugin
	net    *drivers.DriverNetwork
	task   *structs.Task
	taskID string
}

func (h *DriverHandle) ID() string {
	return h.taskID
}

func (h *DriverHandle) WaitCh(ctx context.Context) (<-chan *drivers.ExitResult, error) {
	return h.driver.WaitTask(ctx, h.taskID)
}

func (h *DriverHandle) Update(task *structs.Task) error {
	return nil
}

func (h *DriverHandle) Kill() error {
	return h.driver.StopTask(h.taskID, h.task.KillTimeout, h.task.KillSignal)
}

func (h *DriverHandle) Stats(ctx context.Context, interval time.Duration) (<-chan *cstructs.TaskResourceUsage, error) {
	return h.driver.TaskStats(ctx, h.taskID, interval)
}

func (h *DriverHandle) Signal(s string) error {
	return h.driver.SignalTask(h.taskID, s)
}

func (h *DriverHandle) Exec(timeout time.Duration, cmd string, args []string) ([]byte, int, error) {
	command := append([]string{cmd}, args...)
	res, err := h.driver.ExecTask(h.taskID, command, timeout)
	if err != nil {
		return nil, 0, err
	}
	return res.Stdout, res.ExitResult.ExitCode, res.ExitResult.Err
}

func (h *DriverHandle) Network() *drivers.DriverNetwork {
	return h.net
}
