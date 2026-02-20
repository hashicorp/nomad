// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: MPL-2.0

package drivers

import (
	"context"
	"time"

	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

type TaskConfigSchemaFn func() (*hclspec.Spec, error)
type CapabilitiesFn func() (*Capabilities, error)
type FingerprintFn func(context.Context) (<-chan *Fingerprint, error)
type RecoverTaskFn func(*TaskHandle) error
type StartTaskFn func(*TaskConfig) (*TaskHandle, *DriverNetwork, error)
type WaitTaskFn func(context.Context, string) (<-chan *ExitResult, error)
type StopTaskFn func(string, time.Duration, string) error
type DestroyTaskFn func(string, bool) error
type InspectTaskFn func(string) (*TaskStatus, error)
type TaskStatsFn func(context.Context, string, time.Duration) (<-chan *TaskResourceUsage, error)
type TaskEventsFn func(context.Context) (<-chan *TaskEvent, error)
type SignalTaskFn func(string, string) error
type ExecTaskFn func(string, []string, time.Duration) (*ExecTaskResult, error)

type MockDriverPlugin struct {
	*base.MockPlugin

	TaskConfigSchemaFn TaskConfigSchemaFn
	CapabilitiesFn     CapabilitiesFn
	FingerprintFn      FingerprintFn
	RecoverTaskFn      RecoverTaskFn
	StartTaskFn        StartTaskFn
	WaitTaskFn         WaitTaskFn
	StopTaskFn         StopTaskFn
	DestroyTaskFn      DestroyTaskFn
	InspectTaskFn      InspectTaskFn
	TaskStatsFn        TaskStatsFn
	TaskEventsFn       TaskEventsFn
	SignalTaskFn       SignalTaskFn
	ExecTaskFn         ExecTaskFn
}

func (p *MockDriverPlugin) TaskConfigSchema() (*hclspec.Spec, error) {
	return p.TaskConfigSchemaFn()
}

func (p *MockDriverPlugin) Capabilities() (*Capabilities, error) {
	return p.CapabilitiesFn()
}

func (p *MockDriverPlugin) Fingerprint(ctx context.Context) (<-chan *Fingerprint, error) {
	return p.FingerprintFn(ctx)
}

func (p *MockDriverPlugin) RecoverTask(handle *TaskHandle) error {
	return p.RecoverTaskFn(handle)
}

func (p *MockDriverPlugin) StartTask(config *TaskConfig) (*TaskHandle, *DriverNetwork, error) {
	return p.StartTaskFn(config)
}

func (p *MockDriverPlugin) WaitTask(ctx context.Context, taskID string) (<-chan *ExitResult, error) {
	return p.WaitTaskFn(ctx, taskID)
}

func (p *MockDriverPlugin) StopTask(taskID string, timeout time.Duration, signal string) error {
	return p.StopTaskFn(taskID, timeout, signal)
}

func (p *MockDriverPlugin) DestroyTask(taskID string, force bool) error {
	return p.DestroyTaskFn(taskID, force)
}

func (p *MockDriverPlugin) InspectTask(taskID string) (*TaskStatus, error) {
	return p.InspectTaskFn(taskID)
}

func (p *MockDriverPlugin) TaskStats(ctx context.Context, taskID string, interval time.Duration) (<-chan *TaskResourceUsage, error) {
	return p.TaskStatsFn(ctx, taskID, interval)
}

func (p *MockDriverPlugin) TaskEvents(ctx context.Context) (<-chan *TaskEvent, error) {
	return p.TaskEventsFn(ctx)
}

func (p *MockDriverPlugin) SignalTask(taskID string, signal string) error {
	return p.SignalTaskFn(taskID, signal)
}

func (p *MockDriverPlugin) ExecTask(taskID string, cmd []string, timeout time.Duration) (*ExecTaskResult, error) {
	return p.ExecTaskFn(taskID, cmd, timeout)
}
