package driver

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/nomad/structs"

	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	cstructs "github.com/hashicorp/nomad/client/structs"
)

var (
	// BuiltinDrivers contains the built in registered drivers
	// which are available for allocation handling
	BuiltinDrivers = map[string]Factory{
		"docker":   NewDockerDriver,
		"exec":     NewExecDriver,
		"raw_exec": NewRawExecDriver,
		"java":     NewJavaDriver,
		"qemu":     NewQemuDriver,
		"rkt":      NewRktDriver,
	}

	// DriverStatsNotImplemented is the error to be returned if a driver doesn't
	// implement stats.
	DriverStatsNotImplemented = errors.New("stats not implemented for driver")
)

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

	// Prestart prepares the task environment and performs expensive
	// intialization steps like downloading images.
	Prestart(*ExecContext, *structs.Task) error

	// Start is used to being task execution
	Start(ctx *ExecContext, task *structs.Task) (DriverHandle, error)

	// Open is used to re-open a handle to a task
	Open(ctx *ExecContext, handleID string) (DriverHandle, error)

	// Drivers must validate their configuration
	Validate(map[string]interface{}) error

	// Abilities returns the abilities of the driver
	Abilities() DriverAbilities

	// FSIsolation returns the method of filesystem isolation used
	FSIsolation() cstructs.FSIsolation
}

// DriverAbilities marks the abilities the driver has.
type DriverAbilities struct {
	// SendSignals marks the driver as being able to send signals
	SendSignals bool
}

// LogEventFn is a callback which allows Drivers to emit task events.
type LogEventFn func(message string, args ...interface{})

// DriverContext is a means to inject dependencies such as loggers, configs, and
// node attributes into a Driver without having to change the Driver interface
// each time we do it. Used in conjection with Factory, above.
type DriverContext struct {
	taskName string
	config   *config.Config
	logger   *log.Logger
	node     *structs.Node
	taskEnv  *env.TaskEnvironment

	emitEvent LogEventFn
}

// NewEmptyDriverContext returns a DriverContext with all fields set to their
// zero value.
func NewEmptyDriverContext() *DriverContext {
	return &DriverContext{}
}

// NewDriverContext initializes a new DriverContext with the specified fields.
// This enables other packages to create DriverContexts but keeps the fields
// private to the driver. If we want to change this later we can gorename all of
// the fields in DriverContext.
func NewDriverContext(taskName string, config *config.Config, node *structs.Node,
	logger *log.Logger, taskEnv *env.TaskEnvironment, eventEmitter LogEventFn) *DriverContext {
	return &DriverContext{
		taskName:  taskName,
		config:    config,
		node:      node,
		logger:    logger,
		taskEnv:   taskEnv,
		emitEvent: eventEmitter,
	}
}

// DriverHandle is an opaque handle into a driver used for task
// manipulation
type DriverHandle interface {
	// Returns an opaque handle that can be used to re-open the handle
	ID() string

	// WaitCh is used to return a channel used wait for task completion
	WaitCh() chan *dstructs.WaitResult

	// Update is used to update the task if possible and update task related
	// configurations.
	Update(task *structs.Task) error

	// Kill is used to stop the task
	Kill() error

	// Stats returns aggregated stats of the driver
	Stats() (*cstructs.TaskResourceUsage, error)

	// Signal is used to send a signal to the task
	Signal(s os.Signal) error
}

// ExecContext is a task's execution context
type ExecContext struct {
	// TaskDir contains information about the task directory structure.
	TaskDir *allocdir.TaskDir

	// Alloc ID
	AllocID string
}

// NewExecContext is used to create a new execution context
func NewExecContext(td *allocdir.TaskDir, allocID string) *ExecContext {
	return &ExecContext{
		TaskDir: td,
		AllocID: allocID,
	}
}

// GetTaskEnv converts the alloc dir, the node, task and alloc into a
// TaskEnvironment.
func GetTaskEnv(taskDir *allocdir.TaskDir, node *structs.Node,
	task *structs.Task, alloc *structs.Allocation, conf *config.Config,
	vaultToken string) (*env.TaskEnvironment, error) {

	env := env.NewTaskEnvironment(node).
		SetTaskMeta(alloc.Job.CombinedTaskMeta(alloc.TaskGroup, task.Name)).
		SetJobName(alloc.Job.Name).
		SetEnvvars(task.Env).
		SetTaskName(task.Name)

	// Vary paths by filesystem isolation used
	drv, err := NewDriver(task.Driver, NewEmptyDriverContext())
	if err != nil {
		return nil, err
	}
	switch drv.FSIsolation() {
	case cstructs.FSIsolationNone:
		// Use host paths
		env.SetAllocDir(taskDir.SharedAllocDir)
		env.SetTaskLocalDir(taskDir.LocalDir)
		env.SetSecretsDir(taskDir.SecretsDir)
	default:
		// filesystem isolation; use container paths
		env.SetAllocDir(allocdir.SharedAllocContainerPath)
		env.SetTaskLocalDir(allocdir.TaskLocalContainerPath)
		env.SetSecretsDir(allocdir.TaskSecretsContainerPath)
	}

	if task.Resources != nil {
		env.SetMemLimit(task.Resources.MemoryMB).
			SetCpuLimit(task.Resources.CPU).
			SetNetworks(task.Resources.Networks)
	}

	if alloc != nil {
		env.SetAlloc(alloc)
	}

	if task.Vault != nil {
		env.SetVaultToken(vaultToken, task.Vault.Env)
	}

	// Set the host environment variables.
	filter := strings.Split(conf.ReadDefault("env.blacklist", config.DefaultEnvBlacklist), ",")
	env.AppendHostEnvvars(filter)

	return env.Build(), nil
}

func mapMergeStrInt(maps ...map[string]int) map[string]int {
	out := map[string]int{}
	for _, in := range maps {
		for key, val := range in {
			out[key] = val
		}
	}
	return out
}

func mapMergeStrStr(maps ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, in := range maps {
		for key, val := range in {
			out[key] = val
		}
	}
	return out
}
