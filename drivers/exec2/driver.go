// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package exec2

import (
	"context"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/drivers/exec2/task"
	"github.com/hashicorp/nomad/drivers/shared/eventer"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/drivers/utils"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/hashicorp/nomad/plugins/shared/structs"
	"golang.org/x/sys/unix"
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

	// users looks up system users
	// users util.Users // TODO

	// logger will log to the Nomad agent
	logger hclog.Logger
}

func New(log hclog.Logger) drivers.DriverPlugin {
	ctx, cancel := context.WithCancel(context.Background())
	return &Plugin{
		ctx:    ctx,
		cancel: cancel,
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

	// TODO: validation on plugin configuration
	// currently there is no configuration, so yeah
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
			// TODO: any attributes to add?
			"driver.exec2.hello": structs.NewBoolAttribute(true),
		},
	}
}

func failure(state drivers.HealthState, desc string) *drivers.Fingerprint {
	return &drivers.Fingerprint{
		Health:            state,
		HealthDescription: desc,
	}
}

func (p *Plugin) StartTask(config *drivers.TaskConfig) (*drivers.TaskHandle, *drivers.DriverNetwork, error) {
	if config.User == "" {
		panic("anonymous users not yet implemented")
	}

	if _, exists := p.tasks.Get(config.ID); exists {
		// TODO
	}
	return nil, nil, nil
}

func (*Plugin) RecoverTask(handle *drivers.TaskHandle) error {
	// TODO
	return nil
}

func (*Plugin) WaitTask(ctx context.Context, taskID string) (<-chan *drivers.ExitResult, error) {
	// TODO
	return nil, nil
}

func (*Plugin) StopTask(taskID string, timeout time.Duration, signal string) error {
	// TODO
	return nil
}

func (*Plugin) DestroyTask(taskID string, force bool) error {
	// TODO
	return nil
}

func (*Plugin) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	// TODO
	return nil, nil
}

func (*Plugin) TaskStats(ctx context.Context, taskID string, interval time.Duration) (<-chan *drivers.TaskResourceUsage, error) {
	// TODO
	return nil, nil
}

func (*Plugin) TaskEvents(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
	// TODO
	return nil, nil
}

func (*Plugin) SignalTask(taskID, signal string) error {
	// TODO
	return nil
}

func (*Plugin) ExecTask(taskID string, cmd []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
	// TODO
	return nil, nil
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
