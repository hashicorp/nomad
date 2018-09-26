package raw_exec

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/consul-template/signals"
	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/driver/executor"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/drivers/utils"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"golang.org/x/net/context"
)

const (
	// pluginName is the name of the plugin
	pluginName = "raw_exec"

	// fingerprintPeriod is the interval at which the driver will send fingerprint responses
	fingerprintPeriod = 30 * time.Second
)

var (
	pluginInfo = &base.PluginInfoResponse{
		Type:             base.PluginTypeDriver,
		PluginApiVersion: "0.0.1",
		PluginVersion:    "0.1.0",
		Name:             pluginName,
	}

	configSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"enabled": hclspec.NewDefault(
			hclspec.NewAttr("enabled", "bool", false),
			hclspec.NewLiteral("false"),
		),
		"no_cgroups": hclspec.NewDefault(
			hclspec.NewAttr("no_cgroups", "bool", false),
			hclspec.NewLiteral("false"),
		),
	})

	taskConfigSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"command": hclspec.NewAttr("command", "string", true),
		"args":    hclspec.NewAttr("command", "list(string)", false),
	})

	capabilities = &drivers.Capabilities{
		SendSignals: true,
		Exec:        true,
		FSIsolation: drivers.FSIsolationNone,
	}
)

// The RawExecDriver is a privileged version of the exec driver. It provides no
// resource isolation and just fork/execs. The Exec driver should be preferred
// and this should only be used when explicitly needed.
type RawExecDriver struct {
	*utils.Eventer
	config *Config
	tasks  *taskStore

	// fingerprintCh is a channel which other funcs can send fingerprints to
	// that will immediately be sent
	fingerprintCh chan *drivers.Fingerprint

	stopCh chan struct{}

	logger hclog.Logger
}

type Config struct {
	// NoCgroups tracks whether we should use a cgroup to manage the process
	// tree
	NoCgroups bool `codec:"no_cgroups"`

	// Enabled is set to true to enable the raw_exec driver
	Enabled bool `codec:"enabled"`
}

type TaskConfig struct {
	Command string   `codec:"command"`
	Args    []string `codec:"args"`
}

func NewRawExecDriver(logger hclog.Logger) drivers.DriverPlugin {
	stopCh := make(chan struct{})
	return &RawExecDriver{
		Eventer:       utils.NewEventer(stopCh),
		config:        &Config{},
		tasks:         newTaskStore(),
		fingerprintCh: make(chan *drivers.Fingerprint),
		stopCh:        stopCh,
		logger:        logger.Named(pluginName),
	}
}

func (r *RawExecDriver) PluginInfo() (*base.PluginInfoResponse, error) {
	return pluginInfo, nil
}

func (r *RawExecDriver) ConfigSchema() (*hclspec.Spec, error) {
	return configSpec, nil
}

func (r *RawExecDriver) SetConfig(data []byte) error {
	var config Config
	if err := base.MsgPackDecode(data, &config); err != nil {
		return err
	}

	r.config = &config
	go r.fingerprintNow()
	return nil
}

func (r *RawExecDriver) Shutdown(ctx context.Context) error {
	close(r.stopCh)
	return nil
}

func (r *RawExecDriver) TaskConfigSchema() (*hclspec.Spec, error) {
	return taskConfigSpec, nil
}

func (r *RawExecDriver) Capabilities() (*drivers.Capabilities, error) {
	return capabilities, nil
}

func (r *RawExecDriver) Fingerprint(ctx context.Context) (<-chan *drivers.Fingerprint, error) {
	ch := make(chan *drivers.Fingerprint)
	go r.fingerprint(ctx, ch)
	return ch, nil
}

func (r *RawExecDriver) fingerprint(ctx context.Context, ch chan *drivers.Fingerprint) {
	defer close(r.fingerprintCh)

	ticker := time.NewTimer(0)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ticker.Reset(fingerprintPeriod)
			go r.fingerprintNow()
		case f := <-r.fingerprintCh:
			ch <- f
		}
	}
}

func (r *RawExecDriver) fingerprintNow() {
	if r.fingerprintCh == nil {
		r.logger.Debug("fingerprint channel was nil, skipping fingerprint")
		return
	}
	var health drivers.HealthState
	var desc string
	attrs := map[string]string{}
	if r.config.Enabled {
		health = drivers.HealthStateHealthy
		desc = "raw_exec enabled"
		attrs["driver.raw_exec"] = "1"
	} else {
		health = drivers.HealthStateUndetected
		desc = "raw_exec disabled"
	}
	r.fingerprintCh <- &drivers.Fingerprint{
		Attributes:        map[string]string{},
		Health:            health,
		HealthDescription: desc,
	}
}

