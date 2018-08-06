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

	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/struct"
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
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers/raw-exec/proto"
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

	Tasks map[string]*taskHandle
}

// LogEventFn is a callback which allows Drivers to emit task events.
type LogEventFn func(message string, args ...interface{})

// taskHandle is returned from Start/Open as a handle to the PID
type taskHandle struct {
	id              string
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

type RawExecTaskConfig struct {
	Command string
	Args    []string
}

// NewRawExecDriver is used to create a new raw exec driver
func NewRawExecDriver(ctx *DriverContext) *RawExecDriver {
	return &RawExecDriver{
		DriverContext: *ctx,
		Tasks:         make(map[string]*taskHandle, 0),
	}
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

func (d *RawExecDriver) persistTask(h *taskHandle) {
	d.Tasks[h.id] = h
}

func (d *RawExecDriver) getTask(id string) (*taskHandle, error) {
	h := d.Tasks[id]
	if h == nil {
		return nil, fmt.Errorf("No task with this ID")
	}
	return h, nil
}

func (d *RawExecDriver) Start(ctx *proto.ExecContext, tInfo *proto.TaskInfo) (*proto.StartResponse, error) {
	execCtx := unmarshallExecContext(ctx)
	taskInfo, err := unmarshallTaskInfo(tInfo)
	if err != nil {
		return &proto.StartResponse{}, err
	}

	pluginLogFile := filepath.Join(execCtx.TaskDir.Dir, "executor.out")
	executorConfig := &ExecutorConfig{
		LogFile:  pluginLogFile,
		LogLevel: execCtx.TaskDir.LogLevel,
		MaxPort:  execCtx.MaxPort,
		MinPort:  execCtx.MinPort,
	}

	exec, pluginClient, err := createExecutor(execCtx.TaskDir.LogOutput, executorConfig, nil)
	if err != nil {
		return &proto.StartResponse{}, err
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
			NodeAttrs: execCtx.TaskEnv.NodeAttrs,
			EnvMap:    execCtx.TaskEnv.EnvMap,
		},
		Driver:  "raw_exec",
		Task:    task,
		TaskDir: execCtx.TaskDir.Dir,
		LogDir:  execCtx.TaskDir.LogDir,
	}
	if err := exec.SetContext(executorCtx); err != nil {
		pluginClient.Kill()
		return &proto.StartResponse{}, fmt.Errorf("failed to set executor context: %v", err)
	}

	// TODO taskKillSignal, err := getTaskKillSignal(taskInfo.KillSignal)
	taskKillSignal, err := getTaskKillSignal("")
	if err != nil {
		return &proto.StartResponse{}, err
	}

	execCmd := &executor.ExecCommand{
		Cmd:                taskInfo.Command,
		Args:               taskInfo.Config.Args,
		User:               taskInfo.User,
		TaskKillSignal:     taskKillSignal,
		BasicProcessCgroup: d.useCgroup,
	}
	ps, err := exec.LaunchCmd(execCmd)
	if err != nil {
		pluginClient.Kill()
		return &proto.StartResponse{}, err
	}
	// TODO d.logger.Printf("[DEBUG] driver.raw_exec: started process with pid: %v", ps.Pid)

	// Create a UUID for this task for communication across multiple RPC calls

	id := uuid.Generate()
	// Return a driver handle
	maxKill := execCtx.MaxKillTimeout
	h := &taskHandle{
		id:              id,
		pluginClient:    pluginClient,
		executor:        exec,
		isolationConfig: ps.IsolationConfig,
		userPid:         ps.Pid,
		killTimeout:     GetKillTimeout(task.KillTimeout, maxKill),
		maxKillTimeout:  maxKill,
		version:         execCtx.Version,
		// TODO logger:          d.logger,
		doneCh: make(chan struct{}),
		waitCh: make(chan *dstructs.WaitResult, 1),
		taskEnv: &env.TaskEnv{
			NodeAttrs: execCtx.TaskEnv.NodeAttrs,
			EnvMap:    execCtx.TaskEnv.EnvMap,
		},
		taskDir: &allocdir.TaskDir{
			Dir:    execCtx.TaskDir.Dir,
			LogDir: execCtx.TaskDir.LogDir,
		},
	}
	go h.run()

	d.persistTask(h)

	resp := &proto.StartResponse{}
	taskState := &proto.TaskState{
		TaskId:       h.id,
		ReattachInfo: marshallPluginReattachConfig(h.pluginClient.ReattachConfig()),
		KillTimeout: &duration.Duration{
			Seconds: int64(h.killTimeout.Seconds()),
			Nanos:   int32(h.killTimeout.Nanoseconds()),
		},
		MaxKillTimeout: &duration.Duration{
			Seconds: int64(h.maxKillTimeout.Seconds()),
			Nanos:   int32(h.maxKillTimeout.Nanoseconds()),
		},
		ReattachMeta: &structpb.Struct{
			//TODO(preetha): figure out what other useful stuff can be put in here
			Fields: map[string]*structpb.Value{
				"pid": &structpb.Value{Kind: &structpb.Value_NumberValue{float64(h.userPid)}},
			},
		},
		TaskDir:  execCtx.TaskDir.Dir,
		LogDir:   execCtx.TaskDir.LogDir,
		LogLevel: execCtx.LogLevel,
		MaxPort:  uint32(execCtx.MaxPort),
		MinPort:  uint32(execCtx.MinPort),
	}
	resp.TaskState = taskState

	return resp, nil
}

func (d *RawExecDriver) Restore(taskStates []*proto.TaskState) (*proto.RestoreResponse, error) {
	resp := &proto.RestoreResponse{}
	var toRestore []*proto.TaskState

	// Reconcile with the list of running tasks this driver already knows about
	for _, ts := range taskStates {
		_, ok := d.Tasks[ts.TaskId]
		if !ok {
			toRestore = append(toRestore, ts)
		}
	}

	var responses []*proto.TaskRestoreResponse
	// Reattach to tasks that this driver did not have in its state
	for _, ts := range toRestore {
		pluginReattachConfig := unMarshallPluginReattachConfig(ts.ReattachInfo)
		pluginLogFile := filepath.Join(ts.TaskDir, "executor.out")
		executorConfig := &ExecutorConfig{
			LogFile:  pluginLogFile,
			LogLevel: ts.LogLevel,
			MaxPort:  uint(ts.MaxPort),
			MinPort:  uint(ts.MinPort),
		}

		exec, pluginClient, err := createExecutor(os.Stdout, executorConfig, pluginReattachConfig)
		if err != nil {
			errorMsg := fmt.Errorf("error connecting to executor plugin:%v", err)
			responses = append(responses, &proto.TaskRestoreResponse{TaskId: ts.TaskId, ErrorMessage: errorMsg.Error()})
			continue
		}
		userPID := ts.ReattachMeta.Fields["pid"].GetNumberValue()
		pid := int(userPID)

		taskDir := &allocdir.TaskDir{
			Dir:    ts.TaskDir,
			LogDir: ts.LogDir,
		}

		// Return a driver handle
		h := &taskHandle{
			id:           ts.TaskId,
			pluginClient: pluginClient,
			executor:     exec,
			userPid:      pid,
			// TODO logger:          d.logger,
			killTimeout:    time.Duration(ts.KillTimeout.Seconds),
			maxKillTimeout: time.Duration(ts.MaxKillTimeout.Seconds),
			doneCh:         make(chan struct{}),
			waitCh:         make(chan *dstructs.WaitResult, 1),
			taskEnv:        &env.TaskEnv{},
			taskDir:        taskDir,
		}
		go h.run()
		d.persistTask(h)
		responses = append(responses, &proto.TaskRestoreResponse{TaskId: ts.TaskId})
	}
	resp.RestoreResults = responses
	return resp, nil
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
	h := &taskHandle{
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

func (h *taskHandle) ID() string {
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

func (d *RawExecDriver) Stop(ts *proto.TaskState) (*proto.StopResponse, error) {
	h, err := d.getTask(ts.TaskId)

	if err != nil {
		return &proto.StopResponse{}, err
	}

	if err := h.executor.ShutDown(); err != nil {
		if h.pluginClient.Exited() {
			return &proto.StopResponse{}, nil
		}
		return &proto.StopResponse{}, fmt.Errorf("executor Shutdown failed: %v", err)
	}

	select {
	case <-h.doneCh:
		return &proto.StopResponse{}, nil
	case <-time.After(h.killTimeout):
		if h.pluginClient.Exited() {
			return &proto.StopResponse{}, nil
		}
		if err := h.executor.Exit(); err != nil {
			return &proto.StopResponse{}, fmt.Errorf("executor Exit failed: %v", err)
		}

		return &proto.StopResponse{
			Pid: uint32(h.userPid),
		}, nil
	}
}

func (h *taskHandle) WaitCh() chan *dstructs.WaitResult {
	return h.waitCh
}

func (h *taskHandle) Update(task *structs.Task) error {
	// Store the updated kill timeout.
	h.killTimeout = GetKillTimeout(task.KillTimeout, h.maxKillTimeout)
	h.executor.UpdateTask(task)

	// Update is not possible
	return nil
}

func (h *taskHandle) Exec(ctx context.Context, cmd string, args []string) ([]byte, int, error) {
	return executor.ExecScript(ctx, h.taskDir.Dir, h.taskEnv, nil, cmd, args)
}

func (h *taskHandle) Signal(s os.Signal) error {
	return h.executor.Signal(s)
}

func (h *taskHandle) Kill() error {
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

func (h *taskHandle) Stats() (*cstructs.TaskResourceUsage, error) {
	return h.executor.Stats()
}

func (h *taskHandle) run() {
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
