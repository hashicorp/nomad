package driver

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

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
	d := factory(ctx)
	return d, nil
}

// Factory is used to instantiate a new Driver
type Factory func(*DriverContext) Driver

// PrestartResponse is driver state returned by Driver.Prestart.
type PrestartResponse struct {
	// CreatedResources by the driver.
	CreatedResources *CreatedResources

	// Network contains driver-specific network parameters such as the port
	// map between the host and a container.
	//
	// Since the network configuration may not be fully populated by
	// Prestart, it will only be used for creating an environment for
	// Start. It will be overridden by the DriverNetwork returned by Start.
	Network *cstructs.DriverNetwork
}

// NewPrestartResponse creates a new PrestartResponse with CreatedResources
// initialized.
func NewPrestartResponse() *PrestartResponse {
	return &PrestartResponse{
		CreatedResources: NewCreatedResources(),
	}
}

// CreatedResources is a map of resources (eg downloaded images) created by a driver
// that must be cleaned up.
type CreatedResources struct {
	Resources map[string][]string
}

func NewCreatedResources() *CreatedResources {
	return &CreatedResources{Resources: make(map[string][]string)}
}

// Add a new resource if it doesn't already exist.
func (r *CreatedResources) Add(k, v string) {
	if r.Resources == nil {
		r.Resources = map[string][]string{k: []string{v}}
		return
	}
	existing, ok := r.Resources[k]
	if !ok {
		// Key doesn't exist, create it
		r.Resources[k] = []string{v}
		return
	}
	for _, item := range existing {
		if item == v {
			// resource exists, return
			return
		}
	}

	// Resource type exists but value did not, append it
	r.Resources[k] = append(existing, v)
	return
}

// Remove a resource. Return true if removed, otherwise false.
//
// Removes the entire key if the needle is the last value in the list.
func (r *CreatedResources) Remove(k, needle string) bool {
	haystack := r.Resources[k]
	for i, item := range haystack {
		if item == needle {
			r.Resources[k] = append(haystack[:i], haystack[i+1:]...)
			if len(r.Resources[k]) == 0 {
				delete(r.Resources, k)
			}
			return true
		}
	}
	return false
}

// Copy returns a new deep copy of CreatedResrouces.
func (r *CreatedResources) Copy() *CreatedResources {
	if r == nil {
		return nil
	}

	newr := CreatedResources{
		Resources: make(map[string][]string, len(r.Resources)),
	}
	for k, v := range r.Resources {
		newv := make([]string, len(v))
		copy(newv, v)
		newr.Resources[k] = newv
	}
	return &newr
}

// Merge another CreatedResources into this one. If the other CreatedResources
// is nil this method is a noop.
func (r *CreatedResources) Merge(o *CreatedResources) {
	if o == nil {
		return
	}

	for k, v := range o.Resources {
		// New key
		if len(r.Resources[k]) == 0 {
			r.Resources[k] = v
			continue
		}

		// Existing key
	OUTER:
		for _, item := range v {
			for _, existing := range r.Resources[k] {
				if item == existing {
					// Found it, move on
					continue OUTER
				}
			}

			// New item, append it
			r.Resources[k] = append(r.Resources[k], item)
		}
	}
}

func (r *CreatedResources) Hash() []byte {
	h := md5.New()

	for k, values := range r.Resources {
		io.WriteString(h, k)
		io.WriteString(h, "values")
		for i, v := range values {
			io.WriteString(h, fmt.Sprintf("%d-%v", i, v))
		}
	}

	return h.Sum(nil)
}

// StartResponse is returned by Driver.Start.
type StartResponse struct {
	// Handle to the driver's task executor for controlling the lifecycle
	// of the task.
	Handle DriverHandle

	// Network contains driver-specific network parameters such as the port
	// map between the host and a container.
	//
	// Network may be nil as not all drivers or configurations create
	// networks.
	Network *cstructs.DriverNetwork
}

// Driver is used for execution of tasks. This allows Nomad
// to support many pluggable implementations of task drivers.
// Examples could include LXC, Docker, Qemu, etc.
type Driver interface {
	// Drivers must support the fingerprint interface for detection
	fingerprint.Fingerprint

	// Prestart prepares the task environment and performs expensive
	// intialization steps like downloading images.
	//
	// CreatedResources may be non-nil even when an error occurs.
	Prestart(*ExecContext, *structs.Task) (*PrestartResponse, error)

	// Start is used to begin task execution. If error is nil,
	// StartResponse.Handle will be the handle to the task's executor.
	// StartResponse.Network may be nil if the task doesn't configure a
	// network.
	Start(ctx *ExecContext, task *structs.Task) (*StartResponse, error)

	// Open is used to re-open a handle to a task
	Open(ctx *ExecContext, handleID string) (DriverHandle, error)

	// Cleanup is called to remove resources which were created for a task
	// and no longer needed. Cleanup is not called if CreatedResources is
	// nil.
	//
	// If Cleanup returns a recoverable error it may be retried. On retry
	// it will be passed the same CreatedResources, so all successfully
	// cleaned up resources should be removed or handled idempotently.
	Cleanup(*ExecContext, *CreatedResources) error

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

	// Exec marks the driver as being able to execute arbitrary commands
	// such as health checks. Used by the ScriptExecutor interface.
	Exec bool
}

// LogEventFn is a callback which allows Drivers to emit task events.
type LogEventFn func(message string, args ...interface{})

// DriverContext is a means to inject dependencies such as loggers, configs, and
// node attributes into a Driver without having to change the Driver interface
// each time we do it. Used in conjection with Factory, above.
type DriverContext struct {
	taskName string
	allocID  string
	config   *config.Config
	logger   *log.Logger
	node     *structs.Node

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
func NewDriverContext(taskName, allocID string, config *config.Config, node *structs.Node,
	logger *log.Logger, eventEmitter LogEventFn) *DriverContext {
	return &DriverContext{
		taskName:  taskName,
		allocID:   allocID,
		config:    config,
		node:      node,
		logger:    logger,
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

	// ScriptExecutor is an interface used to execute commands such as
	// health check scripts in the a DriverHandle's context.
	ScriptExecutor
}

// ScriptExecutor is an interface that supports Exec()ing commands in the
// driver's context. Split out of DriverHandle to ease testing.
type ScriptExecutor interface {
	Exec(ctx context.Context, cmd string, args []string) ([]byte, int, error)
}

// ExecContext is a task's execution context
type ExecContext struct {
	// TaskDir contains information about the task directory structure.
	TaskDir *allocdir.TaskDir

	// TaskEnv contains the task's environment variables.
	TaskEnv *env.TaskEnv
}

// NewExecContext is used to create a new execution context
func NewExecContext(td *allocdir.TaskDir, te *env.TaskEnv) *ExecContext {
	return &ExecContext{
		TaskDir: td,
		TaskEnv: te,
	}
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
