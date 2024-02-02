// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package exec2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"time"

	"github.com/hashicorp/go-hclog"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/drivers/exec2/resources"
	"github.com/hashicorp/nomad/drivers/exec2/shim"
	"github.com/hashicorp/nomad/drivers/exec2/task"
	"github.com/hashicorp/nomad/drivers/exec2/util"
	"github.com/hashicorp/nomad/drivers/shared/eventer"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/drivers/utils"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/shoenig/netlog"
	"golang.org/x/sys/unix"
	"oss.indeed.com/go/libtime"
)

type Plugin struct {
	// events is used to handle multiplexing of TaskEvent calls such that
	// an event can be broadcast to all callers
	events *eventer.Eventer

	// config is the plugin configuration set by the SetConfig RPC
	config *Config

	// driverConfig is the driver-client configuration from Nomad
	// driverConfig *base.ClientDriverConfig

	// tasks is the in-memory datastore mapping IDs to handles
	tasks task.Store

	// ctx is used to coordinate shutdown across subsystems
	ctx context.Context

	// cancel is used to shutdown the plugin and its subsystems
	cancel context.CancelFunc

	// logger will log to the Nomad agent
	logger hclog.Logger
}

func New(log hclog.Logger) drivers.DriverPlugin {
	ctx, cancel := context.WithCancel(context.Background())
	return &Plugin{
		ctx:    ctx,
		cancel: cancel,
		config: new(Config),
		tasks:  task.NewStore(),
		events: eventer.NewEventer(ctx, log.Named("exec2events")),
		logger: log.Named("exec2"),
	}
}

func (*Plugin) PluginInfo() (*base.PluginInfoResponse, error) {
	return info, nil
}

func (*Plugin) ConfigSchema() (*hclspec.Spec, error) {
	return driverConfigSpec, nil
}

func (p *Plugin) SetConfig(c *base.Config) error {
	var config Config
	if len(c.PluginConfig) > 0 {
		if err := base.MsgPackDecode(c.PluginConfig, &config); err != nil {
			return err
		}
	}
	p.config = &config
	// currently not much to validate on the plugin config, but if there was
	// that step would go here
	return nil
}

func (*Plugin) TaskConfigSchema() (*hclspec.Spec, error) {
	return taskConfigSpec, nil
}

func (*Plugin) Capabilities() (*drivers.Capabilities, error) {
	return capabilities, nil
}

func (p *Plugin) Fingerprint(ctx context.Context) (<-chan *drivers.Fingerprint, error) {
	ch := make(chan *drivers.Fingerprint)
	go p.fingerprint(ctx, ch)
	return ch, nil
}

func (p *Plugin) fingerprint(ctx context.Context, ch chan<- *drivers.Fingerprint) {
	defer close(ch)

	var timer, cancel = helper.NewSafeTimer(0)
	defer cancel()

	// fingerprint runs every 90 seconds
	const frequency = 90 * time.Second

	for {
		p.logger.Trace("(re)enter fingerprint loop")
		select {
		case <-ctx.Done():
			return
		case <-p.ctx.Done():
			return
		case <-timer.C:
			ch <- p.doFingerprint()
			timer.Reset(frequency)
		}
	}
}

func (p *Plugin) doFingerprint() *drivers.Fingerprint {
	// disable if non-root or non-linux systems
	if utils.IsLinuxOS() && !utils.IsUnixRoot() {
		return failure(drivers.HealthStateUndetected, drivers.DriverRequiresRootMessage)
	}

	// inspect nsenter binary
	nPath, nErr := exec.LookPath("nsenter")
	switch {
	case os.IsNotExist(nErr):
		return failure(drivers.HealthStateUndetected, "nsenter executable not found")
	case nErr != nil:
		return failure(drivers.HealthStateUnhealthy, "failed to find nsenter executable")
	case nPath == "":
		return failure(drivers.HealthStateUndetected, "nsenter executable does not exist")
	}

	// inspect unshare binary
	uPath, uErr := exec.LookPath("unshare")
	switch {
	case os.IsNotExist(uErr):
		return failure(drivers.HealthStateUndetected, "unshare executable not found")
	case uErr != nil:
		return failure(drivers.HealthStateUnhealthy, "failed to find unshare executable")
	case uPath == "":
		return failure(drivers.HealthStateUndetected, "unshare executable does not exist")
	}

	// create our fingerprint
	return &drivers.Fingerprint{
		Health:            drivers.HealthStateHealthy,
		HealthDescription: drivers.DriverHealthy,
		Attributes: map[string]*structs.Attribute{
			"driver.exec2.unveil.tasks":    structs.NewBoolAttribute(p.config.UnveilByTask),
			"driver.exec2.unveil.defaults": structs.NewBoolAttribute(p.config.UnveilDefaults),
		},
	}
}

