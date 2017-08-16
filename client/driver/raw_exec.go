package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/client/driver/executor"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/fingerprint"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/fields"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

const (
	// The option that enables this driver in the Config.Options map.
	rawExecConfigOption = "driver.raw_exec.enable"

	// The key populated in Node Attributes to indicate presence of the Raw Exec
	// driver
	rawExecDriverAttr = "driver.raw_exec"
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
	version        string
	pluginClient   *plugin.Client
	userPid        int
	executor       executor.Executor
	killTimeout    time.Duration
	maxKillTimeout time.Duration
	logger         *log.Logger
	waitCh         chan *dstructs.WaitResult
	doneCh         chan struct{}
	taskEnv        *env.TaskEnv
	taskDir        *allocdir.TaskDir
}

// NewRawExecDriver is used to create a new raw exec driver
func NewRawExecDriver(ctx *DriverContext) Driver {
	return &RawExecDriver{DriverContext: *ctx}
}

// Validate is used to validate the driver configuration
func (d *RawExecDriver) Validate(config map[string]interface{}) error {
	fd := &fields.FieldData{
		Raw: config,
		Schema: map[string]*fields.FieldSchema{
			"command": &fields.FieldSchema{
				Type:     fields.TypeString,
				Required: true,
			},
			"args": &fields.FieldSchema{
				Type: fields.TypeArray,
			},
		},
	}

	if err := fd.Validate(); err != nil {
		return err
	}

	return nil
}

func (d *RawExecDriver) Abilities() DriverAbilities {
	return DriverAbilities{
		SendSignals: true,
		Exec:        true,
	}
}

func (d *RawExecDriver) FSIsolation() cstructs.FSIsolation {
	return cstructs.FSIsolationNone
}

func (d *RawExecDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// Check that the user has explicitly enabled this executor.
	enabled := cfg.ReadBoolDefault(rawExecConfigOption, false)

	if enabled || cfg.DevMode {
		d.logger.Printf("[WARN] driver.raw_exec: raw exec is enabled. Only enable if needed")
		node.Attributes[rawExecDriverAttr] = "1"
		return true, nil
	}

	delete(node.Attributes, rawExecDriverAttr)
	return false, nil
}

func (d *RawExecDriver) Prestart(*ExecContext, *structs.Task) (*PrestartResponse, error) {
	return nil, nil
}

func (d *RawExecDriver) Start(ctx *ExecContext, task *structs.Task) (*StartResponse, error) {
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
		LogFile:  pluginLogFile,
		LogLevel: d.config.LogLevel,
	}

	exec, pluginClient, err := createExecutor(d.config.LogOutput, d.config, executorConfig)
	if err != nil {
		return nil, err
	}
	executorCtx := &executor.ExecutorContext{
		TaskEnv: ctx.TaskEnv,
		Driver:  "raw_exec",
		AllocID: d.DriverContext.allocID,
		Task:    task,
		TaskDir: ctx.TaskDir.Dir,
		LogDir:  ctx.TaskDir.LogDir,
	}
	if err := exec.SetContext(executorCtx); err != nil {
		pluginClient.Kill()
		return nil, fmt.Errorf("failed to set executor context: %v", err)
	}

	execCmd := &executor.ExecCommand{
		Cmd:  command,
		Args: driverConfig.Args,
		User: task.User,
	}
	ps, err := exec.LaunchCmd(execCmd)
	if err != nil {
		pluginClient.Kill()
		return nil, err
	}
	d.logger.Printf("[DEBUG] driver.raw_exec: started process with pid: %v", ps.Pid)

	// Return a driver handle
	maxKill := d.DriverContext.config.MaxKillTimeout
	h := &rawExecHandle{
		pluginClient:   pluginClient,
		executor:       exec,
		userPid:        ps.Pid,
		killTimeout:    GetKillTimeout(task.KillTimeout, maxKill),
		maxKillTimeout: maxKill,
		version:        d.config.Version.VersionNumber(),
		logger:         d.logger,
		doneCh:         make(chan struct{}),
		waitCh:         make(chan *dstructs.WaitResult, 1),
		taskEnv:        ctx.TaskEnv,
		taskDir:        ctx.TaskDir,
	}
	go h.run()
	return &StartResponse{Handle: h}, nil
}

func (d *RawExecDriver) Cleanup(*ExecContext, *CreatedResources) error { return nil }

