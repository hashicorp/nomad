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
	csiMounts            map[string]*csimanager.MountInfo
	signedTaskIdentities map[string]*structs.SignedWorkloadIdentity

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

// GetSignedIdentitiesForTask returns a copy of the map of task names to
// workload identities signed by the identity allocrunner hook
func (a *AllocHookResources) GetSignedIdentitiesForTask(task *structs.Task) (map[string]*structs.SignedWorkloadIdentity, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	signedIdentitiesForTask := make(map[string]string, len(task.Identities))
	for _, identity := range task.Identities {
		if jwt, ok := a.signedTaskIdentities[identity.Name]; ok {
			signedIdentitiesForTask[identity.Name] = jwt.JWT
		}
	}

	return a.signedTaskIdentities, nil
}

// SetSignedTaskIdentities stores the map of identity names to JWT-encoded
// workload identities signed by the identity allocrunner hook.
func (a *AllocHookResources) SetSignedTaskIdentities(s map[string]*structs.SignedWorkloadIdentity) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.signedTaskIdentities = s
}
