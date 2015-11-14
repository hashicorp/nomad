package driver

import (
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/executor"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/client/getter"
	"github.com/hashicorp/nomad/nomad/structs"

	cstructs "github.com/hashicorp/nomad/client/driver/structs"
)

const (
	// The option that enables this driver in the Config.Options map.
	rawExecConfigOption = "driver.raw_exec.enable"
)

// The RawExecDriver is a privileged version of the exec driver. It provides no
// resource isolation and just fork/execs. The Exec driver should be preferred
// and this should only be used when explicitly needed.
type RawExecDriver struct {
	DriverContext
	fingerprint.StaticFingerprinter
}

// rawExecHandle is returned from Start/Open as a handle to the PID
type rawExecHandle struct {
	cmd    executor.Executor
	waitCh chan *cstructs.WaitResult
	doneCh chan struct{}
}

// NewRawExecDriver is used to create a new raw exec driver
func NewRawExecDriver(ctx *DriverContext) Driver {
	return &RawExecDriver{DriverContext: *ctx}
}

func (d *RawExecDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// Check that the user has explicitly enabled this executor.
	enabled, err := strconv.ParseBool(cfg.ReadDefault(rawExecConfigOption, "false"))
	if err != nil {
		return false, fmt.Errorf("Failed to parse %v option: %v", rawExecConfigOption, err)
	}

	if enabled {
		d.logger.Printf("[WARN] driver.raw_exec: raw exec is enabled. Only enable if needed")
		node.Attributes["driver.raw_exec"] = "1"
		return true, nil
	}

	return false, nil
}

func (d *RawExecDriver) Start(ctx *ExecContext, task *structs.Task) (DriverHandle, error) {
	// Get the tasks local directory.
	taskName := d.DriverContext.taskName
	taskDir, ok := ctx.AllocDir.TaskDirs[taskName]
	if !ok {
		return nil, fmt.Errorf("Could not find task directory for task: %v", d.DriverContext.taskName)
	}

	// Get the command to be ran
	command, ok := task.Config["command"]
	if !ok || command == "" {
		return nil, fmt.Errorf("missing command for Raw Exec driver")
	}

	// Check if an artificat is specified and attempt to download it
	source, ok := task.Config["artifact_source"]
	if ok && source != "" {
		// Proceed to download an artifact to be executed.
		_, err := getter.GetArtifact(
			filepath.Join(taskDir, allocdir.TaskLocal),
			task.Config["artifact_source"],
			task.Config["checksum"],
			d.logger,
		)
		if err != nil {
			return nil, err
		}
	}

	// Get the environment variables.
	envVars := TaskEnvironmentVariables(ctx, task)

	// Look for arguments
	var args []string
	if argRaw, ok := task.Config["args"]; ok {
		args = append(args, argRaw)
	}

	// Setup the command
	cmd := executor.NewBasicExecutor()
	executor.SetCommand(cmd, command, args)
	if err := cmd.Limit(task.Resources); err != nil {
		return nil, fmt.Errorf("failed to constrain resources: %s", err)
	}

	// Populate environment variables
	cmd.Command().Env = envVars.List()

	if err := cmd.ConfigureTaskDir(d.taskName, ctx.AllocDir); err != nil {
		return nil, fmt.Errorf("failed to configure task directory: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %v", err)
	}

	// Return a driver handle
	h := &execHandle{
		cmd:    cmd,
		doneCh: make(chan struct{}),
		waitCh: make(chan *cstructs.WaitResult, 1),
	}
	go h.run()
	return h, nil
}

func (d *RawExecDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	// Find the process
	cmd := executor.NewBasicExecutor()
	if err := cmd.Open(handleID); err != nil {
		return nil, fmt.Errorf("failed to open ID %v: %v", handleID, err)
	}

	// Return a driver handle
	h := &execHandle{
		cmd:    cmd,
		doneCh: make(chan struct{}),
		waitCh: make(chan *cstructs.WaitResult, 1),
	}
	go h.run()
	return h, nil
}

func (h *rawExecHandle) ID() string {
	id, _ := h.cmd.ID()
	return id
}

func (h *rawExecHandle) WaitCh() chan *cstructs.WaitResult {
	return h.waitCh
}

func (h *rawExecHandle) Update(task *structs.Task) error {
	// Update is not possible
	return nil
}

func (h *rawExecHandle) Kill() error {
	h.cmd.Shutdown()
	select {
	case <-h.doneCh:
		return nil
	case <-time.After(5 * time.Second):
		return h.cmd.ForceStop()
	}
}

func (h *rawExecHandle) run() {
	res := h.cmd.Wait()
	close(h.doneCh)
	h.waitCh <- res
	close(h.waitCh)
}
