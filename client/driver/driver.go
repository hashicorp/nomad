package driver

import (
	"fmt"
	"log"
	"sync"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/nomad/structs"
)

// BuiltinDrivers contains the built in registered drivers
// which are available for allocation handling
var BuiltinDrivers = map[string]Factory{
	"docker": NewDockerDriver,
	"exec":   NewExecDriver,
	"java":   NewJavaDriver,
	"qemu":   NewQemuDriver,
}

// NewDriver is used to instantiate and return a new driver
// given the name and a logger
func NewDriver(name string, ctx *DriverContext) (Driver, error) {
	// Lookup the factory function
	factory, ok := BuiltinDrivers[name]
	if !ok {
		return nil, fmt.Errorf("unknown driver '%s'", name)
	}

	// Instantiate the driver
	f := factory(ctx)
	return f, nil
}

// Factory is used to instantiate a new Driver
type Factory func(*DriverContext) Driver

// Driver is used for execution of tasks. This allows Nomad
// to support many pluggable implementations of task drivers.
// Examples could include LXC, Docker, Qemu, etc.
type Driver interface {
	// Drivers must support the fingerprint interface for detection
	fingerprint.Fingerprint

	// Start is used to being task execution
	Start(ctx *ExecContext, task *structs.Task) (DriverHandle, error)

	// Open is used to re-open a handle to a task
	Open(ctx *ExecContext, handleID string) (DriverHandle, error)
}

// DriverContext is a means to inject dependencies such as loggers, configs, and
// node attributes into a Driver without having to change the Driver interface
// each time we do it. Used in conjection with Factory, above.
type DriverContext struct {
	config *config.Config
	logger *log.Logger
	node   *structs.Node
}

// NewDriverContext initializes a new DriverContext with the specified fields.
// This enables other packages to create DriverContexts but keeps the fields
// private to the driver. If we want to change this later we can gorename all of
// the fields in DriverContext.
func NewDriverContext(config *config.Config, node *structs.Node, logger *log.Logger) *DriverContext {
	return &DriverContext{
		config: config,
		node:   node,
		logger: logger,
	}
}

// DriverHandle is an opaque handle into a driver used for task
// manipulation
type DriverHandle interface {
	// Returns an opaque handle that can be used to re-open the handle
	ID() string

	// WaitCh is used to return a channel used wait for task completion
	WaitCh() chan error

	// Update is used to update the task if possible
	Update(task *structs.Task) error

	// Kill is used to stop the task
	Kill() error
}

// ExecContext is shared between drivers within an allocation
type ExecContext struct {
	sync.Mutex

	// AllocDir is the directory used for storing any state
	// of this allocation. It will be purged on alloc destroy.
	AllocDir string
}

// NewExecContext is used to create a new execution context
func NewExecContext() *ExecContext {
	ctx := &ExecContext{}
	return ctx
}
