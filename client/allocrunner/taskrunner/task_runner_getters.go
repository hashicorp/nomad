package taskrunner

import (
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

// IsLeader returns true if this task is the leader of its task group.
func (tr *TaskRunner) IsLeader() bool {
	return tr.taskLeader
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
func (tr *TaskRunner) getDriverHandle() *DriverHandle {
	tr.handleLock.Lock()
	defer tr.handleLock.Unlock()
	return tr.handle
}

// setDriverHanlde sets the driver handle, creates a new result proxy, and
// updates the driver network in the task's environment.
func (tr *TaskRunner) setDriverHandle(handle *DriverHandle) {
	tr.handleLock.Lock()
	defer tr.handleLock.Unlock()
	tr.handle = handle

	// Update the environment's driver network
	tr.envBuilder.SetDriverNetwork(handle.net)
}

func (tr *TaskRunner) clearDriverHandle() {
	tr.handleLock.Lock()
	defer tr.handleLock.Unlock()
	if tr.handle != nil {
		tr.driver.DestroyTask(tr.handle.ID(), true)
	}
	tr.handle = nil
}
