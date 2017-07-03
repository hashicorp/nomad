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

const (
	// The key populated in Node Attributes to indicate the presence of the Exec
	// driver
	execDriverAttr = "driver.exec"
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
	pluginClient    *plugin.Client
	executor        executor.Executor
	isolationConfig *dstructs.IsolationConfig
	userPid         int
	taskDir         *allocdir.TaskDir
	killTimeout     time.Duration
	maxKillTimeout  time.Duration
	logger          *log.Logger
	waitCh          chan *dstructs.WaitResult
	doneCh          chan struct{}
	version         string
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
		LogFile:  pluginLogFile,
		LogLevel: d.config.LogLevel,
	}
	exec, pluginClient, err := createExecutor(d.config.LogOutput, d.config, executorConfig)
	if err != nil {
		return nil, err
	}
	executorCtx := &executor.ExecutorContext{
		TaskEnv: ctx.TaskEnv,
		Driver:  "exec",
		AllocID: d.DriverContext.allocID,
		LogDir:  ctx.TaskDir.LogDir,
		TaskDir: ctx.TaskDir.Dir,
		Task:    task,
	}
	if err := exec.SetContext(executorCtx); err != nil {
		pluginClient.Kill()
		return nil, fmt.Errorf("failed to set executor context: %v", err)
	}

	execCmd := &executor.ExecCommand{
		Cmd:            command,
		Args:           driverConfig.Args,
		FSIsolation:    true,
		ResourceLimits: true,
		User:           getExecutorUser(task),
	}

	ps, err := exec.LaunchCmd(execCmd)
	if err != nil {
		pluginClient.Kill()
		return nil, err
	}

	d.logger.Printf("[DEBUG] driver.exec: started process via plugin with pid: %v", ps.Pid)

	// Return a driver handle
	maxKill := d.DriverContext.config.MaxKillTimeout
	h := &execHandle{
		pluginClient:    pluginClient,
		userPid:         ps.Pid,
		executor:        exec,
		isolationConfig: ps.IsolationConfig,
		killTimeout:     GetKillTimeout(task.KillTimeout, maxKill),
		maxKillTimeout:  maxKill,
		logger:          d.logger,
		version:         d.config.Version,
		doneCh:          make(chan struct{}),
		waitCh:          make(chan *dstructs.WaitResult, 1),
		taskDir:         ctx.TaskDir,
	}
	go h.run()
	return &StartResponse{Handle: h}, nil
}

func (d *ExecDriver) Cleanup(*ExecContext, *CreatedResources) error { return nil }

type execId struct {
	Version         string
	KillTimeout     time.Duration
	MaxKillTimeout  time.Duration
	UserPid         int
	IsolationConfig *dstructs.IsolationConfig
	PluginConfig    *PluginReattachConfig
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
		if id.IsolationConfig != nil {
			ePid := pluginConfig.Reattach.Pid
			if e := executor.ClientCleanup(id.IsolationConfig, ePid); e != nil {
				merrs.Errors = append(merrs.Errors, fmt.Errorf("destroying cgroup failed: %v", e))
			}
		}
		return nil, fmt.Errorf("error connecting to plugin: %v", merrs.ErrorOrNil())
	}

	ver, _ := exec.Version()
	d.logger.Printf("[DEBUG] driver.exec : version of executor: %v", ver.Version)
	// Return a driver handle
	h := &execHandle{
		pluginClient:    client,
		executor:        exec,
		userPid:         id.UserPid,
		isolationConfig: id.IsolationConfig,
		logger:          d.logger,
		version:         id.Version,
		killTimeout:     id.KillTimeout,
		maxKillTimeout:  id.MaxKillTimeout,
		doneCh:          make(chan struct{}),
		waitCh:          make(chan *dstructs.WaitResult, 1),
		taskDir:         ctx.TaskDir,
	}
	go h.run()
	return h, nil
}

func (h *execHandle) ID() string {
	id := execId{
		Version:         h.version,
		KillTimeout:     h.killTimeout,
		MaxKillTimeout:  h.maxKillTimeout,
		PluginConfig:    NewPluginReattachConfig(h.pluginClient.ReattachConfig()),
		UserPid:         h.userPid,
		IsolationConfig: h.isolationConfig,
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
	h.executor.UpdateTask(task)

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

func (h *execHandle) Kill() error {
	if err := h.executor.ShutDown(); err != nil {
		if h.pluginClient.Exited() {
			return nil
		}
		return fmt.Errorf("executor Shutdown failed: %v", err)
	}

	select {
	case <-h.doneCh:
	case <-time.After(h.killTimeout):
		if h.pluginClient.Exited() {
			break
		}
		if err := h.executor.Exit(); err != nil {
			return fmt.Errorf("executor Exit failed: %v", err)
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

	// If the exitcode is 0 and we had an error that means the plugin didn't
	// connect and doesn't know the state of the user process so we are killing
	// the user process so that when we create a new executor on restarting the
	// new user process doesn't have collisions with resources that the older
	// user pid might be holding onto.
	if ps.ExitCode == 0 && werr != nil {
		if h.isolationConfig != nil {
			ePid := h.pluginClient.ReattachConfig().Pid
			if e := executor.ClientCleanup(h.isolationConfig, ePid); e != nil {
				h.logger.Printf("[ERR] driver.exec: destroying resource container failed: %v", e)
			}
		}
	}

	// Exit the executor
	if err := h.executor.Exit(); err != nil {
		h.logger.Printf("[ERR] driver.exec: error destroying executor: %v", err)
	}
	h.pluginClient.Kill()

	// Send the results
	h.waitCh <- dstructs.NewWaitResult(ps.ExitCode, ps.Signal, werr)
	close(h.waitCh)
}
