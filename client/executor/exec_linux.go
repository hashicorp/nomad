package executor

import (
	"fmt"
	"os"
	"os/user"
	"syscall"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/structs"
)

func NewExecutor() Executor {
	return &LinuxExecutor{}
}

// Linux executor is designed to run on linux kernel 2.8+. It will fork/exec as
// a user you specify and limit resources using rlimit.
type LinuxExecutor struct {
	cmd
	user *user.User
}

func (e *LinuxExecutor) Limit(resources *structs.Resources) error {
	// TODO limit some things
	return nil
}

func (e *LinuxExecutor) RunAs(userid string) error {
	errs := new(multierror.Error)

	// First, try to lookup the user by uid
	u, err := user.LookupId(userid)
	if err == nil {
		e.user = u
		return nil
	} else {
		errs = multierror.Append(errs, err)
	}

	// Lookup failed, so try by username instead
	u, err = user.Lookup(userid)
	if err == nil {
		e.user = u
		return nil
	} else {
		errs = multierror.Append(errs, err)
	}

	// If we got here we failed to lookup based on id and username, so we'll
	// return those errors.
	return fmt.Errorf("Failed to identify user to run as: %s", errs)
}

func (e *LinuxExecutor) Start() error {
	if e.user == nil {
		// If no user has been specified, try to run as "nobody" user so we
		// don't leak root privilege to the spawned process.
		e.RunAs("nobody")
	}

	// Set the user and group this process should run as
	if e.user != nil {
		e.cmd.SetUID(e.user.Uid)
		e.cmd.SetGID(e.user.Gid)
	}

	// We don't want to call ourself. We want to call Start on our embedded Cmd.
	return e.cmd.Start()
}

func (e *LinuxExecutor) Open(pid int) error {
	process, err := os.FindProcess(pid)
	// FindProcess doesn't do any checking against the process table so it's
	// unlikely we'll ever see this error.
	if err != nil {
		return fmt.Errorf("Failed to reopen pid %d: %s", pid, err)
	}

	// On linux FindProcess() will return a pid but doesn't actually check to
	// see whether that process is running. We'll send signal 0 to see if the
	// process is alive.
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return fmt.Errorf("Unable to signal pid %d: %s", err)
	}
	e.Process = process
	return nil
}

func (e *LinuxExecutor) Wait() error {
	// We don't want to call ourself. We want to call Start on our embedded Cmd
	return e.cmd.Wait()
}

func (e *LinuxExecutor) Pid() (int, error) {
	if e.cmd.Process != nil {
		return e.cmd.Process.Pid, nil
	} else {
		return 0, fmt.Errorf("Process has finished or was never started")
	}
}

func (e *LinuxExecutor) Shutdown() error {
	return e.ForceStop()
}

func (e *LinuxExecutor) ForceStop() error {
	return e.Process.Kill()
}

func (e *LinuxExecutor) Command() *cmd {
	return &e.cmd
}
