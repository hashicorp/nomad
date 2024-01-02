// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package interfaces

import (
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/state"
	"github.com/hashicorp/nomad/client/pluginmanager/csimanager"
	"github.com/hashicorp/nomad/client/pluginmanager/drivermanager"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// AllocRunner is the interface to the allocRunner struct used by client.Client
type AllocRunner interface {
	Alloc() *structs.Allocation

	Run()
	Restore() error
	Update(*structs.Allocation)
	Reconnect(update *structs.Allocation) error
	Shutdown()
	Destroy()

	IsDestroyed() bool
	IsMigrating() bool
	IsWaiting() bool

	WaitCh() <-chan struct{}
	DestroyCh() <-chan struct{}
	ShutdownCh() <-chan struct{}

	AllocState() *state.State
	PersistState() error
	AcknowledgeState(*state.State)
	GetUpdatePriority(*structs.Allocation) cstructs.AllocUpdatePriority
	SetClientStatus(string)

	Signal(taskName, signal string) error
	RestartTask(taskName string, taskEvent *structs.TaskEvent) error
	RestartRunning(taskEvent *structs.TaskEvent) error
	RestartAll(taskEvent *structs.TaskEvent) error

	GetTaskEventHandler(taskName string) drivermanager.EventHandler
	GetTaskExecHandler(taskName string) drivermanager.TaskExecHandler
	GetTaskDriverCapabilities(taskName string) (*drivers.Capabilities, error)
	StatsReporter() AllocStatsReporter
	Listener() *cstructs.AllocListener
	GetAllocDir() *allocdir.AllocDir
}

// TaskStateHandler exposes a handler to be called when a task's state changes
type TaskStateHandler interface {
	// TaskStateUpdated is used to notify the alloc runner about task state
	// changes.
	TaskStateUpdated()
}

// AllocStatsReporter gives access to the latest resource usage from the
// allocation
type AllocStatsReporter interface {
	LatestAllocStats(taskFilter string) (*cstructs.AllocResourceUsage, error)
}

// HookResourceSetter is used to communicate between alloc hooks and task hooks
type HookResourceSetter interface {
	SetCSIMounts(map[string]*csimanager.MountInfo)
	GetCSIMounts(map[string]*csimanager.MountInfo)
}
