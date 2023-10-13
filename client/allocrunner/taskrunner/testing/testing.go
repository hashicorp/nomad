// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package testing

import (
	"context"
	"sync"

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
	Restarts  int
	RestartCh chan struct{}

	SignalCh   chan struct{}
	signals    []string
	signalLock sync.Mutex

	// SignalError is returned when Signal is called on the mock hook
	SignalError error

	UnblockCh chan struct{}

	KillEvent *structs.TaskEvent
	KillCh    chan *structs.TaskEvent

	Events      []*structs.TaskEvent
	EmitEventCh chan *structs.TaskEvent

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
	m.Restarts++
	select {
	case m.RestartCh <- struct{}{}:
	default:
	}
	return nil
}

func (m *MockTaskHooks) Signal(event *structs.TaskEvent, s string) error {
	m.signalLock.Lock()
	m.signals = append(m.signals, s)
	m.signalLock.Unlock()
	select {
	case m.SignalCh <- struct{}{}:
	default:
	}

	return m.SignalError
}

func (m *MockTaskHooks) Signals() []string {
	m.signalLock.Lock()
	defer m.signalLock.Unlock()
	return m.signals
}

func (m *MockTaskHooks) Kill(ctx context.Context, event *structs.TaskEvent) error {
	m.KillEvent = event
	select {
	case m.KillCh <- event:
	default:
	}
	return nil
}

func (m *MockTaskHooks) IsRunning() bool {
	return m.HasHandle
}

func (m *MockTaskHooks) EmitEvent(event *structs.TaskEvent) {
	m.Events = append(m.Events, event)
	select {
	case m.EmitEventCh <- event:
	case <-m.EmitEventCh:
		m.EmitEventCh <- event
	}
}

func (m *MockTaskHooks) SetState(state string, event *structs.TaskEvent) {}
