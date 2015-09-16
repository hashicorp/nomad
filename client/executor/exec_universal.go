// +build !linux

package executor

import (
	"fmt"
	"os"

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

func (e *UniversalExecutor) RunAs(userid string) error {
	// No-op
	return nil
}

func (e *UniversalExecutor) Start() error {
	// We don't want to call ourself. We want to call Start on our embedded Cmd
	return e.cmd.Start()
}

func (e *UniversalExecutor) Open(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("Failed to reopen pid %d: %s", pid, err)
	}
	e.Process = process
	return nil
}

func (e *UniversalExecutor) Wait() error {
	// We don't want to call ourself. We want to call Start on our embedded Cmd
	return e.cmd.Wait()
}

func (e *UniversalExecutor) Pid() (int, error) {
	if e.cmd.Process != nil {
		return e.cmd.Process.Pid, nil
	} else {
		return 0, fmt.Errorf("Process has finished or was never started")
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
