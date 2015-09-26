package driver

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/hashicorp/nomad/client/allocdir"
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
	taskName string
	config   *config.Config
	logger   *log.Logger
	node     *structs.Node
}

// NewDriverContext initializes a new DriverContext with the specified fields.
// This enables other packages to create DriverContexts but keeps the fields
// private to the driver. If we want to change this later we can gorename all of
// the fields in DriverContext.
func NewDriverContext(taskName string, config *config.Config, node *structs.Node, logger *log.Logger) *DriverContext {
	return &DriverContext{
		taskName: taskName,
		config:   config,
		node:     node,
		logger:   logger,
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

	// AllocDir contains information about the alloc directory structure.
	AllocDir *allocdir.AllocDir
}

// NewExecContext is used to create a new execution context
func NewExecContext(alloc *allocdir.AllocDir) *ExecContext {
	return &ExecContext{AllocDir: alloc}
}

// PopulateEnvironment takes a map of environment variables to their values
// and outputs is a list of strings with NAME=value pairs.
func PopulateEnvironment(envVars map[string]string) []string {
	env := []string{}
	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return env
}

// TaskEnvironmentVariables converts exec context and task configuration into a
// map of environment variables.
func TaskEnvironmentVariables(ctx *ExecContext, task *structs.Task) map[string]string {
	env := make(map[string]string)

	// This environment variable is liable to be changed by the drivers.
	env["NOMAD_ALLOC_DIR"] = ctx.AllocDir.AllocDir

	if task.Resources != nil {
		env["NOMAD_MEMORY_LIMIT"] = strconv.Itoa(task.Resources.MemoryMB)
		env["NOMAD_CPU_LIMIT"] = strconv.Itoa(task.Resources.CPU)

		if len(task.Resources.Networks) > 0 {
			network := task.Resources.Networks[0]

			// IP address for this task
			env["NOMAD_IP"] = network.IP

			// Named ports for this task
			for label, port := range network.MapDynamicPorts() {
				env[fmt.Sprintf("NOMAD_PORT_%s", label)] = strconv.Itoa(port)
			}
		}
	}

	// Meta values
	for key, value := range task.Meta {
		env[fmt.Sprintf("NOMAD_META_%s", strings.ToUpper(key))] = value
	}

	return env
}
