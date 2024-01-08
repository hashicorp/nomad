// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package exec2

import (
	"context"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/drivers/shared/eventer"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

type ExecTwo struct {
	// events is used to handle multiplexing of TaskEvent calls such that
	// an event can be broadcast to all callers
	events *eventer.Eventer

	// config is the plugin configuration set by the SetConfig RPC
	config *Config

	// driverConfig is the driver-client configuration from Nomad
	// driverConfig *base.ClientDriverConfig

	// tasks is the in-memory datastore mapping IDs to handles
	// tasks task.Store // TODO

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
	return &ExecTwo{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (e *ExecTwo) PluginInfo() (*base.PluginInfoResponse, error) {
	return info, nil
}

func (e *ExecTwo) ConfigSchema() (*hclspec.Spec, error) {
	return driverConfigSpec, nil
}

func (e *ExecTwo) SetConfig(c *base.Config) error {
	// TODO
	return nil
}

func (e *ExecTwo) TaskConfigSchema() (*hclspec.Spec, error) {
	return taskConfigSpec, nil
}

func (e *ExecTwo) Capabilities() (*drivers.Capabilities, error) {
	return capabilities, nil
}

func (e *ExecTwo) Fingerprint(ctx context.Context) (<-chan *drivers.Fingerprint, error) {
	ch := make(chan *drivers.Fingerprint)
	// TODO go fingerprint
	return ch, nil
}

func (e *ExecTwo) StartTask(config *drivers.TaskConfig) (*drivers.TaskHandle, *drivers.DriverNetwork, error) {
	// TODO
	return nil, nil, nil
}

func (e *ExecTwo) RecoverTask(handle *drivers.TaskHandle) error {
	// TODO
	return nil
}

func (e *ExecTwo) WaitTask(ctx context.Context, taskID string) (<-chan *drivers.ExitResult, error) {
	// TODO
	return nil, nil
}

func (e *ExecTwo) StopTask(taskID string, timeout time.Duration, signal string) error {
	// TODO
	return nil
}

func (e *ExecTwo) DestroyTask(taskID string, force bool) error {
	// TODO
	return nil
}

func (e *ExecTwo) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	// TODO
	return nil, nil
}

func (e *ExecTwo) TaskStats(ctx context.Context, taskID string, interval time.Duration) (<-chan *drivers.TaskResourceUsage, error) {
	// TODO
	return nil, nil
}

func (e *ExecTwo) TaskEvents(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
	// TODO
	return nil, nil
}

func (e *ExecTwo) SignalTask(taskID, signal string) error {
	// TODO
	return nil
}

func (e *ExecTwo) ExecTask(taskID string, cmd []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
	// TODO
	return nil, nil
}
