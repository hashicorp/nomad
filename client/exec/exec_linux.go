package exec

import (
	"fmt"
	"os"
	"os/user"
	"runtime"
	"strconv"
	"syscall"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/structs"
)

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

// Linux executor is designed to run on linux kernel 2.8+. It will fork/exec as
// a user you specify and limit resources using rlimit.
type LinuxExecutor struct {
	cmd
	user *user.User
}

func (e *LinuxExecutor) Available() bool {
	return runtime.GOOS == "linux"
}

func (e *LinuxExecutor) Limit(resources structs.Resources) error {
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
		SetUID(&e.cmd, e.user.Uid)
		SetGID(&e.cmd, e.user.Gid)
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

func (e *LinuxExecutor) Shutdown() error {
	return e.ForceStop()
}

func (e *LinuxExecutor) ForceStop() error {
	return e.Process.Kill()
}

func (e *LinuxExecutor) Command() *cmd {
	return &e.cmd
}
