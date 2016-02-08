package driver

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/executor"
	cstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/client/getter"
	"github.com/hashicorp/nomad/helper/discover"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
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
	pluginClient *plugin.Client
	userPid      int
	executor     executor.Executor
	killTimeout  time.Duration
	logger       *log.Logger
	waitCh       chan *cstructs.WaitResult
	doneCh       chan struct{}
}

// NewRawExecDriver is used to create a new raw exec driver
func NewRawExecDriver(ctx *DriverContext) Driver {
	return &RawExecDriver{DriverContext: *ctx}
}

func (d *RawExecDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// Check that the user has explicitly enabled this executor.
	enabled := cfg.ReadBoolDefault(rawExecConfigOption, false)

	if enabled {
		d.logger.Printf("[WARN] driver.raw_exec: raw exec is enabled. Only enable if needed")
		node.Attributes["driver.raw_exec"] = "1"
		return true, nil
	}

	return false, nil
}

func (d *RawExecDriver) Start(ctx *ExecContext, task *structs.Task) (DriverHandle, error) {
	var driverConfig ExecDriverConfig
	if err := mapstructure.WeakDecode(task.Config, &driverConfig); err != nil {
		return nil, err
	}
	// Get the tasks local directory.
	taskName := d.DriverContext.taskName
	taskDir, ok := ctx.AllocDir.TaskDirs[taskName]
	if !ok {
		return nil, fmt.Errorf("Could not find task directory for task: %v", d.DriverContext.taskName)
	}

	// Get the command to be ran
	command := driverConfig.Command
	if command == "" {
		return nil, fmt.Errorf("missing command for Raw Exec driver")
	}

	// Check if an artificat is specified and attempt to download it
	source, ok := task.Config["artifact_source"]
	if ok && source != "" {
		// Proceed to download an artifact to be executed.
		_, err := getter.GetArtifact(
			taskDir,
			driverConfig.ArtifactSource,
			driverConfig.Checksum,
			d.logger,
		)
		if err != nil {
			return nil, err
		}
	}

	bin, err := discover.NomadExecutable()
	if err != nil {
		return nil, fmt.Errorf("unable to find the nomad binary: %v", err)
	}
	pluginLogFile := filepath.Join(taskDir, fmt.Sprintf("%s-executor.out", task.Name))
	pluginConfig := &plugin.ClientConfig{
		Cmd: exec.Command(bin, "executor", pluginLogFile),
	}

	exec, pluginClient, err := createExecutor(pluginConfig, d.config.LogOutput, d.config)
	if err != nil {
		return nil, err
	}
	executorCtx := &executor.ExecutorContext{
		TaskEnv:       d.taskEnv,
		AllocDir:      ctx.AllocDir,
		TaskName:      task.Name,
		TaskResources: task.Resources,
	}
	ps, err := exec.LaunchCmd(&executor.ExecCommand{Cmd: command, Args: driverConfig.Args}, executorCtx)
	if err != nil {
		pluginClient.Kill()
		return nil, fmt.Errorf("error starting process via the plugin: %v", err)
	}
	d.logger.Printf("[DEBUG] driver.raw_exec: started process with pid: %v", ps.Pid)

	// Return a driver handle
	h := &rawExecHandle{
		pluginClient: pluginClient,
		executor:     exec,
		userPid:      ps.Pid,
		killTimeout:  d.DriverContext.KillTimeout(task),
		logger:       d.logger,
		doneCh:       make(chan struct{}),
		waitCh:       make(chan *cstructs.WaitResult, 1),
	}
	go h.run()
	return h, nil
}

type rawExecId struct {
	KillTimeout  time.Duration
	UserPid      int
	PluginConfig *ExecutorReattachConfig
}

func (d *RawExecDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	id := &rawExecId{}
	if err := json.Unmarshal([]byte(handleID), id); err != nil {
		return nil, fmt.Errorf("Failed to parse handle '%s': %v", handleID, err)
	}

	pluginConfig := &plugin.ClientConfig{
		Reattach: id.PluginConfig.PluginConfig(),
	}
	executor, pluginClient, err := createExecutor(pluginConfig, d.config.LogOutput, d.config)
	if err != nil {
		d.logger.Println("[ERROR] driver.raw_exec: error connecting to plugin so destroying plugin pid and user pid")
		if e := destroyPlugin(id.PluginConfig.Pid, id.UserPid); e != nil {
			d.logger.Printf("[ERROR] driver.raw_exec: error destroying plugin and userpid: %v", e)
		}
		return nil, fmt.Errorf("error connecting to plugin: %v", err)
	}

	// Return a driver handle
	h := &rawExecHandle{
		pluginClient: pluginClient,
		executor:     executor,
		userPid:      id.UserPid,
		logger:       d.logger,
		killTimeout:  id.KillTimeout,
		doneCh:       make(chan struct{}),
		waitCh:       make(chan *cstructs.WaitResult, 1),
	}
	go h.run()
	return h, nil
}

func (h *rawExecHandle) ID() string {
	id := rawExecId{
		KillTimeout:  h.killTimeout,
		PluginConfig: NewExecutorReattachConfig(h.pluginClient.ReattachConfig()),
		UserPid:      h.userPid,
	}

	data, err := json.Marshal(id)
	if err != nil {
		h.logger.Printf("[ERR] driver.raw_exec: failed to marshal ID to JSON: %s", err)
	}
	return string(data)
}

func (h *rawExecHandle) WaitCh() chan *cstructs.WaitResult {
	return h.waitCh
}

func (h *rawExecHandle) Update(task *structs.Task) error {
	// Store the updated kill timeout.
	h.killTimeout = task.KillTimeout

	// Update is not possible
	return nil
}

func (h *rawExecHandle) Kill() error {
	h.executor.ShutDown()
	select {
	case <-h.doneCh:
		return nil
	case <-time.After(h.killTimeout):
		return h.executor.Exit()
	}
}

func (h *rawExecHandle) run() {
	ps, err := h.executor.Wait()
	close(h.doneCh)
	h.waitCh <- &cstructs.WaitResult{ExitCode: ps.ExitCode, Signal: 0, Err: err}
	close(h.waitCh)
	h.pluginClient.Kill()
}
