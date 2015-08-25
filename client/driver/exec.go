package driver

import (
	"log"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ExecDriver is the simplest possible driver. It literally just
// fork/execs tasks. It should probably not be used for most things,
// but is useful for testing purposes or for very simple tasks.
type ExecDriver struct {
	logger *log.Logger
}

// execHandle is returned from Start/Open as a handle to the PID
type execHandle struct {
	waitCh chan error
}

// NewExecDriver is used to create a new exec driver
func NewExecDriver(logger *log.Logger) Driver {
	d := &ExecDriver{
		logger: logger,
	}
	return d
}

func (d *ExecDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// We can always do a fork/exec
	node.Attributes["driver.exec"] = "1"
	return true, nil
}

func (d *ExecDriver) Start(ctx *ExecContext, task *structs.Task) (DriverHandle, error) {
	// TODO
	return nil, nil
}

func (d *ExecDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	// TODO
	return nil, nil
}

func (h *execHandle) ID() string {
	return "test"
}

func (h *execHandle) WaitCh() chan error {
	return h.waitCh
}

func (h *execHandle) Kill() error {
	return nil
}