func failure(state drivers.HealthState, desc string) *drivers.Fingerprint {
	return &drivers.Fingerprint{
		Health:            state,
		HealthDescription: desc,
	}
}

var (
	anonymousRe = regexp.MustCompile(`nomad:[\d]+`)
)

// StartTask will setup the environment for and then launch the actual unix
// process of the task. This information will be encoded into, stored as, and
// returned as a task handle.
func (p *Plugin) StartTask(config *drivers.TaskConfig) (*drivers.TaskHandle, *drivers.DriverNetwork, error) {
	if config.User == "" {
		// if no user is provided in task configuration, nomad should have
		// allocated an anonymous user and set it in the driver task config
		return nil, nil, errors.New("user must be set")
	}

	netlog.Yellow("Plugin", "user", config.User)

	// ensure we do not already have a handle for this task
	if _, exists := p.tasks.Get(config.ID); exists {
		p.logger.Error("task with id already started", "id", config.ID)
		return nil, nil, fmt.Errorf("task with ID %s already started", config.ID)
	}

	// create a handle for this task
	handle := drivers.NewTaskHandle(handleVersion)
	handle.Config = config

	// open descripters for the task's stdout and stderr
	stdout, stderr, err := open(config.StdoutPath, config.StderrPath)
	if err != nil {
		p.logger.Error("failed to open log files", "error", err)
		return nil, nil, fmt.Errorf("failed to open log file(s): %w", err)
	}

	// compute memory and memory_max values
	memory := uint64(config.Resources.NomadResources.Memory.MemoryMB) * 1024 * 1024
	memoryMax := uint64(config.Resources.NomadResources.Memory.MemoryMaxMB) * 1024 * 1024

	// compute cpu bandwidth value
	bandwidth, err := resources.Bandwidth(uint64(config.Resources.NomadResources.Cpu.CpuShares))
	if err != nil {
		p.logger.Error("failed to compute cpu bandwidth: %w", err)
		return nil, nil, fmt.Errorf("failed to compute cpu bandwidth: %w", err)
	}

	// get our assigned cpuset cores
	cpuset := config.Resources.LinuxResources.CpusetCpus
	p.logger.Trace("resources", "memory", memory, "memory_max", memoryMax, "compute", bandwidth, "cpuset", cpuset)

	// with cgroups v2 this is just the task cgroup
	cgroup := config.Resources.LinuxResources.CpusetCgroupPath

	// set the task execution environment
	env := &shim.Environment{
		Out:          stdout,
		Err:          stderr,
		Env:          config.Env,
		TaskDir:      config.TaskDir().Dir,
		User:         config.User,
		Cgroup:       cgroup,
		Net:          netns(config),
		Memory:       memory,
		MemoryMax:    memoryMax,
		CPUBandwidth: bandwidth,
	}

	// set the task execution runtime options
	opts, err := p.setOptions(config)
	if err != nil {
		p.logger.Error("failed to parse options: %v", err)
		return nil, nil, err
	}

	// what is about to happen
	p.logger.Info(
		"exec2 runner",
		"cmd", opts.Command,
		"args", opts.Arguments,
		"unveil_paths", opts.UnveilPaths,
		"unveil_defaults", opts.UnveilDefaults,
	)

	// create the runner and start it
	runner := shim.New(env, opts)
	if err = runner.Start(p.ctx); err != nil {
		return nil, nil, fmt.Errorf("failed to start task: %w", err)
	}

	// create and store a handle for the runner we just started
	h, started := task.NewHandle(runner, config)
	state := &task.State{
		PID:        runner.PID(),
		TaskConfig: config,
		StartedAt:  started,
	}
	if err = handle.SetDriverState(state); err != nil {
		return nil, nil, fmt.Errorf("failed to set driver state: %w", err)
	}
	p.tasks.Set(config.ID, h)

	return handle, nil, nil
}

