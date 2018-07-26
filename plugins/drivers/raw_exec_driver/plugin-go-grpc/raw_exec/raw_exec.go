package raw_exec

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/client/driver/executor"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/fingerprint"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/fields"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers/raw_exec_driver/proto"
)

const (
	// rawExecEnableOption is the option that enables this driver in the Config.Options map.
	rawExecEnableOption = "driver.raw_exec.enable"

	// rawExecNoCgroupOption forces no cgroups.
	rawExecNoCgroupOption = "driver.raw_exec.no_cgroups"

	// The key populated in Node Attributes to indicate presence of the Raw Exec
	// driver
	rawExecDriverAttr = "driver.raw_exec"
)

// The RawExecDriver is a privileged version of the exec driver. It provides no
// resource isolation and just fork/execs. The Exec driver should be preferred
// and this should only be used when explicitly needed.
type RawExecDriver struct {
	DriverContext DriverContext
	fingerprint.StaticFingerprinter

	// useCgroup tracks whether we should use a cgroup to manage the process
	// tree
	useCgroup bool
}

// LogEventFn is a callback which allows Drivers to emit task events.
type LogEventFn func(message string, args ...interface{})

// rawExecHandle is returned from Start/Open as a handle to the PID
type rawExecHandle struct {
	version         string
	pluginClient    *plugin.Client
	userPid         int
	executor        executor.Executor
	isolationConfig *dstructs.IsolationConfig
	killTimeout     time.Duration
	maxKillTimeout  time.Duration
	logger          *log.Logger
	waitCh          chan *dstructs.WaitResult
	doneCh          chan struct{}
	taskEnv         *env.TaskEnv
	taskDir         *allocdir.TaskDir
}

// NewRawExecDriver is used to create a new raw exec driver
func NewRawExecDriver(ctx *DriverContext) *RawExecDriver {
	return &RawExecDriver{DriverContext: *ctx}
}

