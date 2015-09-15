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
// The semantics of any particular instance are left up to the implementation.
// However, these should be completely transparent to the calling context. In
// other words, the Java driver should be able to call exec for any platform and
// just work.
package exec

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hashicorp/nomad/nomad/structs"
)

// Executor is an interface that any platform- or capability-specific exec
// wrapper must implement. You should not need to implement a Java executor.
// Rather, you would implement a cgroups executor that the Java driver will use.
type Executor interface {
	// Available should return true or false based on whether the current platform
	// can run this type of executor, based on capability testing. Returning
	// true does not guarantee that this executor will be used.
	Available() bool

	// Limit must be called before Start and restricts the amount of resources
	// the process can use. Note that an error may be returned ONLY IF the
	// executor implements resource limiting. Otherwise Limit is ignored.
	Limit(structs.Resources) error

	// RunAs sets the user we should use to run this command. This may be set as
	// a username, uid, or other identifier. The implementation will decide what
	// to do with it, if anything. Note that an error may be returned ONLY IF
	// the executor implements user lookups. Otherwise RunAs is ignored.
	RunAs(string) error

	// Start the process. This may wrap the actual process in another command,
	// depending on the capabilities in this environment. Errors that arise from
	// Limits or Runas will bubble through Start()
	Start() error

	// Open should be called to restore a previous pid. This might be needed if
	// nomad is restarted. This sets os.Process internally.
	Open(int) error

	// Shutdown should use a graceful stop mechanism so the application can
	// perform checkpointing or cleanup, if such a mechanism is available.
	// If such a mechanism is not available, Shutdown() should call ForceStop().
	Shutdown() error

	// ForceStop will terminate the process without waiting for cleanup. Every
	// implementations must provide this.
	ForceStop() error

	// Access the underlying Cmd struct. This should never be nil. Also, this is
	// not intended to be access outside the exec package, so YMMV.
	Command() *cmd
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

// Command is a mirror of exec.Command that returns a platform-specific Executor
func Command(name string, arg ...string) Executor {
	executor := AutoselectExecutor()
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

func OpenPid(int) Executor {
	executor := AutoselectExecutor()
	return executor
}

// AutoselectExecutor uses capability testing to give you the best available
// executor based on your platform and execution environment. If you need a
// specific executor, call it directly.
//
// This is a simplistic strategy pattern. We can potentially improve this by
// using a decorator pattern instead.
func AutoselectExecutor() Executor {
	// These will be IN ORDER and the first available will be used, so preferred
	// ones should be at the top and fallbacks at the bottom.
	// TODO refactor this to be more lightweight.
	executors := []Executor{
		&LinuxExecutor{},
	}

	for _, executor := range executors {
		if executor.Available() {
			return executor
		}
	}

	// Always return something, even if we don't have advanced capabilities.
	return &UniversalExecutor{}
}

// UniversalExecutor should work everywhere, and as a result does not include
// any resource restrictions or runas capabilities.
type UniversalExecutor struct {
	cmd
}

func (e *UniversalExecutor) Available() bool {
	return true
}

func (e *UniversalExecutor) Limit(resources structs.Resources) error {
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

func (e *UniversalExecutor) Shutdown() error {
	return e.ForceStop()
}

func (e *UniversalExecutor) ForceStop() error {
	return e.Process.Kill()
}

func (e *UniversalExecutor) Command() *cmd {
	return &e.cmd
}