// RecoverTask will re-create the in-memory state of a task from a TaskHandle
// coming from Nomad.
func (p *Plugin) RecoverTask(handle *drivers.TaskHandle) error {
	if handle == nil {
		return errors.New("failed to recover task, handle is nil")
	}

	p.logger.Info("recovering task", "id", handle.Config.ID)

	if _, exists := p.tasks.Get(handle.Config.ID); exists {
		return nil // nothing to do
	}

	var taskState task.State
	if err := handle.GetDriverState(&taskState); err != nil {
		return fmt.Errorf("failed to decode task state: %w", err)
	}

	taskState.TaskConfig = handle.Config.Copy()

	// with cgroups v2 this is just the task cgroup
	cgroup := taskState.TaskConfig.Resources.LinuxResources.CpusetCgroupPath

	// re-create the environment for pledge
	env := &shim.Environment{
		Out:     util.NullCloser(nil),
		Err:     util.NullCloser(nil),
		Env:     handle.Config.Env,
		TaskDir: handle.Config.TaskDir().Dir,
		User:    handle.Config.User,
		Cgroup:  cgroup,
	}

	// re-establish task handle by locating the unix process of the PID
	runner := shim.Recover(taskState.PID, env)
	recHandle := task.RecreateHandle(runner, taskState.TaskConfig, taskState.StartedAt)
	p.tasks.Set(taskState.TaskConfig.ID, recHandle)
	return nil
}

// WaitTask waits on the task to reach completion - whether by terminating
// gracefully and setting an exit code or by being rudely interrupted.
func (p *Plugin) WaitTask(ctx context.Context, taskID string) (<-chan *drivers.ExitResult, error) {
	p.logger.Info("waiting on task", "id", taskID)

	handle, exists := p.tasks.Get(taskID)
	if !exists {
		return nil, fmt.Errorf("task does not exist: %s", taskID)
	}

	ch := make(chan *drivers.ExitResult)
	go func() {
		handle.Block()
		result := handle.Status()
		ch <- result.ExitResult
	}()
	return ch, nil
}

// StopTask will issue the given signal to the task, followed by KILL if the
// process does not exit within the given timeout.
func (p *Plugin) StopTask(taskID string, timeout time.Duration, signal string) error {
	if signal == "" {
		signal = "sigterm"
	}

	p.logger.Debug("stop task", "id", taskID, "timeout", timeout, "signal", signal)

	h, exists := p.tasks.Get(taskID)
	if !exists {
		return nil
	}
	return h.Stop(signal, timeout)
}

// DestroyTask will stop the given task if necessary and remove its state from
// memory. If the task is currently running it will only be stopped and removed
// if the force option is set.
func (p *Plugin) DestroyTask(taskID string, force bool) error {
	p.logger.Debug("destroy task", "id", taskID, "force", force)

	h, exists := p.tasks.Get(taskID)
	if !exists {
		return nil
	}

	var err error
	if h.IsRunning() {
		switch force {
		case false:
			err = errors.New("cannot destroy running task")
		case true:
			err = h.Stop("sigabrt", 100*time.Millisecond)
		}
	}

	p.tasks.Del(taskID)
	return err
}

// InspectTask is not yet implemented.
func (*Plugin) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	// TODO
	return nil, errors.New("InspectTask is not implemented (yet)")
}

// TaskStats starts a background goroutine that produces TaskResourceUsage
// every interval and returns them on the returned channel.
func (p *Plugin) TaskStats(ctx context.Context, taskID string, interval time.Duration) (<-chan *drivers.TaskResourceUsage, error) {
	h, exists := p.tasks.Get(taskID)
	if !exists {
		return nil, nil
	}
	ch := make(chan *drivers.TaskResourceUsage)
	go p.stats(ctx, ch, interval, h)
	return ch, nil
}

