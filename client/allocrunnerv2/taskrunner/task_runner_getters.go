package taskrunner

import (
	"github.com/hashicorp/nomad/client/driver"
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

func (tr *TaskRunner) getDriverHandle() driver.DriverHandle {
	tr.handleLock.Lock()
	defer tr.handleLock.Unlock()
	return tr.handle
}

func (tr *TaskRunner) setDriverHandle(handle driver.DriverHandle) {
	tr.handleLock.Lock()
	defer tr.handleLock.Unlock()
	tr.handle = handle
}
