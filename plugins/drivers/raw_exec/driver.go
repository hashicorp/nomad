package raw_exec

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/client/driver/executor"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/helper/fields"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/plugins/drivers/base"
	"github.com/hashicorp/nomad/plugins/drivers/utils"
	"github.com/mitchellh/mapstructure"
)

const DriverName = "raw_exec"

// The RawExecDriver is a privileged version of the exec driver. It provides no
// resource isolation and just fork/execs. The Exec driver should be preferred
// and this should only be used when explicitly needed.
type RawExecDriver struct {

	// useCgroup tracks whether we should use a cgroup to manage the process
	// tree
	useCgroup bool

	tasks *taskStore
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

func NewRawExecDriver() base.Driver {
	return &RawExecDriver{tasks: newTaskStore()}
}

func (d *RawExecDriver) Fingerprint() *base.Fingerprint {
	return nil
}

func (d *RawExecDriver) RecoverTask(*base.TaskHandle) error {
	return nil
}

func (d *RawExecDriver) StartTask(config *base.TaskConfig) (*base.TaskHandle, error) {
	err := d.Validate(config.DriverConfig)
	if err != nil {
		return nil, err
	}

	if config.ID == "" {
		config.ID = uuid.Generate()
	} else {
		if _, ok := d.tasks.Get(config.ID); ok {
			return nil, fmt.Errorf("task with id '%s' already exists", config.ID)
		}
	}

	handle := base.NewTaskHandle(DriverName)
	handle.Config = config

	// TODO: Move this struct to correct package
	var driverConfig driver.ExecDriverConfig
	if err := mapstructure.WeakDecode(handle.Config.DriverConfig, &driverConfig); err != nil {
		return nil, err
	}

	// Get the command to be ran
	command := driverConfig.Command
	if err := utils.ValidateCommand(command, "args"); err != nil {
		return nil, err
	}

	pluginLogFile := filepath.Join(config.TaskDir().Dir, "executor.out")
	executorConfig := &dstructs.ExecutorConfig{
		LogFile:  pluginLogFile,
		LogLevel: "debug",
	}

	exec, pluginClient, err := utils.CreateExecutor(os.Stderr, "debug", 14000, 14512, executorConfig)
	if err != nil {
		return nil, err
	}
	executorCtx := &executor.ExecutorContext{
		Driver:  DriverName,
		TaskDir: config.TaskDir().Dir,
		LogConfig: &executor.LogConfig{
			LogDir:        config.TaskDir().LogDir,
			StdoutLogFile: fmt.Sprintf("%v.stdout", config.Name),
			StderrLogFile: fmt.Sprintf("%v.stderr", config.Name),
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Env:       config.EnvList(),
		Resources: &executor.Resources{},
	}
	if err := exec.SetContext(executorCtx); err != nil {
		pluginClient.Kill()
		return nil, fmt.Errorf("failed to set executor context: %v", err)
	}

	execCmd := &executor.ExecCommand{
		Cmd:                driverConfig.Command,
		Args:               driverConfig.Args,
		User:               config.User,
		TaskKillSignal:     os.Interrupt,
		BasicProcessCgroup: d.useCgroup,
	}

	ps, err := exec.LaunchCmd(execCmd)
	if err != nil {
		pluginClient.Kill()
		return nil, err
	}

	h := &rawExecTaskHandle{
		ps:           ps,
		exec:         exec,
		pluginClient: pluginClient,
		task:         config,
		startTime:    time.Now(),
		procState:    base.TaskStateRunning,
		doneCh:       make(chan struct{}),
	}

	d.tasks.Set(config.ID, h)

	handle.SetDriverState(RawExecTaskState{
		ReattachConfig:  pluginClient.ReattachConfig(),
		ExecutorContext: executorCtx,
		Pid:             ps.Pid,
	})

	go h.run()

	return handle, nil
}

type RawExecTaskState struct {
	ReattachConfig  *plugin.ReattachConfig
	ExecutorContext *executor.ExecutorContext
	Pid             int
}

type rawExecTaskHandle struct {
	ps           *executor.ProcessState
	exec         executor.Executor
	pluginClient *plugin.Client
	task         *base.TaskConfig
	startTime    time.Time
	procState    base.TaskState
	logger       hclog.Logger
	doneCh       chan struct{}
}

func (h *rawExecTaskHandle) run() {
	h.exec.Wait()
	h.procState = base.TaskStateDead
}

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

func (d *RawExecDriver) WaitTask(taskID string) chan *base.TaskResult {
	ch := make(chan *base.TaskResult)
	go func() {
		defer close(ch)
		handle, ok := d.tasks.Get(taskID)
		if !ok {
			ch <- &base.TaskResult{
				Err: fmt.Errorf("no task found for id: %s", taskID),
			}
			return
		}

		ps, err := handle.exec.Wait()
		if err != nil {
			ch <- &base.TaskResult{
				Err: fmt.Errorf("executor: error waiting on process: %v", err),
			}
			return
		}
		ch <- &base.TaskResult{
			ExitCode: ps.ExitCode,
			Signal:   ps.Signal,
		}
	}()
	return ch
}

func (d *RawExecDriver) StopTask(taskID string, timeout time.Duration, signal string) error {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return base.ErrTaskNotFound
	}

	//TODO executor only supports shutting down with the initial signal provided
	if err := handle.exec.ShutDown(); err != nil {
		if handle.pluginClient.Exited() {
			return nil
		}
		return fmt.Errorf("executor Shutdown failed: %v", err)
	}

	select {
	case <-d.WaitTask(taskID):
		return nil
	case <-time.After(timeout):
		if handle.pluginClient.Exited() {
			return nil
		}
		if err := handle.exec.Exit(); err != nil {
			return fmt.Errorf("executor Exit failed: %v", err)
		}
		return nil
	}
}
func (d *RawExecDriver) DestroyTask(taskID string) {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return
	}

	if !handle.pluginClient.Exited() {
		if err := handle.exec.Exit(); err != nil {
			handle.logger.Error("killing executor failed", "err", err)
		}
	}

	handle.pluginClient.Kill()
	d.tasks.Delete(taskID)

}

