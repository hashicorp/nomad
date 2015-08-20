package driver

import (
	"fmt"
	"log"

	"github.com/hashicorp/nomad/client/fingerprint"
)

// BuiltinDrivers contains the built in registered drivers
// which are available for allocation handling
var BuiltinDrivers = map[string]Factory{
	"exec": NewExecDriver,
}

// NewDriver is used to instantiate and return a new driver
// given the name and a logger
func NewDriver(name string, logger *log.Logger) (Driver, error) {
	// Lookup the factory function
	factory, ok := BuiltinDrivers[name]
	if !ok {
		return nil, fmt.Errorf("unknown driver '%s'", name)
	}

	// Instantiate the driver
	f := factory(logger)
	return f, nil
}

// Factory is used to instantiate a new Driver
type Factory func(*log.Logger) Driver

// Driver is used for execution of tasks. This allows Nomad
// to support many pluggable implementations of task drivers.
// Examples could include LXC, Docker, Qemu, etc.
type Driver interface {
	// Drivers must support the fingerprint interface for detection
	fingerprint.Fingerprint
}
