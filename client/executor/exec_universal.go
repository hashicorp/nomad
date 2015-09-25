// +build !linux

package executor

import (
	"fmt"
	"os"
	"strconv"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/nomad/structs"
)

func NewExecutor() Executor {
	return &UniversalExecutor{}
}

// UniversalExecutor should work everywhere, and as a result does not include
// any resource restrictions or runas capabilities.
type UniversalExecutor struct {
	cmd
}

func (e *UniversalExecutor) Limit(resources *structs.Resources) error {
	// No-op
	return nil
}

func (e *UniversalExecutor) ConfigureTaskDir(taskName string, alloc *allocdir.AllocDir) error {
	// No-op
	return nil
}

func (e *UniversalExecutor) Start() error {
	// We don't want to call ourself. We want to call Start on our embedded Cmd
	return e.cmd.Start()
}

func (e *UniversalExecutor) Open(pid string) error {
	pidNum, err := strconv.Atoi(pid)
	if err != nil {
		return fmt.Errorf("Failed to parse pid %v: %v", pid, err)
	}

	process, err := os.FindProcess(pidNum)
	if err != nil {
		return fmt.Errorf("Failed to reopen pid %d: %v", pidNum, err)
	}
	e.Process = process
	return nil
}

func (e *UniversalExecutor) Wait() error {
	// We don't want to call ourself. We want to call Start on our embedded Cmd
	return e.cmd.Wait()
}

func (e *UniversalExecutor) ID() (string, error) {
	if e.cmd.Process != nil {
		return strconv.Itoa(e.cmd.Process.Pid), nil
	} else {
		return "", fmt.Errorf("Process has finished or was never started")
	}
}

func (e *UniversalExecutor) Shutdown() error {
	return e.ForceStop()
}

func (e *UniversalExecutor) ForceStop() error {
	return e.Process.Kill()
}

func (e *UniversalExecutor) Command() *cmd {
	return &e.cmd
}