func (r *RawExecDriver) RecoverTask(handle *drivers.TaskHandle) error {
	var taskState RawExecTaskState

	err := handle.GetDriverState(&taskState)
	if err != nil {
		r.logger.Error("failed to recover task", "error", err, "task_id", handle.Config.ID)
		return err
	}

	plugRC, err := utils.ReattachConfigToGoPlugin(taskState.ReattachConfig)
	if err != nil {
		r.logger.Error("failed to recover task", "error", err, "task_id", handle.Config.ID)
		return err
	}

	pluginConfig := &plugin.ClientConfig{
		Reattach: plugRC,
	}

	exec, pluginClient, err := utils.CreateExecutorWithConfig(pluginConfig, os.Stderr)
	if err != nil {
		r.logger.Error("failed to recover task", "error", err, "task_id", handle.Config.ID)
		return err
	}

	h := &rawExecTaskHandle{
		exec:         exec,
		pid:          taskState.Pid,
		pluginClient: pluginClient,
		task:         taskState.TaskConfig,
		procState:    drivers.TaskStateRunning,
		startedAt:    taskState.StartedAt,
		exitResult:   &drivers.ExitResult{},
	}

	r.tasks.Set(taskState.TaskConfig.ID, h)

	go h.run()
	return nil
}

func (r *RawExecDriver) StartTask(cfg *drivers.TaskConfig) (*drivers.TaskHandle, error) {
	if _, ok := r.tasks.Get(cfg.ID); ok {
		return nil, fmt.Errorf("task with ID '%s' already started", cfg.ID)
	}

	var driverConfig TaskConfig
	if err := cfg.DecodeDriverConfig(&driverConfig); err != nil {
		return nil, err
	}

	handle := drivers.NewTaskHandle(pluginName)
	handle.Config = cfg

	pluginLogFile := filepath.Join(cfg.TaskDir().Dir, "executor.out")
	executorConfig := &dstructs.ExecutorConfig{
		LogFile:  pluginLogFile,
		LogLevel: "debug",
	}

	exec, pluginClient, err := utils.CreateExecutor(os.Stderr, hclog.Debug, 14000, 14512, executorConfig)
	if err != nil {
		return nil, err
	}

	execCmd := &executor.ExecCommand{
		Cmd:                driverConfig.Command,
		Args:               driverConfig.Args,
		Env:                cfg.EnvList(),
		User:               cfg.User,
		BasicProcessCgroup: !r.config.NoCgroups,
		TaskDir:            cfg.TaskDir().Dir,
		StdoutPath:         cfg.StdoutPath,
		StderrPath:         cfg.StderrPath,
	}

	ps, err := exec.Launch(execCmd)
	if err != nil {
		pluginClient.Kill()
		return nil, err
	}

	h := &rawExecTaskHandle{
		exec:         exec,
		pid:          ps.Pid,
		pluginClient: pluginClient,
		task:         cfg,
		procState:    drivers.TaskStateRunning,
		startedAt:    time.Now().Round(time.Millisecond),
		logger:       r.logger,
	}

	r.tasks.Set(cfg.ID, h)

	driverState := RawExecTaskState{
		ReattachConfig: utils.ReattachConfigFromGoPlugin(pluginClient.ReattachConfig()),
		Pid:            ps.Pid,
		TaskConfig:     cfg,
		StartedAt:      h.startedAt,
	}

	if err := handle.SetDriverState(&driverState); err != nil {
		r.logger.Error("failed to start task, error setting driver state", "error", err)
		exec.Shutdown("", 0)
		pluginClient.Kill()
		return nil, err
	}

	go h.run()
	return handle, nil
}

func (r *RawExecDriver) WaitTask(ctx context.Context, taskID string) (<-chan *drivers.ExitResult, error) {
	ch := make(chan *drivers.ExitResult)
	handle, ok := r.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}
	go r.handleWait(ctx, handle, ch)

	return ch, nil
}

func (r *RawExecDriver) handleWait(ctx context.Context, handle *rawExecTaskHandle, ch chan *drivers.ExitResult) {
	defer close(ch)
	var result *drivers.ExitResult
	ps, err := handle.exec.Wait()
	if err != nil {
		result = &drivers.ExitResult{
			Err: fmt.Errorf("executor: error waiting on process: %v", err),
		}
	} else {
		result = &drivers.ExitResult{
			ExitCode: ps.ExitCode,
			Signal:   ps.Signal,
		}
	}

	select {
	case <-ctx.Done():
		return
	case ch <- result:
	}
}

func (r *RawExecDriver) StopTask(taskID string, timeout time.Duration, signal string) error {
	handle, ok := r.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	if err := handle.exec.Shutdown(signal, timeout); err != nil {
		if handle.pluginClient.Exited() {
			return nil
		}
		return fmt.Errorf("executor Shutdown failed: %v", err)
	}

	return nil
}

