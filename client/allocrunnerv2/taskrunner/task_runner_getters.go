package taskrunner

import (
	"github.com/hashicorp/nomad/client/driver"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

func (tr *TaskRunner) Alloc() *structs.Allocation {
	tr.allocLock.Lock()
	defer tr.allocLock.Unlock()
	return tr.alloc
}

func (tr *TaskRunner) Task() *structs.Task {
	tr.taskLock.RLock()
	defer tr.taskLock.RUnlock()
	return tr.task
}

func (tr *TaskRunner) TaskState() *structs.TaskState {
	tr.stateLock.Lock()
	defer tr.stateLock.Unlock()
	return tr.state.Copy()
}

func (tr *TaskRunner) getVaultToken() string {
	tr.vaultTokenLock.Lock()
	defer tr.vaultTokenLock.Unlock()
	return tr.vaultToken
}

func (tr *TaskRunner) setVaultToken(token string) {
	tr.vaultTokenLock.Lock()
	defer tr.vaultTokenLock.Unlock()
	tr.vaultToken = token
}

// getDriverHandle returns the DriverHandle and associated driver metadata (at
// this point just the network) if it exists.
func (tr *TaskRunner) getDriverHandle() (driver.DriverHandle, *cstructs.DriverNetwork) {
	tr.handleLock.Lock()
	defer tr.handleLock.Unlock()
	return tr.handle, tr.driverNet
}

func (tr *TaskRunner) setDriverHandle(handle driver.DriverHandle, net *cstructs.DriverNetwork) {
	tr.handleLock.Lock()
	defer tr.handleLock.Unlock()
	tr.handle = handle
	tr.driverNet = net
}

// clearDriverHandle clears the driver handle and associated driver metadata
// (driver network).
func (tr *TaskRunner) clearDriverHandle() {
	tr.handleLock.Lock()
	defer tr.handleLock.Unlock()
	tr.handle = nil
	tr.driverNet = nil
}
