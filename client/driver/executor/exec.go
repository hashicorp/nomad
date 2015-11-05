// Package executor is used to invoke child processes across various operating
// systems in a way that provides the following features:
//
// - Least privilege
// - Resource constraints
// - Process isolation
//
// An operating system may be something like "windows" or "linux with systemd".
// Executors allow drivers like `exec` and `java` to share an implementation
// for isolation capabilities on a particular operating system.
//
// For example:
//
// - `exec` and `java` on Linux use a cgroups executor
// - `exec` and `java` on FreeBSD use a jails executor
//
// However, drivers that provide their own isolation should not use executors.
// For example, using an executor to start QEMU means that the QEMU call is
// run inside a chroot+cgroup, even though the VM already provides isolation for
// the task running inside it. This is an extraneous level of indirection.
package executor

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/nomad/structs"
)

var errNoResources = fmt.Errorf("No resources are associated with this task")

// Executor is an interface that any platform- or capability-specific exec
// wrapper must implement. You should not need to implement a Java executor.
// Rather, you would implement a cgroups executor that the Java driver will use.
type Executor interface {
	// Limit must be called before Start and restricts the amount of resources
	// the process can use. Note that an error may be returned ONLY IF the
	// executor implements resource limiting. Otherwise Limit is ignored.
	Limit(*structs.Resources) error

	// ConfigureTaskDir must be called before Start and ensures that the tasks
	// directory is properly configured.
	ConfigureTaskDir(taskName string, alloc *allocdir.AllocDir) error

	// Start the process. This may wrap the actual process in another command,
	// depending on the capabilities in this environment. Errors that arise from
	// Limits or Runas may bubble through Start()
	Start() error

	// Open should be called to restore a previous execution. This might be needed if
	// nomad is restarted.
	Open(string) error

	// Wait waits till the user's command is completed.
	Wait() error

	// Returns a handle that is executor specific for use in reopening.
	ID() (string, error)

	// Shutdown should use a graceful stop mechanism so the application can
	// perform checkpointing or cleanup, if such a mechanism is available.
	// If such a mechanism is not available, Shutdown() should call ForceStop().
	Shutdown() error

	// ForceStop will terminate the process without waiting for cleanup. Every
	// implementations must provide this.
	ForceStop() error

	// Command provides access the underlying Cmd struct in case the Executor
	// interface doesn't expose the functionality you need.
	Command() *exec.Cmd
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

// OpenId is similar to executor.Command but will attempt to reopen with the
// passed ID.
func OpenId(id string) (Executor, error) {
	executor := NewExecutor()
	err := executor.Open(id)
	if err != nil {
		return nil, err
	}
	return executor, nil
}
