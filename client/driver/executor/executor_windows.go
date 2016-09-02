package executor

import (
	"github.com/hashicorp/nomad/helper/discover"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

const (
	CREATE_NEW_CONSOLE = 0x00000010
	CREATE_NO_WINDOW   = 0x08000000
)

func (e *UniversalExecutor) LaunchSyslogServer(ctx *ExecutorContext) (*SyslogServerState, error) {
	return nil, nil
}

func (e *UniversalExecutor) sendShutdownTask(proc *os.Process) error {
	e.logger.Printf("[DEBUG] sendShutdownWin: Sending sendShutdownWin to task with pid: %v", proc.Pid)
	binary, err := discover.NomadExecutable()
	if err != nil {
		return err
	}
	cmd := exec.Command(binary, "shutdown-task", strconv.Itoa(proc.Pid))
	err = cmd.Run()
	if err != nil {
		e.logger.Printf("[ERR] executor: unable to send shutdown signal to task: %v", err)
		// Probably we have GUI process here. Currently gracefully stopping GUI apps is not suported for windows.
		e.logger.Printf("[INFO] executor: Terminating process pid: %v", proc.Pid)
		if err := proc.Kill(); err != nil && err.Error() != finishedErr {
			return err
		}
	}
	return nil
}

func setSysProcAttributes(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	// Guaranty that the new process has a new console,
	// instead of inheriting its parent's console (the default)
	// Separate console is usefull to send CTRL+C signals without killing ourself
	// CREATE_NO_WINDOW - run console application's with separate hidden console.
	cmd.SysProcAttr.CreationFlags = CREATE_NO_WINDOW
}
