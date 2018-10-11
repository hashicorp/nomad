package interfaces

import (
	"context"
	"time"

	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// DriverHandle wraps operations to a driver such that they are operated on a specific
// task
type DriverHandle interface {
	// ID returns the task ID
	ID() string

	// WaitCh is used to return a channel used to wait for task completion
	WaitCh(context.Context) (<-chan *drivers.ExitResult, error)

	// Update is used to update the task if possible and update task related
	// configurations.
	Update(task *structs.Task) error

	// Kill is used to stop the task
	Kill() error

	// Stats returns aggregated stats of the driver
	Stats() (*cstructs.TaskResourceUsage, error)

	// Signal is used to send a signal to the task
	Signal(s string) error

	// ScriptExecutor is an interface used to execute commands such as
	// health check scripts in the a DriverHandle's context.
	ScriptExecutor

	// Network returns the driver's network or nil if the driver did not
	// create a network.
	Network() *cstructs.DriverNetwork
}

// ScriptExecutor is an interface that supports Exec()ing commands in the
// driver's context. Split out of DriverHandle to ease testing.
type ScriptExecutor interface {
	Exec(timeout time.Duration, cmd string, args []string) ([]byte, int, error)
}
