// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"sync"

	"github.com/hashicorp/nomad/client/pluginmanager/csimanager"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

type TaskIdentity struct {
	TaskName     string
	IdentityName string
}

// AllocHookResources contains data that is provided by AllocRunner Hooks for
// consumption by TaskRunners. This should be instantiated once in the
// AllocRunner and then only accessed via getters and setters that hold the
// lock.
type AllocHookResources struct {
	csiMounts map[string]*csimanager.MountInfo

	// SignedTaskIdentities is a map of task names to channels that contain maps of
	// identity names to signed WI.
	// WARNING: these maps or channels are *not* allocated in the AllocHookResources
	// constructor, but in the allocrunner identity_hook instead.
	SignedTaskIdentities map[*TaskIdentity]chan *structs.SignedWorkloadIdentity
	StopChanForTask      map[string]chan struct{}

	mu sync.RWMutex
}

func NewAllocHookResources() *AllocHookResources {
	stop := make(map[string]chan struct{})
	return &AllocHookResources{
		csiMounts:       map[string]*csimanager.MountInfo{},
		StopChanForTask: stop,
	}
}

// GetCSIMounts returns a copy of the CSI mount info previously written by the
// CSI allocrunner hook
func (a *AllocHookResources) GetCSIMounts() map[string]*csimanager.MountInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return helper.DeepCopyMap(a.csiMounts)
}

// SetCSIMounts stores the CSI mount info for later use by the volume taskrunner
// hook
func (a *AllocHookResources) SetCSIMounts(m map[string]*csimanager.MountInfo) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.csiMounts = m
}

// func (a *AllocHookResources) GetSignedIdentitiesForTask(taskname string)