type rawExecId struct {
	Version        string
	KillTimeout    time.Duration
	MaxKillTimeout time.Duration
	UserPid        int
	PluginConfig   *PluginReattachConfig
}

func (d *RawExecDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	id := &rawExecId{}
	if err := json.Unmarshal([]byte(handleID), id); err != nil {
		return nil, fmt.Errorf("Failed to parse handle '%s': %v", handleID, err)
	}

	pluginConfig := &plugin.ClientConfig{
		Reattach: id.PluginConfig.PluginConfig(),
	}
	exec, pluginClient, err := createExecutorWithConfig(pluginConfig, d.config.LogOutput)
	if err != nil {
		d.logger.Println("[ERR] driver.raw_exec: error connecting to plugin so destroying plugin pid and user pid")
		if e := destroyPlugin(id.PluginConfig.Pid, id.UserPid); e != nil {
			d.logger.Printf("[ERR] driver.raw_exec: error destroying plugin and userpid: %v", e)
		}
		return nil, fmt.Errorf("error connecting to plugin: %v", err)
	}

	ver, _ := exec.Version()
	d.logger.Printf("[DEBUG] driver.raw_exec: version of executor: %v", ver.Version)

	// Return a driver handle
	h := &rawExecHandle{
		pluginClient:   pluginClient,
		executor:       exec,
		userPid:        id.UserPid,
		logger:         d.logger,
		killTimeout:    id.KillTimeout,
		maxKillTimeout: id.MaxKillTimeout,
		version:        id.Version,
		doneCh:         make(chan struct{}),
		waitCh:         make(chan *dstructs.WaitResult, 1),
		taskEnv:        ctx.TaskEnv,
		taskDir:        ctx.TaskDir,
	}
	go h.run()
	return h, nil
}

func (h *rawExecHandle) ID() string {
	id := rawExecId{
		Version:        h.version,
		KillTimeout:    h.killTimeout,
		MaxKillTimeout: h.maxKillTimeout,
		PluginConfig:   NewPluginReattachConfig(h.pluginClient.ReattachConfig()),
		UserPid:        h.userPid,
	}

	data, err := json.Marshal(id)
	if err != nil {
		h.logger.Printf("[ERR] driver.raw_exec: failed to marshal ID to JSON: %s", err)
	}
	return string(data)
}

func (h *rawExecHandle) WaitCh() chan *dstructs.WaitResult {
	return h.waitCh
}

func (h *rawExecHandle) Update(task *structs.Task) error {
	// Store the updated kill timeout.
	h.killTimeout = GetKillTimeout(task.KillTimeout, h.maxKillTimeout)
	h.executor.UpdateTask(task)

	// Update is not possible
	return nil
}

func (h *rawExecHandle) Exec(ctx context.Context, cmd string, args []string) ([]byte, int, error) {
	return executor.ExecScript(ctx, h.taskDir.Dir, h.taskEnv, nil, cmd, args)
}

func (h *rawExecHandle) Signal(s os.Signal) error {
	return h.executor.Signal(s)
}

func (h *rawExecHandle) Kill() error {
	if err := h.executor.ShutDown(); err != nil {
		if h.pluginClient.Exited() {
			return nil
		}
		return fmt.Errorf("executor Shutdown failed: %v", err)
	}

	select {
	case <-h.doneCh:
		return nil
	case <-time.After(h.killTimeout):
		if h.pluginClient.Exited() {
			return nil
		}
		if err := h.executor.Exit(); err != nil {
			return fmt.Errorf("executor Exit failed: %v", err)
		}

		return nil
	}
}

func (h *rawExecHandle) Stats() (*cstructs.TaskResourceUsage, error) {
	return h.executor.Stats()
}

func (h *rawExecHandle) run() {
	ps, werr := h.executor.Wait()
	close(h.doneCh)
	if ps.ExitCode == 0 && werr != nil {
		if e := killProcess(h.userPid); e != nil {
			h.logger.Printf("[ERR] driver.raw_exec: error killing user process: %v", e)
		}
	}

	// Exit the executor
	if err := h.executor.Exit(); err != nil {
		h.logger.Printf("[ERR] driver.raw_exec: error killing executor: %v", err)
	}
	h.pluginClient.Kill()

	// Send the results
	h.waitCh <- &dstructs.WaitResult{ExitCode: ps.ExitCode, Signal: ps.Signal, Err: werr}
	close(h.waitCh)
}
