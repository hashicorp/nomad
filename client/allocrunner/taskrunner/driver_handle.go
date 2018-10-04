package taskrunner

import (
	"context"
	"time"

	"github.com/hashicorp/nomad/client/allocrunnerv2/taskrunner/interfaces"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

func NewDriverHandle(driver drivers.DriverPlugin, taskID string, task *structs.Task, net *cstructs.DriverNetwork) interfaces.DriverHandle {
	return &driverHandleImpl{
		driver: driver,
		net:    net,
		taskID: taskID,
		task:   task,
	}
}

type driverHandleImpl struct {
	driver drivers.DriverPlugin
	net    *cstructs.DriverNetwork
	task   *structs.Task
	taskID string
}

func (h *driverHandleImpl) ID() string {
	return h.taskID
}

func (h *driverHandleImpl) WaitCh(ctx context.Context) (<-chan *drivers.ExitResult, error) {
	return h.driver.WaitTask(ctx, h.taskID)
}

func (h *driverHandleImpl) Update(task *structs.Task) error {
	return nil
}

func (h *driverHandleImpl) Kill() error {
	return h.driver.StopTask(h.taskID, h.task.KillTimeout, h.task.KillSignal)
}

func (h *driverHandleImpl) Stats() (*cstructs.TaskResourceUsage, error) {
	return h.driver.TaskStats(h.taskID)
}

func (h *driverHandleImpl) Signal(s string) error {
	return h.driver.SignalTask(h.taskID, s)
}

func (h *driverHandleImpl) Exec(timeout time.Duration, cmd string, args []string) ([]byte, int, error) {
	command := append([]string{cmd}, args...)
	res, err := h.driver.ExecTask(h.taskID, command, timeout)
	if err != nil {
		return nil, 0, err
	}
	return res.Stdout, res.ExitResult.ExitCode, res.ExitResult.Err
}

func (h *driverHandleImpl) Network() *cstructs.DriverNetwork {
	return h.net
}
