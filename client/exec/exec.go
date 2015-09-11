// Package exec is used to invoke child processes across various operating
// systems and child processes and provide the following features:
//
// - Least privilege
// - Resource constraints
// - Process isolation
//
// The semantics and implementation for these differ between operating systems,
// operating system versions, and types of child processes. For example, running
// Docker on Linux has different semantics than running Java on Windows. Also,
// versions of an OS may provide different capabilities for resource isolation,
// such as ulimits, cgroups, containers, jails, etc. Please refer to the
// relevant implementation for specific details.
package exec

import "github.com/hashicorp/nomad/nomad/structs"

type Command struct {
	// This may be a username or Uid. The implementation will decide how to use it.
	UserID string
}

type Executor interface {
	// Limit must be called before Start and restricts the amount of resources
	// the process can use
	Limit(structs.Resources)

	// Start the process. This may wrap the actual process in another command,
	// depending on the capabilities in this environment.
	Start() error

	// Shutdown should use a graceful stop mechanism so the application can
	// perform checkpointing or cleanup, if such a mechanism is available.
	// If such a mechanism is not available, Showdown() should call ForceStop().
	Shutdown() error

	// ForceStop will terminate the process without waiting for cleanup. Every
	// implementations must provide this.
	ForceStop() error
}

// DefaultExecutor uses capability testing to give you the best available
// executor based on your platform and execution environment. If you need a
// specific executor, call it directly.
func DefaultExecutor() Executor {
	// TODO Implement this
}
