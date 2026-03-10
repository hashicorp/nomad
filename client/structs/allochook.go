// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"sync"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/client/pluginmanager/csimanager"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

// AllocHookResources contains data that is provided by AllocRunner Hooks for
// consumption by TaskRunners. This should be instantiated once in the
// AllocRunner and then only accessed via getters and setters that hold the
// lock.
type AllocHookResources struct {
	csiMounts      map[string]*csimanager.MountInfo
	consulTokens   map[string]map[string]*consulapi.ACLToken // Consul cluster -> service identity -> token
	consulCheckIDs [][]string
	networkStatus  *structs.AllocNetworkStatus

	mu sync.RWMutex
}

func NewAllocHookResources() *AllocHookResources {
	return &AllocHookResources{
		csiMounts:      map[string]*csimanager.MountInfo{},
		consulTokens:   map[string]map[string]*consulapi.ACLToken{},
		consulCheckIDs: [][]string{},
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

// GetConsulTokens returns all the Consul tokens previously written by the
// consul allocrunner hook
func (a *AllocHookResources) GetConsulTokens() map[string]map[string]*consulapi.ACLToken {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.consulTokens
}

// SetConsulTokens merges a given map of Consul cluster names to task
// identities to Consul tokens with previously written data. This method is
// called by the allocrunner consul hook.
func (a *AllocHookResources) SetConsulTokens(m map[string]map[string]*consulapi.ACLToken) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for k, v := range m {
		a.consulTokens[k] = v
	}
}

// GetConsulCheckIDs returns a set of Consul check IDs interpolated in the group
// service check, for use in the script check hook. These will be in the same
// order they appear in the jobspec.
func (a *AllocHookResources) GetConsulCheckIDs() [][]string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.consulCheckIDs == nil {
		return nil
	}
	// update hooks could mutate these concurrently, so deep copy
	result := make([][]string, len(a.consulCheckIDs))
	for i, inner := range a.consulCheckIDs {
		if inner != nil {
			result[i] = make([]string, len(inner))
			copy(result[i], inner)
		}
	}

	return result
}

// SetConsulCheckIDs records the set of Consul check IDs interpolated in the
// group service check, for use in the script check hook. These should be in the
// same order they appear in the jobspec.
func (a *AllocHookResources) SetConsulCheckIDs(ids [][]string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.consulCheckIDs = ids
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
