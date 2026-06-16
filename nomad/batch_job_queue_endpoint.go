// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/acl"
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
	q.srv.MeasureRPCRate("queue.status", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	aclObj, err := q.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityListJobs) {
		return structs.ErrPermissionDenied
	}

	state, _ := q.srv.State().Snapshot()

	allow := aclObj.AllowNsOpFunc(acl.NamespaceCapabilityListJobs)
	allowableNamespaces, err := allowedNSes(aclObj, &state.StateStore, allow)
	if err == structs.ErrPermissionDenied {
		// return empty results if token isn't authorized for any
		// namespace, matching other endpoints
		reply.Results = make([]structs.QueueStatusResponse, 0)
	} else if err != nil {
		return err
	} else {
		status := q.srv.batchJobQueue.Status(allowableNamespaces, *args)
		reply.Results = status.Results
		reply.Type = status.Type
	}

	q.srv.setQueryMeta(&reply.QueryMeta)

	return nil
}
