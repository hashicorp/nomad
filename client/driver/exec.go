package driver

import (
	"log"

	"github.com/hashicorp/nomad/nomad/structs"
)

// ExecDriver is the simplest possible driver. It literally just
// fork/execs tasks. It should probably not be used for most things,
// but is useful for testing purposes or for very simple tasks.
type ExecDriver struct {
	logger *log.Logger
}

// NewExecDriver is used to create a new exec driver
func NewExecDriver(logger *log.Logger) Driver {
	d := &ExecDriver{
		logger: logger,
	}
	return d
}

func (d *ExecDriver) Fingerprint(node *structs.Node) (bool, error) {
	// We can always do a fork/exec
	node.Attributes["driver.exec"] = "1"
	return true, nil
}
