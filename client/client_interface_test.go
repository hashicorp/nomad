// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"sync"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/allocrunner/state"
	"github.com/hashicorp/nomad/client/config"
	cinterfaces "github.com/hashicorp/nomad/client/interfaces"
	"github.com/hashicorp/nomad/client/pluginmanager/drivermanager"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/testutil"
)

// TestEmptyAllocRunner demonstrates the minimum interface necessary to
// implement a mock AllocRunner that can report client status back to the server
func TestEmptyAllocRunner(t *testing.T) {
	ci.Parallel(t)

	s1, _, cleanupS1 := testServer(t, nil)
	defer cleanupS1()

	_, cleanup := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.AllocRunnerFactory = newEmptyAllocRunnerFunc
	})
	defer cleanup()

	job := mock.Job()
	job.Constraints = nil
	job.TaskGroups[0].Constraints = nil
	job.TaskGroups[0].Count = 1
	task := job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{
		"run_for": "10s",
	}
	task.Services = nil

	// WaitForRunning polls the server until the ClientStatus is running
	testutil.WaitForRunning(t, s1.RPC, job)
}

type emptyAllocRunner struct {
	c          cinterfaces.AllocStateHandler
	alloc      *structs.Allocation
	allocState *state.State
	allocLock  sync.RWMutex
}

func newEmptyAllocRunnerFunc(conf *config.AllocRunnerConfig) (interfaces.AllocRunner, error) {
	return &emptyAllocRunner{
		c:          conf.StateUpdater,
		alloc:      conf.Alloc,
		allocState: &state.State{},
	}, nil
}

func (ar *emptyAllocRunner) Alloc() *structs.Allocation {
	ar.allocLock.RLock()
	defer ar.allocLock.RUnlock()
	return ar.alloc.Copy()
}

func (ar *emptyAllocRunner) Run() {
	ar.allocLock.RLock()
	defer ar.allocLock.RUnlock()
	ar.alloc.ClientStatus = "running"
	ar.c.AllocStateUpdated(ar.alloc)
}

func (ar *emptyAllocRunner) Restore() error { return nil }
func (ar *emptyAllocRunner) Update(update *structs.Allocation) {
	ar.allocLock.Lock()
	defer ar.allocLock.Unlock()
	ar.alloc = update
}

func (ar *emptyAllocRunner) Reconnect(update *structs.Allocation) error {
	ar.allocLock.Lock()
	defer ar.allocLock.Unlock()
	ar.alloc = update
	return nil
}

func (ar *emptyAllocRunner) Shutdown() {}
func (ar *emptyAllocRunner) Destroy()  {}

func (ar *emptyAllocRunner) IsDestroyed() bool { return false }
func (ar *emptyAllocRunner) IsMigrating() bool { return false }
func (ar *emptyAllocRunner) IsWaiting() bool   { return false }

func (ar *emptyAllocRunner) WaitCh() <-chan struct{} { return make(chan struct{}) }

func (ar *emptyAllocRunner) DestroyCh() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (ar *emptyAllocRunner) ShutdownCh() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (ar *emptyAllocRunner) AllocState() *state.State {
	ar.allocLock.RLock()
	defer ar.allocLock.RUnlock()
	return ar.allocState.Copy()
}

func (ar *emptyAllocRunner) PersistState() error           { return nil }
func (ar *emptyAllocRunner) AcknowledgeState(*state.State) {}
func (ar *emptyAllocRunner) GetUpdatePriority(*structs.Allocation) cstructs.AllocUpdatePriority {
	return cstructs.AllocUpdatePriorityUrgent
}

func (ar *emptyAllocRunner) SetClientStatus(status string) {
	ar.allocLock.Lock()
	defer ar.allocLock.Unlock()
	ar.alloc.ClientStatus = status
}

func (ar *emptyAllocRunner) Signal(taskName, signal string) error { return nil }
func (ar *emptyAllocRunner) RestartTask(taskName string, taskEvent *structs.TaskEvent) error {
	return nil
}
func (ar *emptyAllocRunner) RestartRunning(taskEvent *structs.TaskEvent) error { return nil }
func (ar *emptyAllocRunner) RestartAll(taskEvent *structs.TaskEvent) error     { return nil }

func (ar *emptyAllocRunner) GetTaskEventHandler(taskName string) drivermanager.EventHandler {
	return nil
}
func (ar *emptyAllocRunner) GetTaskExecHandler(taskName string) drivermanager.TaskExecHandler {
	return nil
}
func (ar *emptyAllocRunner) GetTaskDriverCapabilities(taskName string) (*drivers.Capabilities, error) {
	return nil, nil
}

func (ar *emptyAllocRunner) StatsReporter() interfaces.AllocStatsReporter { return ar }
func (ar *emptyAllocRunner) Listener() *cstructs.AllocListener            { return nil }
func (ar *emptyAllocRunner) GetAllocDir() *allocdir.AllocDir              { return nil }

// LatestAllocStats lets this empty runner implement AllocStatsReporter
func (ar *emptyAllocRunner) LatestAllocStats(taskFilter string) (*cstructs.AllocResourceUsage, error) {
	return &cstructs.AllocResourceUsage{
		ResourceUsage: &cstructs.ResourceUsage{
			MemoryStats: &cstructs.MemoryStats{},
			CpuStats:    &cstructs.CpuStats{},
			DeviceStats: []*device.DeviceGroupStats{},
		},
		Tasks:     map[string]*cstructs.TaskResourceUsage{},
		Timestamp: 0,
	}, nil
}
