// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

// deploymentWatcherRaftShim is the shim that provides the state watching
// methods. These should be set by the server and passed to the deployment
// watcher.
type deploymentWatcherRaftShim struct {
	// apply is used to apply a message to Raft
	apply raftApplyFn
}

// convertApplyErrors parses the results of a raftApply and returns the index at
// which it was applied and any error that occurred. Raft Apply returns two
// separate errors, Raft library errors and user returned errors from the FSM.
// This helper, joins the errors by inspecting the applyResponse for an error.
func (d *deploymentWatcherRaftShim) convertApplyErrors(applyResp interface{}, index uint64, err error) (uint64, error) {
	if applyResp != nil {
		if fsmErr, ok := applyResp.(error); ok && fsmErr != nil {
			return index, fsmErr
		}
	}
	return index, err
}

func (d *deploymentWatcherRaftShim) UpsertJob(job *structs.Job) (uint64, error) {
	job.SetSubmitTime()
	update := &structs.JobRegisterRequest{
		Job: job,
	}

	fsmErrIntf, index, raftErr := d.apply(structs.JobRegisterRequestType, update)
	return d.convertApplyErrors(fsmErrIntf, index, raftErr)
}

func (d *deploymentWatcherRaftShim) UpdateDeploymentStatus(u *structs.DeploymentStatusUpdateRequest) (uint64, error) {
	fsmErrIntf, index, raftErr := d.apply(structs.DeploymentStatusUpdateRequestType, u)
	return d.convertApplyErrors(fsmErrIntf, index, raftErr)
}

func (d *deploymentWatcherRaftShim) UpdateDeploymentPromotion(req *structs.ApplyDeploymentPromoteRequest) (uint64, error) {
	fsmErrIntf, index, raftErr := d.apply(structs.DeploymentPromoteRequestType, req)
	return d.convertApplyErrors(fsmErrIntf, index, raftErr)
}

func (d *deploymentWatcherRaftShim) UpdateDeploymentAllocHealth(req *structs.ApplyDeploymentAllocHealthRequest) (uint64, error) {
	fsmErrIntf, index, raftErr := d.apply(structs.DeploymentAllocHealthRequestType, req)
	return d.convertApplyErrors(fsmErrIntf, index, raftErr)
}

func (d *deploymentWatcherRaftShim) UpdateAllocDesiredTransition(req *structs.AllocUpdateDesiredTransitionRequest) (uint64, error) {
	fsmErrIntf, index, raftErr := d.apply(structs.AllocUpdateDesiredTransitionRequestType, req)
	return d.convertApplyErrors(fsmErrIntf, index, raftErr)
}
