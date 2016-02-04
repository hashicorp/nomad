package driver

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/plugins"
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
	executor     plugins.Executor
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
			filepath.Join(taskDir, allocdir.TaskLocal),
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
	pluginConfig := &plugin.ClientConfig{
		HandshakeConfig: plugins.HandshakeConfig,
		Plugins:         plugins.PluginMap,
		Cmd:             exec.Command(bin, "executor"),
		SyncStdout:      d.config.LogOutput,
		SyncStderr:      d.config.LogOutput,
	}

	executor, pluginClient, err := d.executor(pluginConfig)
	if err != nil {
		return nil, err
	}
	executorCtx := &plugins.ExecutorContext{
		TaskEnv:  d.taskEnv,
		AllocDir: ctx.AllocDir,
		Task:     task,
	}
	ps, err := executor.LaunchCmd(&plugins.ExecCommand{Cmd: command, Args: driverConfig.Args}, executorCtx)
	if err != nil {
		pluginClient.Kill()
		return nil, fmt.Errorf("error starting process via the plugin: %v", err)
	}
	d.logger.Printf("DIPTANU Started process via plugin: %#v", ps)

	// Return a driver handle
	h := &rawExecHandle{
		pluginClient: pluginClient,
		executor:     executor,
		userPid:      ps.Pid,
		killTimeout:  d.DriverContext.KillTimeout(task),
		logger:       d.logger,
		doneCh:       make(chan struct{}),
		waitCh:       make(chan *cstructs.WaitResult, 1),
	}
	go h.run()
	return h, nil
}
func (d *RawExecDriver) executor(config *plugin.ClientConfig) (plugins.Executor, *plugin.Client, error) {
	executorClient := plugin.NewClient(config)
	rpcClient, err := executorClient.Client()
	if err != nil {
		return nil, nil, fmt.Errorf("error creating rpc client for executor plugin: %v", err)
	}
	rpcClient.SyncStreams(d.config.LogOutput, d.config.LogOutput)

	raw, err := rpcClient.Dispense("executor")
	if err != nil {
		return nil, nil, fmt.Errorf("unable to dispense the executor plugin: %v", err)
	}
	executorPlugin := raw.(plugins.Executor)
	return executorPlugin, executorClient, nil
}

type rawExecId struct {
	KillTimeout  time.Duration
	PluginConfig *plugin.ReattachConfig
	UserPid      int
}

func (d *RawExecDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	id := &rawExecId{}
	if err := json.Unmarshal([]byte(handleID), id); err != nil {
		return nil, fmt.Errorf("Failed to parse handle '%s': %v", handleID, err)
	}

	bin, err := discover.NomadExecutable()
	if err != nil {
		return nil, fmt.Errorf("unable to find the nomad binary: %v", err)
	}

	pluginConfig := &plugin.ClientConfig{
		HandshakeConfig: plugins.HandshakeConfig,
		Plugins:         plugins.PluginMap,
		Cmd:             exec.Command(bin, "executor"),
		Reattach:        id.PluginConfig,
		SyncStdout:      d.config.LogOutput,
		SyncStderr:      d.config.LogOutput,
	}
	executor, client, err := d.executor(pluginConfig)
	if err != nil {
		return nil, fmt.Errorf("error connecting to plugin: %v", err)
	}

	// Return a driver handle
	h := &execHandle{
		pluginClient: client,
		executor:     executor,
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
		PluginConfig: h.pluginClient.ReattachConfig(),
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
		err := h.executor.Exit()
		return err
	}
}

func (h *rawExecHandle) run() {
	ps, err := h.executor.Wait()
	close(h.doneCh)
	h.waitCh <- &cstructs.WaitResult{ExitCode: ps.ExitCode, Signal: 0, Err: err}
	close(h.waitCh)
	h.pluginClient.Kill()
}