// TaskEvents produces an empty chan of TaskEvents.
func (*Plugin) TaskEvents(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
	// currently the exec2 driver does not produce any task events
	// (an example usage would be like downloading an image, etc.)
	ch := make(chan *drivers.TaskEvent, 1)
	return ch, nil
}

// SignalTask will use the kill() syscall to send signal to the unix process
// of the task.
func (p *Plugin) SignalTask(taskID, signal string) error {
	if signal == "" {
		return errors.New("signal must be set")
	}
	h, exists := p.tasks.Get(taskID)
	if !exists {
		return nil
	}
	return h.Signal(signal)
}

// ExecTask is not yet implemented.
func (*Plugin) ExecTask(taskID string, cmd []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
	// TODO
	return nil, errors.New("ExecTask is not implemented (yet)")
}

func open(stdout, stderr string) (io.WriteCloser, io.WriteCloser, error) {
	a, err := os.OpenFile(stdout, unix.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		return nil, nil, err
	}
	b, err := os.OpenFile(stderr, unix.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		return nil, nil, err
	}
	return a, b, nil
}

// netns returns the filepath to the network namespace if the network
// isolation mode is set to bridge
func netns(c *drivers.TaskConfig) string {
	const none = ""
	switch {
	case c == nil:
		return none
	case c.NetworkIsolation == nil:
		return none
	case c.NetworkIsolation.Mode == drivers.NetIsolationModeGroup:
		return c.NetworkIsolation.Path
	default:
		return none
	}
}

func (p *Plugin) stats(ctx context.Context, ch chan<- *drivers.TaskResourceUsage, interval time.Duration, h *task.Handle) {
	defer close(ch)
	ticks, stop := libtime.SafeTimer(interval)
	defer stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks.C:
			ticks.Reset(interval)
		}

		usage := h.Stats()

		ch <- &drivers.TaskResourceUsage{
			ResourceUsage: &cstructs.ResourceUsage{
				MemoryStats: &cstructs.MemoryStats{
					Cache:    usage.Cache,
					Swap:     usage.Swap,
					Usage:    usage.Memory,
					Measured: []string{"Cache", "Swap", "Usage"},
				},
				CpuStats: &cstructs.CpuStats{
					UserMode:         float64(usage.User),
					SystemMode:       float64(usage.System),
					Percent:          float64(usage.Percent),
					TotalTicks:       float64(usage.Ticks),
					ThrottledPeriods: 0,
					ThrottledTime:    0,
					Measured:         []string{"System Mode", "User Mode", "Percent"},
				},
			},
			Timestamp: time.Now().UTC().UnixNano(),
			Pids:      nil,
		}
	}
}

func (p *Plugin) setOptions(driverTaskConfig *drivers.TaskConfig) (*shim.Options, error) {
	var taskConfig TaskConfig
	if err := driverTaskConfig.DecodeDriverConfig(&taskConfig); err != nil {
		return nil, fmt.Errorf("failed to decode driver task config: %w", err)
	}

	unveil := slices.Clone(p.config.UnveilPaths)

	if len(taskConfig.Unveil) == 0 {
		// by default, enable all permissions in the task and alloc directories
		unveil = append(unveil, "rwxc:"+driverTaskConfig.Env["NOMAD_TASK_DIR"])
		unveil = append(unveil, "rwxc:"+driverTaskConfig.Env["NOMAD_ALLOC_DIR"])
	} else if len(taskConfig.Unveil) > 0 {
		if !p.config.UnveilByTask {
			// if task.config.unveil is set, the plugin config must allow it
			return nil, fmt.Errorf("task set unveil paths but driver config does not allow this")
		}
		// use the user specified unveil paths from task.config.unveil
		unveil = append(unveil, taskConfig.Unveil...)
	}

	return &shim.Options{
		Command:        taskConfig.Command,
		Arguments:      taskConfig.Args,
		UnveilPaths:    unveil,
		UnveilDefaults: p.config.UnveilDefaults,
	}, nil
}
