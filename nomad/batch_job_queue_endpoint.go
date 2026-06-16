// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

type BatchJobQueue struct {
	srv    *Server
	ctx    *RPCContext
	logger hclog.Logger
}

func NewBatchJobQueueEndpoints(s *Server, ctx *RPCContext) *BatchJobQueue {
	return &BatchJobQueue{
		srv:    s,
		ctx:    ctx,
		logger: s.logger.Named("batch_job_queue"),
	}
}

func (q *BatchJobQueue) Status(args *structs.QueueStatusRequest, reply *structs.QueueStatusResponse) error {
	authErr := q.srv.Authenticate(q.ctx, args)
	if done, err := q.srv.forward("BatchJobQueue.Status", args, args, reply); done {
		return err
	}
	q.srv.MeasureRPCRate("queue", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	status := q.srv.batchJobQueue.Status()

	reply.Type = status.Type
	reply.Workloads = status.Workloads
	return nil
}
