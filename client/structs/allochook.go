package structs

import (
	"sync"

	"github.com/hashicorp/nomad/client/pluginmanager/csimanager"
)

// AllocHookResources contains data that is provided by AllocRunner Hooks for
// consumption by TaskRunners
type AllocHookResources struct {
	CSIMounts map[string]*csimanager.MountInfo

	mu sync.RWMutex
}

func (a *AllocHookResources) GetCSIMounts() map[string]*csimanager.MountInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.CSIMounts
}

func (a *AllocHookResources) SetCSIMounts(m map[string]*csimanager.MountInfo) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.CSIMounts = m
}