// Validate is used to validate the driver configuration
func (d *RawExecDriver) Validate(config map[string]interface{}) error {
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

func (d *RawExecDriver) Abilities() driver.DriverAbilities {
	return driver.DriverAbilities{
		SendSignals: true,
		Exec:        true,
	}
}

func (d *RawExecDriver) FSIsolation() cstructs.FSIsolation {
	return cstructs.FSIsolationNone
}

func (d *RawExecDriver) Fingerprint(req *cstructs.FingerprintRequest, resp *cstructs.FingerprintResponse) error {
	// Check that the user has explicitly enabled this executor.
	enabled := req.Config.ReadBoolDefault(rawExecEnableOption, false)

	if enabled || req.Config.DevMode {
		// TODO d.logger.Printf("[WARN] driver.raw_exec: raw exec is enabled. Only enable if needed")
		resp.AddAttribute(rawExecDriverAttr, "1")
		resp.Detected = true
		return nil
	}

	resp.RemoveAttribute(rawExecDriverAttr)
	return nil
}

func (d *RawExecDriver) Prestart(*driver.ExecContext, *structs.Task) (*driver.PrestartResponse, error) {
	// If we are on linux, running as root, cgroups are mounted, and cgroups
	// aren't disabled by the operator use cgroups for pid management.
	forceDisable := d.DriverContext.Config.ReadBoolDefault(rawExecNoCgroupOption, false)
	if !forceDisable && runtime.GOOS == "linux" &&
		syscall.Geteuid() == 0 && cgroupsMounted(d.DriverContext.node) {
		d.useCgroup = true
	}

	return nil, nil
}

// Shim for Start method, this needs to be cleaned up with methods extracted
// and all extra types need to be added to the protobuf and not be hardcoded here.
func (d *RawExecDriver) NewStart(ctx *proto.ExecContext, tInfo *proto.TaskInfo) (*proto.StartResponse, error) {
	execCtx := &ExecContext{
		TaskEnv: &TaskEnv{},
		TaskDir: &TaskDir{
			Dir:       ctx.TaskDir.Directory,
			LogDir:    ctx.TaskDir.LogDir,
			LogLevel:  ctx.TaskDir.LogLevel,
			LogOutput: os.Stdout,
		},
		MaxPort:        5000,
		MinPort:        2000,
		MaxKillTimeout: time.Duration(5),
		Version:        "1.0", // TODO was d.DriverContext.Config.Version.VersionNumber()
	}
	taskInfo := &TaskInfo{
		Resources: &Resources{
			CPU:      int(tInfo.Resources.Cpu),
			MemoryMB: int(tInfo.Resources.MemoryMb),
			DiskMB:   int(tInfo.Resources.DiskMb),
		},
		LogConfig: &LogConfig{
			MaxFiles:      int(tInfo.LogConfig.MaxFiles),
			MaxFileSizeMB: int(tInfo.LogConfig.MaxFileSizeMb),
		},
		Name: "taskName",
		Config: &Config{
			Command: "echo",
			Args:    []string{"hello world"},
		},
	}

	startResp, err := d.Start(execCtx, taskInfo)
	if err != nil || startResp == nil || startResp.Handle == nil {
		return &proto.StartResponse{}, err
	}
	resp := &proto.StartResponse{
		TaskId: startResp.Handle.ID(),
	}
	return resp, nil
}

func (d *RawExecDriver) Start(ctx *ExecContext, taskInfo *TaskInfo) (*driver.StartResponse, error) {
	command := taskInfo.Config.Command
	if err := validateCommand(command, "args"); err != nil {
		return nil, err
	}

	pluginLogFile := filepath.Join(ctx.TaskDir.Dir, "executor.out")
	executorConfig := &ExecutorConfig{
		LogFile:  pluginLogFile,
		LogLevel: ctx.TaskDir.LogLevel,
		MaxPort:  ctx.MaxPort,
		MinPort:  ctx.MinPort,
	}

	exec, pluginClient, err := createExecutor2(ctx.TaskDir.LogOutput, executorConfig)
	if err != nil {
		return nil, err
	}
	task := &structs.Task{
		Name:   taskInfo.Name,
		Driver: "raw_exec",
		User:   taskInfo.User,
		Config: map[string]interface{}{
			"command": taskInfo.Command,
			"args":    taskInfo.Args,
		},
		Resources: &structs.Resources{
			CPU:      taskInfo.Resources.CPU,
			MemoryMB: taskInfo.Resources.MemoryMB,
			DiskMB:   taskInfo.Resources.DiskMB,
			IOPS:     taskInfo.Resources.IOPS,
			Networks: structs.Networks{}, // TODO
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      taskInfo.LogConfig.MaxFiles,
			MaxFileSizeMB: taskInfo.LogConfig.MaxFileSizeMB,
		},
	}
	executorCtx := &executor.ExecutorContext{
		TaskEnv: &env.TaskEnv{
			NodeAttrs: ctx.TaskEnv.NodeAttrs,
			EnvMap:    ctx.TaskEnv.EnvMap,
		},
		Driver:  "raw_exec",
		Task:    task,
		TaskDir: ctx.TaskDir.Dir,
		LogDir:  ctx.TaskDir.LogDir,
	}
	if err := exec.SetContext(executorCtx); err != nil {
		pluginClient.Kill()
		return nil, fmt.Errorf("failed to set executor context: %v", err)
	}

	// TODO taskKillSignal, err := getTaskKillSignal(taskInfo.KillSignal)
	taskKillSignal, err := getTaskKillSignal("")
	if err != nil {
		return nil, err
	}

	execCmd := &executor.ExecCommand{
		Cmd:                command,
		Args:               taskInfo.Config.Args,
		User:               taskInfo.User,
		TaskKillSignal:     taskKillSignal,
		BasicProcessCgroup: d.useCgroup,
	}
	ps, err := exec.LaunchCmd(execCmd)
	if err != nil {
		pluginClient.Kill()
		return nil, err
	}
	// TODO d.logger.Printf("[DEBUG] driver.raw_exec: started process with pid: %v", ps.Pid)

	// Return a driver handle
	maxKill := ctx.MaxKillTimeout
	h := &rawExecHandle{
		pluginClient:    pluginClient,
		executor:        exec,
		isolationConfig: ps.IsolationConfig,
		userPid:         ps.Pid,
		killTimeout:     GetKillTimeout(task.KillTimeout, maxKill),
		maxKillTimeout:  maxKill,
		version:         ctx.Version,
		// TODO logger:          d.logger,
		doneCh: make(chan struct{}),
		waitCh: make(chan *dstructs.WaitResult, 1),
		taskEnv: &env.TaskEnv{
			NodeAttrs: ctx.TaskEnv.NodeAttrs,
			EnvMap:    ctx.TaskEnv.EnvMap,
		},
		taskDir: &allocdir.TaskDir{
			Dir:    ctx.TaskDir.Dir,
			LogDir: ctx.TaskDir.LogDir,
		},
	}
	go h.run()
	return &driver.StartResponse{Handle: h}, nil
}

func (d *RawExecDriver) Cleanup(*driver.ExecContext, *driver.CreatedResources) error { return nil }

type rawExecId struct {
	Version         string
	KillTimeout     time.Duration
	MaxKillTimeout  time.Duration
	UserPid         int
	PluginConfig    *driver.PluginReattachConfig
	IsolationConfig *dstructs.IsolationConfig
}

func (d *RawExecDriver) Open(ctx *driver.ExecContext, handleID string) (driver.DriverHandle, error) {
	id := &rawExecId{}
	if err := json.Unmarshal([]byte(handleID), id); err != nil {
		return nil, fmt.Errorf("Failed to parse handle '%s': %v", handleID, err)
	}

	pluginConfig := &plugin.ClientConfig{
		Reattach: id.PluginConfig.PluginConfig(),
	}
	exec, pluginClient, err := createExecutorWithConfig(pluginConfig, d.DriverContext.Config.LogOutput)
	if err != nil {
		merrs := new(multierror.Error)
		merrs.Errors = append(merrs.Errors, err)
		// TODO d.logger.Println("[ERR] driver.raw_exec: error connecting to plugin so destroying plugin pid and user pid")
		if e := destroyPlugin(id.PluginConfig.Pid, id.UserPid); e != nil {
			merrs.Errors = append(merrs.Errors, fmt.Errorf("error destroying plugin and userpid: %v", e))
		}
		if id.IsolationConfig != nil {
			ePid := pluginConfig.Reattach.Pid
			if e := executor.ClientCleanup(id.IsolationConfig, ePid); e != nil {
				merrs.Errors = append(merrs.Errors, fmt.Errorf("destroying resource container failed: %v", e))
			}
		}
		return nil, fmt.Errorf("error connecting to plugin: %v", merrs.ErrorOrNil())
	}

	// TODO ver, _ := exec.Version()
	// TODO d.logger.Printf("[DEBUG] driver.raw_exec: version of executor: %v", ver.Version)

	// Return a driver handle
	h := &rawExecHandle{
		pluginClient:    pluginClient,
		executor:        exec,
		userPid:         id.UserPid,
		isolationConfig: id.IsolationConfig,
		// TODO logger:          d.logger,
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
		Version:         h.version,
		KillTimeout:     h.killTimeout,
		MaxKillTimeout:  h.maxKillTimeout,
		PluginConfig:    driver.NewPluginReattachConfig(h.pluginClient.ReattachConfig()),
		UserPid:         h.userPid,
		IsolationConfig: h.isolationConfig,
	}

	data, err := json.Marshal(id)
	if err != nil {
		// TODO h.logger.Printf("[ERR] driver.raw_exec: failed to marshal ID to JSON: %s", err)
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
		if h.isolationConfig != nil {
			ePid := h.pluginClient.ReattachConfig().Pid
			if e := executor.ClientCleanup(h.isolationConfig, ePid); e != nil {
				// TODO h.logger.Printf("[ERR] driver.raw_exec: destroying resource container failed: %v", e)
			}
		} else {
			if e := killProcess(h.userPid); e != nil {
				// TODO h.logger.Printf("[ERR] driver.raw_exec: error killing user process: %v", e)
			}
		}
	}

	// Exit the executor
	if err := h.executor.Exit(); err != nil {
		// TODO h.logger.Printf("[ERR] driver.raw_exec: error killing executor: %v", err)
	}
	h.pluginClient.Kill()

	// Send the results
	h.waitCh <- &dstructs.WaitResult{ExitCode: ps.ExitCode, Signal: ps.Signal, Err: werr}
	close(h.waitCh)
}
