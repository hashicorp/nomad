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
type AllocHookResources struct {
	csiMounts     map[string]*csimanager.MountInfo
	networkStatus *structs.AllocNetworkStatus

	mu sync.RWMutex
}

func NewAllocHookResources() *AllocHookResources {
	return &AllocHookResources{
		csiMounts: map[string]*csimanager.MountInfo{},
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

// GetAllocNetworkStatus returns a copy of the AllocNetworkStatus previously
// written the group's network_hook
func (a *AllocHookResources) GetAllocNetworkStatus() *structs.AllocNetworkStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.networkStatus.Copy()
}

// SetAllocNetworkStatus stores the AllocNetworkStatus for later use by the
// taskrunner's buildTaskConfig() method
func (a *AllocHookResources) SetAllocNetworkStatus(ans *structs.AllocNetworkStatus) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.networkStatus = ans
}
