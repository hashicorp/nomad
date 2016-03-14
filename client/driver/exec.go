package driver

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/executor"
	cstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/getter"
	"github.com/hashicorp/nomad/helper/discover"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

// ExecDriver fork/execs tasks using as many of the underlying OS's isolation
// features.
type ExecDriver struct {
	DriverContext
}

type ExecDriverConfig struct {
	ArtifactSource string   `mapstructure:"artifact_source"`
	Checksum       string   `mapstructure:"checksum"`
	Command        string   `mapstructure:"command"`
	Args           []string `mapstructure:"args"`
}

// execHandle is returned from Start/Open as a handle to the PID
type execHandle struct {
	pluginClient    *plugin.Client
	executor        executor.Executor
	isolationConfig *cstructs.IsolationConfig
	userPid         int
	allocDir        *allocdir.AllocDir
	killTimeout     time.Duration
	maxKillTimeout  time.Duration
	logger          *log.Logger
	waitCh          chan *cstructs.WaitResult
	doneCh          chan struct{}
	version         string
}

// NewExecDriver is used to create a new exec driver
func NewExecDriver(ctx *DriverContext) Driver {
	return &ExecDriver{DriverContext: *ctx}
}

func (d *ExecDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// Only enable if cgroups are available and we are root
	if _, ok := node.Attributes["unique.cgroup.mountpoint"]; !ok {
		d.logger.Printf("[DEBUG] driver.exec: cgroups unavailable, disabling")
		return false, nil
	} else if syscall.Geteuid() != 0 {
		d.logger.Printf("[DEBUG] driver.exec: must run as root user, disabling")
		return false, nil
	}

	node.Attributes["driver.exec"] = "1"
	return true, nil
}

func (d *ExecDriver) Periodic() (bool, time.Duration) {
	return true, 15 * time.Second
}

func (d *ExecDriver) Start(ctx *ExecContext, task *structs.Task) (DriverHandle, error) {
	var driverConfig ExecDriverConfig
	if err := mapstructure.WeakDecode(task.Config, &driverConfig); err != nil {
		return nil, err
	}
	// Get the command to be ran
	command := driverConfig.Command
	if err := validateCommand(command, "args"); err != nil {
		return nil, err
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
		TaskEnv:          d.taskEnv,
		AllocDir:         ctx.AllocDir,
		TaskName:         task.Name,
		TaskResources:    task.Resources,
		LogConfig:        task.LogConfig,
		ResourceLimits:   true,
		FSIsolation:      true,
		UnprivilegedUser: true,
	}
	ps, err := exec.LaunchCmd(&executor.ExecCommand{Cmd: command, Args: driverConfig.Args}, executorCtx)
	if err != nil {
		pluginClient.Kill()
		return nil, fmt.Errorf("error starting process via the plugin: %v", err)
	}
	d.logger.Printf("[DEBUG] driver.exec: started process via plugin with pid: %v", ps.Pid)

	// Return a driver handle
	maxKill := d.DriverContext.config.MaxKillTimeout
	h := &execHandle{
		pluginClient:    pluginClient,
		userPid:         ps.Pid,
		executor:        exec,
		allocDir:        ctx.AllocDir,
		isolationConfig: ps.IsolationConfig,
		killTimeout:     GetKillTimeout(task.KillTimeout, maxKill),
		maxKillTimeout:  maxKill,
		logger:          d.logger,
		version:         d.config.Version,
		doneCh:          make(chan struct{}),
		waitCh:          make(chan *cstructs.WaitResult, 1),
	}
	go h.run()
	return h, nil
}

type execId struct {
	Version         string
	KillTimeout     time.Duration
	MaxKillTimeout  time.Duration
	UserPid         int
	TaskDir         string
	AllocDir        *allocdir.AllocDir
	IsolationConfig *cstructs.IsolationConfig
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
	exec, client, err := createExecutor(pluginConfig, d.config.LogOutput, d.config)
	if err != nil {
		merrs := new(multierror.Error)
		merrs.Errors = append(merrs.Errors, err)
		d.logger.Println("[ERR] driver.exec: error connecting to plugin so destroying plugin pid and user pid")
		if e := destroyPlugin(id.PluginConfig.Pid, id.UserPid); e != nil {
			merrs.Errors = append(merrs.Errors, fmt.Errorf("error destroying plugin and userpid: %v", e))
		}
		if id.IsolationConfig != nil {
			if e := executor.DestroyCgroup(id.IsolationConfig.Cgroup); e != nil {
				merrs.Errors = append(merrs.Errors, fmt.Errorf("destroying cgroup failed: %v", e))
			}
		}
		if e := ctx.AllocDir.UnmountAll(); e != nil {
			merrs.Errors = append(merrs.Errors, e)
		}
		return nil, fmt.Errorf("error connecting to plugin: %v", merrs.ErrorOrNil())
	}

	// Return a driver handle
	h := &execHandle{
		pluginClient:    client,
		executor:        exec,
		userPid:         id.UserPid,
		allocDir:        id.AllocDir,
		isolationConfig: id.IsolationConfig,
		logger:          d.logger,
		version:         id.Version,
		killTimeout:     id.KillTimeout,
		maxKillTimeout:  id.MaxKillTimeout,
		doneCh:          make(chan struct{}),
		waitCh:          make(chan *cstructs.WaitResult, 1),
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
		AllocDir:        h.allocDir,
		IsolationConfig: h.isolationConfig,
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
	// Store the updated kill timeout.
	h.killTimeout = GetKillTimeout(task.KillTimeout, h.maxKillTimeout)
	h.executor.UpdateLogConfig(task.LogConfig)

	// Update is not possible
	return nil
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

func (h *execHandle) run() {
	ps, err := h.executor.Wait()
	close(h.doneCh)

	// If the exitcode is 0 and we had an error that means the plugin didn't
	// connect and doesn't know the state of the user process so we are killing
	// the user process so that when we create a new executor on restarting the
	// new user process doesn't have collisions with resources that the older
	// user pid might be holding onto.
	if ps.ExitCode == 0 && err != nil {
		if h.isolationConfig != nil {
			if e := executor.DestroyCgroup(h.isolationConfig.Cgroup); e != nil {
				h.logger.Printf("[ERR] driver.exec: destroying cgroup failed while killing cgroup: %v", e)
			}
		}
		if e := h.allocDir.UnmountAll(); e != nil {
			h.logger.Printf("[ERR] driver.exec: unmounting dev,proc and alloc dirs failed: %v", e)
		}
	}
	h.waitCh <- cstructs.NewWaitResult(ps.ExitCode, 0, err)
	close(h.waitCh)
	h.pluginClient.Kill()
}
