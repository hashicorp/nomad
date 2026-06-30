// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/state/paginator"
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

func (q *BatchJobQueue) Jobs(args *structs.QueueJobsRequest, reply *structs.QueueJobsResponse) error {
	authErr := q.srv.Authenticate(q.ctx, args)
	if done, err := q.srv.forward("BatchJobQueue.Jobs", args, args, reply); done {
		return err
	}
	q.srv.MeasureRPCRate("queue.jobs", structs.RateMetricList, args)
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
		reply.Workloads = make([]any, 0)
		q.srv.setQueryMeta(&reply.QueryMeta)
		return err
	}

	batchJobQueue := q.srv.batchQueueMgr.Queue()
	iter := batchJobQueue.Jobs()

	selector := func(workload structs.QueueWorkload) bool {
		if len(allowableNamespaces) == 0 {
			return true
		}
		// Filter by namespace
		if !allowableNamespaces[workload.GetNamespace()] {
			return false
		}
		return true
	}

	pager, err := paginator.NewPaginator(iter, args.QueryOptions, selector, paginator.CreateIndexAndIDTokenizer[structs.QueueWorkload](args.NextToken), func(workload structs.QueueWorkload) (structs.QueueWorkload, error) {
		return workload, nil
	})
	if err != nil {
		return err
	}
	workloads, token, err := pager.Page()
	if err != nil {
		return err
	}
	reply.Workloads = workloads

	reply.Type = batchJobQueue.Type()
	reply.QueryMeta.NextToken = token
	q.srv.setQueryMeta(&reply.QueryMeta)

	return nil
}

func (q *BatchJobQueue) Tenants(args *structs.QueueTenantsRequest, reply *structs.QueueTenantsResponse) error {
	authErr := q.srv.Authenticate(q.ctx, args)
	if done, err := q.srv.forward("BatchJobQueue.Tenants", args, args, reply); done {
		return err
	}
	q.srv.MeasureRPCRate("queue.tenants", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	aclObj, err := q.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if !aclObj.AllowOperatorRead() {
		return structs.ErrPermissionDenied
	}

	status := q.srv.batchQueueMgr.Queue().Tenants()
	reply.Tenants = status.Tenants
	reply.Type = status.Type

	q.srv.setQueryMeta(&reply.QueryMeta)

	return nil
}