func (r *RawExecDriver) DestroyTask(taskID string, force bool) error {
	handle, ok := r.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	if handle.IsRunning() && !force {
		return fmt.Errorf("cannot destroy running task")
	}

	if !handle.pluginClient.Exited() {
		if handle.IsRunning() {
			if err := handle.exec.Shutdown("", 0); err != nil {
				handle.logger.Error("destroying executor failed", "err", err)
			}
		}

		handle.pluginClient.Kill()
	}

	r.tasks.Delete(taskID)
	return nil
}

func (r *RawExecDriver) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	handle, ok := r.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	status := &drivers.TaskStatus{
		ID:          handle.task.ID,
		Name:        handle.task.Name,
		State:       handle.procState,
		StartedAt:   handle.startedAt,
		CompletedAt: handle.completedAt,
		ExitResult:  handle.exitResult,
		DriverAttributes: map[string]string{
			"pid": strconv.Itoa(handle.pid),
		},
		NetworkOverride: &drivers.NetworkOverride{},
	}

	return status, nil
}

func (r *RawExecDriver) TaskStats(taskID string) (*drivers.TaskStats, error) {
	handle, ok := r.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	stats, err := handle.exec.Stats()
	if err != nil {
		return nil, err
	}

	return &drivers.TaskStats{
		ID:                 handle.task.ID,
		Timestamp:          stats.Timestamp,
		AggResourceUsage:   stats.ResourceUsage,
		ResourceUsageByPid: stats.Pids,
	}, nil
}

func (r *RawExecDriver) SignalTask(taskID string, signal string) error {
	handle, ok := r.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	sig := os.Interrupt
	if s, ok := signals.SignalLookup[signal]; ok {
		sig = s
	}
	return handle.exec.Signal(sig)
}

func (r *RawExecDriver) ExecTask(taskID string, cmd []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
	if len(cmd) == 0 {
		return nil, fmt.Errorf("error cmd must have atleast one value")
	}
	handle, ok := r.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	args := []string{}
	if len(cmd) > 1 {
		args = cmd[1:]
	}

	out, exitCode, err := handle.exec.Exec(time.Now().Add(timeout), cmd[0], args)
	if err != nil {
		return nil, err
	}

	return &drivers.ExecTaskResult{
		Stdout: out,
		ExitResult: &drivers.ExitResult{
			ExitCode: exitCode,
		},
	}, nil
}

type taskStore struct {
	store map[string]*rawExecTaskHandle
	lock  sync.RWMutex
}

func newTaskStore() *taskStore {
	return &taskStore{store: map[string]*rawExecTaskHandle{}}
}

func (ts *taskStore) Set(id string, handle *rawExecTaskHandle) {
	ts.lock.Lock()
	defer ts.lock.Unlock()
	ts.store[id] = handle
}

func (ts *taskStore) Get(id string) (*rawExecTaskHandle, bool) {
	ts.lock.RLock()
	defer ts.lock.RUnlock()
	t, ok := ts.store[id]
	return t, ok
}

func (ts *taskStore) Delete(id string) {
	ts.lock.Lock()
	defer ts.lock.Unlock()
	delete(ts.store, id)
}

func (ts *taskStore) Range(f func(id string, handle *rawExecTaskHandle) bool) {
	ts.lock.RLock()
	defer ts.lock.RUnlock()
	for k, v := range ts.store {
		if f(k, v) {
			break
		}
	}
}

type RawExecTaskState struct {
	ReattachConfig *utils.ReattachConfig
	TaskConfig     *drivers.TaskConfig
	Pid            int
	StartedAt      time.Time
}

type rawExecTaskHandle struct {
	exec         executor.Executor
	pid          int
	pluginClient *plugin.Client
	task         *drivers.TaskConfig
	procState    drivers.TaskState
	startedAt    time.Time
	completedAt  time.Time
	exitResult   *drivers.ExitResult
	logger       hclog.Logger
}

func (h *rawExecTaskHandle) IsRunning() bool {
	return h.procState == drivers.TaskStateRunning
}

func (h *rawExecTaskHandle) run() {
	if h.exitResult == nil {
		h.exitResult = &drivers.ExitResult{}
	}

	ps, err := h.exec.Wait()
	if err != nil {
		h.exitResult.Err = err
		h.procState = drivers.TaskStateUnknown
		h.completedAt = time.Now()
		return
	}
	h.procState = drivers.TaskStateExited
	h.exitResult.ExitCode = ps.ExitCode
	h.exitResult.Signal = ps.Signal
	h.completedAt = ps.Time

	//h.pluginClient.Kill()
}
