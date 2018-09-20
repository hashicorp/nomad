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

func (tr *TaskRunner) setAlloc(updated *structs.Allocation) {
	tr.allocLock.Lock()
	tr.alloc = updated
	tr.allocLock.Unlock()
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

// setVaultToken updates the vault token on the task runner as well as in the
// task's environment. These two places must be set atomically to avoid a task
// seeing a different token on the task runner and in its environment.
func (tr *TaskRunner) setVaultToken(token string) {
	tr.vaultTokenLock.Lock()
	defer tr.vaultTokenLock.Unlock()

	// Update the Vault token on the runner
	tr.vaultToken = token

	// Update the task's environment
	tr.envBuilder.SetVaultToken(token, tr.task.Vault.Env)
}

// getDriverHandle returns a driver handle and its result proxy. Use the
// result proxy instead of the handle's WaitCh.
func (tr *TaskRunner) getDriverHandle() (driver.DriverHandle, *handleResult) {
	tr.handleLock.Lock()
	defer tr.handleLock.Unlock()
	return tr.handle, tr.handleResult
}

// setDriverHanlde sets the driver handle and creates a new result proxy.
func (tr *TaskRunner) setDriverHandle(handle driver.DriverHandle) {
	tr.handleLock.Lock()
	defer tr.handleLock.Unlock()
	tr.handle = handle
	tr.handleResult = newHandleResult(handle.WaitCh())
}

func (tr *TaskRunner) clearDriverHandle() {
	tr.handleLock.Lock()
	defer tr.handleLock.Unlock()
	tr.handle = nil
	tr.handleResult = nil
}
