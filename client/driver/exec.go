package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver/executor"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/fields"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

// ExecDriver fork/execs tasks using as many of the underlying OS's isolation
// features.
type ExecDriver struct {
	DriverContext

	// A tri-state boolean to know if the fingerprinting has happened and
	// whether it has been successful
	fingerprintSuccess *bool
}

type ExecDriverConfig struct {
	Command string   `mapstructure:"command"`
	Args    []string `mapstructure:"args"`
}

// execHandle is returned from Start/Open as a handle to the PID
type execHandle struct {
	pluginClient       *plugin.Client
	executor           executor.Executor
	userPid            int
	taskShutdownSignal string
	taskDir            *allocdir.TaskDir
	killTimeout        time.Duration
	maxKillTimeout     time.Duration
	logger             *log.Logger
	waitCh             chan *dstructs.WaitResult
	doneCh             chan struct{}
	version            string
}

// NewExecDriver is used to create a new exec driver
func NewExecDriver(ctx *DriverContext) Driver {
	return &ExecDriver{DriverContext: *ctx}
}

// Validate is used to validate the driver configuration
func (d *ExecDriver) Validate(config map[string]interface{}) error {
	fd := &fields.FieldData{
		Raw: config,
		Schema: map[string]*fields.FieldSchema{
			"command": {
				Type:     fields.TypeString,
				Required: true,
			},
			"args": {
				Type: fields.TypeArray,
			},
		},
	}

	if err := fd.Validate(); err != nil {
		return err
	}

	return nil
}

func (d *ExecDriver) Abilities() DriverAbilities {
	return DriverAbilities{
		SendSignals: true,
		Exec:        true,
	}
}

func (d *ExecDriver) FSIsolation() cstructs.FSIsolation {
	return cstructs.FSIsolationChroot
}

func (d *ExecDriver) Periodic() (bool, time.Duration) {
	return true, 15 * time.Second
}

func (d *ExecDriver) Prestart(*ExecContext, *structs.Task) (*PrestartResponse, error) {
	return nil, nil
}

func (d *ExecDriver) Start(ctx *ExecContext, task *structs.Task) (*StartResponse, error) {
	var driverConfig ExecDriverConfig
	if err := mapstructure.WeakDecode(task.Config, &driverConfig); err != nil {
		return nil, err
	}

	// Get the command to be ran
	command := driverConfig.Command
	if err := validateCommand(command, "args"); err != nil {
		return nil, err
	}

	pluginLogFile := filepath.Join(ctx.TaskDir.Dir, "executor.out")
	executorConfig := &dstructs.ExecutorConfig{
		LogFile:     pluginLogFile,
		LogLevel:    d.config.LogLevel,
		FSIsolation: true,
	}
	exec, pluginClient, err := createExecutor(d.config.LogOutput, d.config, executorConfig)
	if err != nil {
		return nil, err
	}

	_, err = getTaskKillSignal(task.KillSignal)
	if err != nil {
		return nil, err
	}

	execCmd := &executor.ExecCommand{
		Cmd:            command,
		Args:           driverConfig.Args,
		ResourceLimits: true,
		User:           getExecutorUser(task),
		Resources: &executor.Resources{
			CPU:      task.Resources.CPU,
			MemoryMB: task.Resources.MemoryMB,
			IOPS:     task.Resources.IOPS,
			DiskMB:   task.Resources.DiskMB,
		},
		Env:        ctx.TaskEnv.List(),
		TaskDir:    ctx.TaskDir.Dir,
		StdoutPath: ctx.StdoutFifo,
		StderrPath: ctx.StderrFifo,
	}

	ps, err := exec.Launch(execCmd)
	if err != nil {
		pluginClient.Kill()
		return nil, err
	}

	d.logger.Printf("[DEBUG] driver.exec: started process via plugin with pid: %v", ps.Pid)

	// Return a driver handle
	maxKill := d.DriverContext.config.MaxKillTimeout
	h := &execHandle{
		pluginClient:       pluginClient,
		userPid:            ps.Pid,
		taskShutdownSignal: task.KillSignal,
		executor:           exec,
		killTimeout:        GetKillTimeout(task.KillTimeout, maxKill),
		maxKillTimeout:     maxKill,
		logger:             d.logger,
		version:            d.config.Version.VersionNumber(),
		doneCh:             make(chan struct{}),
		waitCh:             make(chan *dstructs.WaitResult, 1),
		taskDir:            ctx.TaskDir,
	}
	go h.run()
	return &StartResponse{Handle: h}, nil
}

func (d *ExecDriver) Cleanup(*ExecContext, *CreatedResources) error { return nil }

