// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package testing

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

// MockEmitter is a mock of the EventEmitter interface.
type MockEmitter struct {
	lock   sync.Mutex
	events []*structs.TaskEvent
}

func (m *MockEmitter) EmitEvent(ev *structs.TaskEvent) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.events = append(m.events, ev)
}

func (m *MockEmitter) Events() []*structs.TaskEvent {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.events
}

// MockTaskHooks is a mock of the TaskHooks interface useful for testing
type MockTaskHooks struct {
	lock sync.Mutex

	RestartCh chan struct{}
	restarts  int

	SignalCh chan struct{}
	signals  []string

	// SignalError is returned when Signal is called on the mock hook
	SignalError error

	UnblockCh chan struct{}

	KillCh    chan *structs.TaskEvent
	killEvent *structs.TaskEvent

	EmitEventCh chan *structs.TaskEvent
	events      []*structs.TaskEvent

	execCode int
	execErr  error

	// HasHandle can be set to simulate restoring a task after client restart
	HasHandle bool
}

func NewMockTaskHooks() *MockTaskHooks {
	return &MockTaskHooks{
		UnblockCh:   make(chan struct{}, 1),
		RestartCh:   make(chan struct{}, 1),
		SignalCh:    make(chan struct{}, 1),
		KillCh:      make(chan *structs.TaskEvent, 1),
		EmitEventCh: make(chan *structs.TaskEvent, 1),
	}
}
func (m *MockTaskHooks) Restart(ctx context.Context, event *structs.TaskEvent, failure bool) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.restarts++
	select {
	case m.RestartCh <- struct{}{}:
	default:
	}
	return nil
}

func (m *MockTaskHooks) Signal(event *structs.TaskEvent, s string) error {
	m.lock.Lock()
	m.signals = append(m.signals, s)
	m.lock.Unlock()

	select {
	case m.SignalCh <- struct{}{}:
	default:
	}

	return m.SignalError
}

func (m *MockTaskHooks) Signals() []string {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.signals
}

func (m *MockTaskHooks) Kill(ctx context.Context, event *structs.TaskEvent) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.killEvent = event
	select {
	case m.KillCh <- event:
	default:
	}
	return nil
}

func (m *MockTaskHooks) Exec(timeout time.Duration, cmd string, args []string) ([]byte, int, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	return []byte{}, m.execCode, m.execErr
}

func (m *MockTaskHooks) SetupExecTest(code int, err error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.execCode = code
	m.execErr = err
}

func (m *MockTaskHooks) IsRunning() bool {
	return m.HasHandle
}

func (m *MockTaskHooks) EmitEvent(event *structs.TaskEvent) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.events = append(m.events, event)
	select {
	case m.EmitEventCh <- event:
	case <-m.EmitEventCh:
		m.EmitEventCh <- event
	}
}

func (m *MockTaskHooks) SetState(state string, event *structs.TaskEvent) {}

func (m *MockTaskHooks) KillEvent() *structs.TaskEvent {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.killEvent
}

func (m *MockTaskHooks) Events() []*structs.TaskEvent {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.events
}

func (m *MockTaskHooks) Restarts() int {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.restarts
}
