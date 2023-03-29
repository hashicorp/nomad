package structs

import (
	"sync"

	"github.com/hashicorp/nomad/client/pluginmanager/csimanager"
)

// AllocHookResources contains data that is provided by AllocRunner Hooks for
// consumption by TaskRunners. This should be instantiated once in the
// AllocRunner and then only accessed via getters and setters that hold the
// lock.
type AllocHookResources struct {
	csiMounts map[string]*csimanager.MountInfo

	mu sync.RWMutex
}

func NewAllocHookResources() *AllocHookResources {
	return &AllocHookResources{
		csiMounts: map[string]*csimanager.MountInfo{},
	}
}

func (a *AllocHookResources) GetCSIMounts() map[string]*csimanager.MountInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.csiMounts
}

func (a *AllocHookResources) SetCSIMounts(m map[string]*csimanager.MountInfo) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.csiMounts = m
}
