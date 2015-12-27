package driver

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/executor"
	cstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/client/getter"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

// ExecDriver fork/execs tasks using as many of the underlying OS's isolation
// features.
type ExecDriver struct {
	DriverContext
	fingerprint.StaticFingerprinter
}
type ExecDriverConfig struct {
	ArtifactSource string   `mapstructure:"artifact_source"`
	Checksum       string   `mapstructure:"checksum"`
	Command        string   `mapstructure:"command"`
	Args           []string `mapstructure:"args"`
}

// execHandle is returned from Start/Open as a handle to the PID
type execHandle struct {
	cmd         executor.Executor
	killTimeout time.Duration
	logger      *log.Logger
	waitCh      chan *cstructs.WaitResult
	doneCh      chan struct{}
}

// NewExecDriver is used to create a new exec driver
func NewExecDriver(ctx *DriverContext) Driver {
	return &ExecDriver{DriverContext: *ctx}
}

func (d *ExecDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// Only enable if we are root on linux.
	if runtime.GOOS != "linux" {
		d.logger.Printf("[DEBUG] driver.exec: only available on linux, disabling")
		return false, nil
	} else if syscall.Geteuid() != 0 {
		d.logger.Printf("[DEBUG] driver.exec: must run as root user, disabling")
		return false, nil
	}

	node.Attributes["driver.exec"] = "1"
	return true, nil
}

func (d *ExecDriver) Start(ctx *ExecContext, task *structs.Task) (DriverHandle, error) {
	var driverConfig ExecDriverConfig
	if err := mapstructure.WeakDecode(task.Config, &driverConfig); err != nil {
		return nil, err
	}
	// Get the command to be ran
	command := driverConfig.Command
	if command == "" {
		return nil, fmt.Errorf("missing command for exec driver")
	}

	// Create a location to download the artifact.
	taskDir, ok := ctx.AllocDir.TaskDirs[d.DriverContext.taskName]
	if !ok {
		return nil, fmt.Errorf("Could not find task directory for task: %v", d.DriverContext.taskName)
	}

	// Check if an artificat is specified and attempt to download it
	source, ok := task.Config["artifact_source"]
	if ok && source != "" {
		// Proceed to download an artifact to be executed.
		_, err := getter.GetArtifact(
			filepath.Join(taskDir, allocdir.TaskLocal),
			driverConfig.ArtifactSource,
			driverConfig.Checksum,
			d.logger,
		)
		if err != nil {
			return nil, err
		}
	}

	// Get the environment variables.
	envVars := TaskEnvironmentVariables(ctx, task)

	// Setup the command
	cmd := executor.Command(command, driverConfig.Args...)
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
		cmd:         cmd,
		killTimeout: d.DriverContext.KillTimeout(task),
		logger:      d.logger,
		doneCh:      make(chan struct{}),
		waitCh:      make(chan *cstructs.WaitResult, 1),
	}
	go h.Wait()
	return h, nil
}

type execId struct {
	ExecutorId  string
	KillTimeout time.Duration
}

func (d *ExecDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	id := &execId{}
	if err := json.Unmarshal([]byte(handleID), id); err != nil {
		return nil, fmt.Errorf("Failed to parse handle '%s': %v", handleID, err)
	}

	// Find the process
	cmd, err := executor.OpenId(id.ExecutorId)
	if err != nil {
		return nil, fmt.Errorf("failed to open ID %v: %v", id.ExecutorId, err)
	}

	// Return a driver handle
	h := &execHandle{
		cmd:         cmd,
		logger:      d.logger,
		killTimeout: id.KillTimeout,
		doneCh:      make(chan struct{}),
		waitCh:      make(chan *cstructs.WaitResult, 1),
	}
	return h, nil
}

func (h *execHandle) ID() string {
	executorId, _ := h.cmd.ID()
	id := execId{
		ExecutorId:  executorId,
		KillTimeout: h.killTimeout,
	}

	data, err := json.Marshal(id)
	if err != nil {
		h.logger.Printf("[ERR] driver.exec: failed to marshal ID to JSON: %s", err)
	}
	return string(data)
}

func (h *execHandle) WaitCh() chan *cstructs.WaitResult {
	return h.waitCh
}

func (h *execHandle) Update(task *structs.Task) error {
	// Update is not possible
	return nil
}

func (h *execHandle) Kill() error {
	h.cmd.Shutdown()
	select {
	case <-h.doneCh:
		return nil
	case <-time.After(h.killTimeout):
		return h.cmd.ForceStop()
	}
}

func (h *execHandle) Wait() {
	res := h.cmd.Wait()
	close(h.doneCh)
	h.waitCh <- res
	close(h.waitCh)
}

func (h *execHandle) Logs() (io.Reader, error) {
	return h.cmd.Logs()
}
