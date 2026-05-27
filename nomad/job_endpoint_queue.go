// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

func (j *Job) QueueStatus(args *structs.QueueStatusRequest, reply *structs.QueueStatusResponse) error {
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Job.QueueStatus", args, args, reply); done {
		return err
	}
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	status := j.srv.batchJobQueue.Status()

	reply.Type = status.Type
	reply.Workloads = status.Workloads
	return nil
}
