// +build !linux

package executor

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver/args"
	"github.com/hashicorp/nomad/client/driver/environment"
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
	if resources == nil {
		return errNoResources
	}
	return nil
}

func (e *UniversalExecutor) ConfigureTaskDir(taskName string, alloc *allocdir.AllocDir) error {
	taskDir, ok := alloc.TaskDirs[taskName]
	if !ok {
		return fmt.Errorf("Error finding task dir for (%s)", taskName)
	}
	e.Dir = taskDir
	return nil
}

func (e *UniversalExecutor) Start() error {
	// Parse the commands arguments and replace instances of Nomad environment
	// variables.
	envVars, err := environment.ParseFromList(e.cmd.Env)
	if err != nil {
		return err
	}

	parsedPath, err := args.ParseAndReplace(e.cmd.Path, envVars.Map())
	if err != nil {
		return err
	} else if len(parsedPath) != 1 {
		return fmt.Errorf("couldn't properly parse command path: %v", e.cmd.Path)
	}

	e.cmd.Path = parsedPath[0]
	combined := strings.Join(e.cmd.Args, " ")
	parsed, err := args.ParseAndReplace(combined, envVars.Map())
	if err != nil {
		return err
	}
	e.Cmd.Args = parsed

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
