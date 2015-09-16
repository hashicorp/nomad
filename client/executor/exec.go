// Package exec is used to invoke child processes across various platforms to
// provide the following features:
//
// - Least privilege
// - Resource constraints
// - Process isolation
//
// A "platform" may be defined as coarsely as "Windows" or as specifically as
// "linux 3.20 with systemd". This allows Nomad to use best-effort, best-
// available capabilities of each platform to provide resource constraints,
// process isolation, and security features, or otherwise take advantage of
// features that are unique to that platform.
//
// The `semantics of any particular instance are left up to the implementation.
// However, these should be completely transparent to the calling context. In
// other words, the Java driver should be able to call exec for any platform and
// just work.
package executor

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/hashicorp/nomad/nomad/structs"
)

// Executor is an interface that any platform- or capability-specific exec
// wrapper must implement. You should not need to implement a Java executor.
// Rather, you would implement a cgroups executor that the Java driver will use.
type Executor interface {
	// Limit must be called before Start and restricts the amount of resources
	// the process can use. Note that an error may be returned ONLY IF the
	// executor implements resource limiting. Otherwise Limit is ignored.
	Limit(*structs.Resources) error

	// RunAs sets the user we should use to run this command. This may be set as
	// a username, uid, or other identifier. The implementation will decide what
	// to do with it, if anything. Note that an error may be returned ONLY IF
	// the executor implements user lookups. Otherwise RunAs is ignored.
	RunAs(string) error

	// Start the process. This may wrap the actual process in another command,
	// depending on the capabilities in this environment. Errors that arise from
	// Limits or Runas may bubble through Start()
	Start() error

	// Open should be called to restore a previous pid. This might be needed if
	// nomad is restarted. This sets os.Process internally.
	Open(int) error

	// This is a convenience wrapper around Command().Wait()
	Wait() error

	// This is a convenience wrapper around Command().Process.Pid
	Pid() (int, error)

	// Shutdown should use a graceful stop mechanism so the application can
	// perform checkpointing or cleanup, if such a mechanism is available.
	// If such a mechanism is not available, Shutdown() should call ForceStop().
	Shutdown() error

	// ForceStop will terminate the process without waiting for cleanup. Every
	// implementations must provide this.
	ForceStop() error

	// Command provides access the underlying Cmd struct in case the Executor
	// interface doesn't expose the functionality you need.
	Command() *cmd
}

// Command is a mirror of exec.Command that returns a platform-specific Executor
func Command(name string, arg ...string) Executor {
	executor := NewExecutor()
	cmd := executor.Command()
	cmd.Path = name
	cmd.Args = append([]string{name}, arg...)

	if filepath.Base(name) == name {
		if lp, err := exec.LookPath(name); err != nil {
			// cmd.lookPathErr = err
		} else {
			cmd.Path = lp
		}
	}
	return executor
}

// OpenPid is similar to executor.Command but will initialize executor.Cmd with
// the Pid set to the one specified.
func OpenPid(pid int) (Executor, error) {
	executor := NewExecutor()
	err := executor.Open(pid)
	if err != nil {
		return nil, err
	}
	return executor, nil
}

// Cmd is an extension of exec.Cmd that incorporates functionality for
// re-attaching to processes, dropping priviledges, etc., based on platform-
// specific implementations.
type cmd struct {
	exec.Cmd

	// Resources is used to limit CPU and RAM used by the process, by way of
	// cgroups or a similar mechanism.
	Resources structs.Resources

	// RunAs may be a username or Uid. The implementation will decide how to use it.
	RunAs string
}

// SetUID changes the Uid for this command (must be set before starting)
func (c *cmd) SetUID(userid string) error {
	uid, err := strconv.ParseUint(userid, 10, 32)
	if err != nil {
		return fmt.Errorf("Unable to convert userid to uint32: %s", err)
	}
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	if c.SysProcAttr.Credential == nil {
		c.SysProcAttr.Credential = &syscall.Credential{}
	}
	c.SysProcAttr.Credential.Uid = uint32(uid)
	return nil
}

// SetGID changes the Gid for this command (must be set before starting)
func (c *cmd) SetGID(groupid string) error {
	gid, err := strconv.ParseUint(groupid, 10, 32)
	if err != nil {
		return fmt.Errorf("Unable to convert groupid to uint32: %s", err)
	}
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	if c.SysProcAttr.Credential == nil {
		c.SysProcAttr.Credential = &syscall.Credential{}
	}
	c.SysProcAttr.Credential.Uid = uint32(gid)
	return nil
}
