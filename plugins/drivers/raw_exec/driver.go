package raw_exec

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/client/driver/executor"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/helper/fields"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/drivers/base"
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

	tasks sync.Map
}

type taskHandle struct {
	id     string
	config *base.TaskConfig
}

func NewRawExecDriver() base.Driver {
	return &RawExecDriver{}
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

	handle := base.NewTaskHandle(DriverName)
	handle.Config = config
	d.tasks.Store(config.ID, handle)

	// TODO: Move this struct to correct package
	var driverConfig driver.ExecDriverConfig
	if err := mapstructure.WeakDecode(handle.Config.DriverConfig, &driverConfig); err != nil {
		return nil, err
	}

	// Get the command to be ran
	command := driverConfig.Command
	if err := drivers.ValidateCommand(command, "args"); err != nil {
		return nil, err
	}

	pluginLogFile := filepath.Join(config.TaskDir().Dir, "executor.out")
	executorConfig := &dstructs.ExecutorConfig{
		LogFile:  pluginLogFile,
		LogLevel: "debug",
	}

	exec, pluginClient, err := drivers.CreateExecutor(os.Stderr, "debug", 14000, 14512, executorConfig)
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
	}

	d.tasks.Store(config.ID, h)

	handle.SetDriverState(RawExecTaskState{
		ReattachConfig:  pluginClient.ReattachConfig(),
		ExecutorContext: executorCtx,
		Pid:             ps.Pid,
	})

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
		rawTask, ok := d.tasks.Load(taskID)
		if !ok {
			ch <- &base.TaskResult{
				Err: fmt.Errorf("no task found for id: %s", taskID),
			}
			return
		}

		handle := rawTask.(*rawExecTaskHandle)
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
	return nil
}
func (d *RawExecDriver) DestroyTask(taskID string) {}
func (d *RawExecDriver) ListTasks(*base.ListTasksQuery) ([]*base.TaskSummary, error) {

	return nil, nil
}
func (d *RawExecDriver) InspectTask(taskID string) (*base.TaskStatus, error) {
	return nil, nil
}
func (d *RawExecDriver) TaskStats(taskID string) (*base.TaskStats, error) {
	return nil, nil
}
