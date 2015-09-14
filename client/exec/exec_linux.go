package exec

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"syscall"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Linux executor is designed to run on linux kernel 2.8+. It will fork/exec as
// a user you specify and limit resources using rlimit.
type LinuxExecutor struct {
	cmd
	user *user.User
}

// SetUID changes the Uid for this command (must be set before starting)
func SetUID(command *cmd, userid string) error {
	uid, err := strconv.ParseUint(userid, 10, 32)
	if err != nil {
		return fmt.Errorf("Unable to convert userid to uint32: %s", err)
	}
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	if command.SysProcAttr.Credential == nil {
		command.SysProcAttr.Credential = &syscall.Credential{}
	}
	command.SysProcAttr.Credential.Uid = uint32(uid)
	return nil
}

// SetGID changes the Gid for this command (must be set before starting)
func SetGID(command *cmd, groupid string) error {
	gid, err := strconv.ParseUint(groupid, 10, 32)
	if err != nil {
		return fmt.Errorf("Unable to convert groupid to uint32: %s", err)
	}
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	if command.SysProcAttr.Credential == nil {
		command.SysProcAttr.Credential = &syscall.Credential{}
	}
	command.SysProcAttr.Credential.Uid = uint32(gid)
	return nil
}

func (e *LinuxExecutor) Limit(resources structs.Resources) error {
	// TODO rlimit
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

	// If we got here we failed to looking based on id and username, so we'll
	// return those errors.
	return fmt.Errorf("Failed to identify user to run as: %s", errs)
}

func (e *LinuxExecutor) Start() error {
	// Set the user and group this process should run as
	if e.user != nil {
		SetUID(&e.cmd, e.user.Uid)
		SetGID(&e.cmd, e.user.Gid)
	}

	// We don't want to call ourself. We want to call Start on our embedded Cmd.
	return e.cmd.Start()
}

func (e *LinuxExecutor) Open(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("Failed to reopen pid %d: %s", pid, err)
	}
	// TODO signal the process with signal 0 to see if it's alive. Error if it
	// is not.
	e.Process = process
	return nil
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
