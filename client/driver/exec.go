package driver

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

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
	proc   *os.Process
	waitCh chan error
	doneCh chan struct{}
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
	// Get the command
	command, ok := task.Config["command"]
	if !ok || command == "" {
		return nil, fmt.Errorf("missing command for exec driver")
	}

	// Look for arguments
	argRaw, ok := task.Config["args"]
	var args []string
	if ok {
		args = strings.Split(argRaw, " ")
	}

	// Setup the command
	cmd := exec.Command(command, args...)
	err := cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start command: %v", err)
	}

	// Return a driver handle
	h := &execHandle{
		proc:   cmd.Process,
		doneCh: make(chan struct{}),
		waitCh: make(chan error, 1),
	}
	go h.run()
	return h, nil
}

func (d *ExecDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	// Split the handle
	pidStr := strings.TrimPrefix(handleID, "PID:")
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse handle '%s': %v", handleID, err)
	}

	// Find the process
	proc, err := os.FindProcess(pid)
	if proc == nil || err != nil {
		return nil, fmt.Errorf("failed to find PID %d: %v", pid, err)
	}

	// Return a driver handle
	h := &execHandle{
		proc:   proc,
		doneCh: make(chan struct{}),
		waitCh: make(chan error, 1),
	}
	go h.run()
	return h, nil
}

func (h *execHandle) ID() string {
	// Return a handle to the PID
	return fmt.Sprintf("PID:%d", h.proc.Pid)
}

func (h *execHandle) WaitCh() chan error {
	return h.waitCh
}

func (h *execHandle) Update(task *structs.Task) error {
	// Update is not possible
	return nil
}

// Kill is used to terminate the task. We send an Interrupt
// and then provide a 5 second grace period before doing a Kill.
func (h *execHandle) Kill() error {
	h.proc.Signal(os.Interrupt)
	select {
	case <-h.doneCh:
		return nil
	case <-time.After(5 * time.Second):
		return h.proc.Kill()
	}
}

func (h *execHandle) run() {
	ps, err := h.proc.Wait()
	close(h.doneCh)
	if err != nil {
		h.waitCh <- err
	} else if !ps.Success() {
		h.waitCh <- fmt.Errorf("task exited with error")
	}
	close(h.waitCh)
}
