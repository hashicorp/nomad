// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import "github.com/hashicorp/nomad/nomad/structs"

func (tr *TaskRunner) StoreWorkloadIdentity(name string, wi *structs.SignedWorkloadIdentity) error {
	tr.widsLocks.Lock()
	defer tr.widsLocks.Unlock()

	tr.wids[name] = wi
	return nil
}

func (tr *TaskRunner) GetWorkloadIdentity(name string) (*structs.SignedWorkloadIdentity, error) {
	tr.widsLocks.RLock()
	defer tr.widsLocks.RUnlock()

	return tr.wids[name], nil
}
