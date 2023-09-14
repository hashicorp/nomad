// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"sync"

	"github.com/hashicorp/nomad/client/pluginmanager/csimanager"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

// AllocHookResources contains data that is provided by AllocRunner Hooks for
// consumption by TaskRunners. This should be instantiated once in the
// AllocRunner and then only accessed via getters and setters that hold the
// lock.
//
// WARNING: create a SignedTaskIdentities channel manually before use. The
// constructor will not create it, because it has to be a buffered channel
// (size of the amount of task in an allocation).
type AllocHookResources struct {
	csiMounts            map[string]*csimanager.MountInfo
	SignedTaskIdentities chan map[string]*structs.SignedWorkloadIdentity
	StopChan             chan struct{}

	mu sync.RWMutex
}

func NewAllocHookResources() *AllocHookResources {
	stop := make(chan struct{})
	return &AllocHookResources{
		csiMounts: map[string]*csimanager.MountInfo{},
		StopChan:  stop,
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
