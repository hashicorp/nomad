package allocrunner

import (
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

type hookResourceSetter interface {
	GetAllocHookResources() *cstructs.AllocHookResources
	SetAllocHookResources(*cstructs.AllocHookResources)
}

type allocHookResourceSetter struct {
	ar *allocRunner
}

func (a *allocHookResourceSetter) GetAllocHookResources() *cstructs.AllocHookResources {
	return nil
}

func (a *allocHookResourceSetter) SetAllocHookResources(res *cstructs.AllocHookResources) {}

// allocHealthSetter is a shim to allow the alloc health watcher hook to set
// and clear the alloc health without full access to the alloc runner state
type allocHealthSetter struct {
	ar *allocRunner
}

// HasHealth returns true if a deployment status is already set.
func (*allocHealthSetter) HasHealth() bool {
	return true
}

// ClearHealth allows the health watcher hook to clear the alloc's deployment
// health if the deployment id changes. It does not update the server as the
// status is only cleared when already receiving an update from the server.
//
// Only for use by health hook.
func (a *allocHealthSetter) ClearHealth() {}

// SetHealth allows the health watcher hook to set the alloc's
// deployment/migration health and emit task events.
//
// Only for use by health hook.
func (a *allocHealthSetter) SetHealth(healthy, isDeploy bool, trackerTaskEvents map[string]*structs.TaskEvent) {
}