func (d *RawExecDriver) ListTasks(*base.ListTasksQuery) ([]*base.TaskSummary, error) {
	//TODO ignore query for now

	tasks := []*base.TaskSummary{}
	rangeF := func(id string, handle *rawExecTaskHandle) bool {
		tasks = append(tasks, &base.TaskSummary{
			ID:        id,
			Name:      handle.task.Name,
			State:     string(handle.procState),
			CreatedAt: handle.startTime,
		})
		return false
	}
	d.tasks.Range(rangeF)

	return tasks, nil
}
func (d *RawExecDriver) InspectTask(taskID string) (*base.TaskStatus, error) {
	return nil, nil
}
func (d *RawExecDriver) TaskStats(taskID string) (*base.TaskStats, error) {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, base.ErrTaskNotFound
	}

	eStats, err := handle.exec.Stats()
	if err != nil {
		return nil, err
	}

	stats := &base.TaskStats{
		ID:        handle.task.ID,
		Timestamp: eStats.Timestamp,
		AggResourceUsage: base.ResourceUsage{
			CPU: base.CPUUsage{
				SystemMode:       eStats.ResourceUsage.CpuStats.SystemMode,
				UserMode:         eStats.ResourceUsage.CpuStats.UserMode,
				TotalTicks:       eStats.ResourceUsage.CpuStats.TotalTicks,
				ThrottledPeriods: eStats.ResourceUsage.CpuStats.ThrottledPeriods,
				ThrottledTime:    eStats.ResourceUsage.CpuStats.ThrottledTime,
				Percent:          eStats.ResourceUsage.CpuStats.Percent,
				Measured:         eStats.ResourceUsage.CpuStats.Measured,
			},
			Memory: base.MemoryUsage{
				RSS:            eStats.ResourceUsage.MemoryStats.RSS,
				Cache:          eStats.ResourceUsage.MemoryStats.Cache,
				Swap:           eStats.ResourceUsage.MemoryStats.Swap,
				MaxUsage:       eStats.ResourceUsage.MemoryStats.MaxUsage,
				KernelUsage:    eStats.ResourceUsage.MemoryStats.KernelUsage,
				KernelMaxUsage: eStats.ResourceUsage.MemoryStats.KernelMaxUsage,
				Measured:       eStats.ResourceUsage.MemoryStats.Measured,
			},
		},
		ResourceUsageByPid: map[string]*base.ResourceUsage{},
	}

	for pid, usage := range eStats.Pids {
		stats.ResourceUsageByPid[pid] = &base.ResourceUsage{
			CPU: base.CPUUsage{
				SystemMode:       usage.CpuStats.SystemMode,
				UserMode:         usage.CpuStats.UserMode,
				TotalTicks:       usage.CpuStats.TotalTicks,
				ThrottledPeriods: usage.CpuStats.ThrottledPeriods,
				ThrottledTime:    usage.CpuStats.ThrottledTime,
				Percent:          usage.CpuStats.Percent,
				Measured:         usage.CpuStats.Measured,
			},
			Memory: base.MemoryUsage{
				RSS:            usage.MemoryStats.RSS,
				Cache:          usage.MemoryStats.Cache,
				Swap:           usage.MemoryStats.Swap,
				MaxUsage:       usage.MemoryStats.MaxUsage,
				KernelUsage:    usage.MemoryStats.KernelUsage,
				KernelMaxUsage: usage.MemoryStats.KernelMaxUsage,
				Measured:       usage.MemoryStats.Measured,
			},
		}
	}

	return stats, nil
}