type execId struct {
	Version        string
	KillTimeout    time.Duration
	MaxKillTimeout time.Duration
	UserPid        int
	PluginConfig   *PluginReattachConfig
}

func (d *ExecDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	id := &execId{}
	if err := json.Unmarshal([]byte(handleID), id); err != nil {
		return nil, fmt.Errorf("Failed to parse handle '%s': %v", handleID, err)
	}

	pluginConfig := &plugin.ClientConfig{
		Reattach: id.PluginConfig.PluginConfig(),
	}
	exec, client, err := createExecutorWithConfig(pluginConfig, d.config.LogOutput)
	if err != nil {
		merrs := new(multierror.Error)
		merrs.Errors = append(merrs.Errors, err)
		d.logger.Println("[ERR] driver.exec: error connecting to plugin so destroying plugin pid and user pid")
		if e := destroyPlugin(id.PluginConfig.Pid, id.UserPid); e != nil {
			merrs.Errors = append(merrs.Errors, fmt.Errorf("error destroying plugin and userpid: %v", e))
		}
		return nil, fmt.Errorf("error connecting to plugin: %v", merrs.ErrorOrNil())
	}

	ver, _ := exec.Version()
	d.logger.Printf("[DEBUG] driver.exec : version of executor: %v", ver.Version)
	// Return a driver handle
	h := &execHandle{
		pluginClient:   client,
		executor:       exec,
		userPid:        id.UserPid,
		logger:         d.logger,
		version:        id.Version,
		killTimeout:    id.KillTimeout,
		maxKillTimeout: id.MaxKillTimeout,
		doneCh:         make(chan struct{}),
		waitCh:         make(chan *dstructs.WaitResult, 1),
		taskDir:        ctx.TaskDir,
	}
	go h.run()
	return h, nil
}

func (h *execHandle) ID() string {
	id := execId{
		Version:        h.version,
		KillTimeout:    h.killTimeout,
		MaxKillTimeout: h.maxKillTimeout,
		PluginConfig:   NewPluginReattachConfig(h.pluginClient.ReattachConfig()),
		UserPid:        h.userPid,
	}

	data, err := json.Marshal(id)
	if err != nil {
		h.logger.Printf("[ERR] driver.exec: failed to marshal ID to JSON: %s", err)
	}
	return string(data)
}

func (h *execHandle) WaitCh() chan *dstructs.WaitResult {
	return h.waitCh
}

func (h *execHandle) Update(task *structs.Task) error {
	// Store the updated kill timeout.
	h.killTimeout = GetKillTimeout(task.KillTimeout, h.maxKillTimeout)
	h.executor.UpdateResources(&executor.Resources{
		CPU:      task.Resources.CPU,
		MemoryMB: task.Resources.MemoryMB,
		IOPS:     task.Resources.IOPS,
		DiskMB:   task.Resources.DiskMB,
	})

	// Update is not possible
	return nil
}

func (h *execHandle) Exec(ctx context.Context, cmd string, args []string) ([]byte, int, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		// No deadline set on context; default to 1 minute
		deadline = time.Now().Add(time.Minute)
	}
	return h.executor.Exec(deadline, cmd, args)
}

func (h *execHandle) Signal(s os.Signal) error {
	return h.executor.Signal(s)
}

func (d *execHandle) Network() *cstructs.DriverNetwork {
	return nil
}

func (h *execHandle) Kill() error {
	if err := h.executor.Shutdown(h.taskShutdownSignal, h.killTimeout); err != nil {
		if h.pluginClient.Exited() {
			return nil
		}
		return fmt.Errorf("executor Kill failed: %v", err)
	}

	select {
	case <-h.doneCh:
	case <-time.After(h.killTimeout):
		if h.pluginClient.Exited() {
			break
		}
		if err := h.executor.Shutdown(h.taskShutdownSignal, h.killTimeout); err != nil {
			return fmt.Errorf("executor Destroy failed: %v", err)
		}
	}
	return nil
}

func (h *execHandle) Stats() (*cstructs.TaskResourceUsage, error) {
	return h.executor.Stats()
}

func (h *execHandle) run() {
	ps, werr := h.executor.Wait()
	close(h.doneCh)

	// Destroy the executor
	if err := h.executor.Shutdown(h.taskShutdownSignal, 0); err != nil {
		h.logger.Printf("[ERR] driver.exec: error destroying executor: %v", err)
	}
	h.pluginClient.Kill()

	// Send the results
	h.waitCh <- dstructs.NewWaitResult(ps.ExitCode, ps.Signal, werr)
	close(h.waitCh)
}
