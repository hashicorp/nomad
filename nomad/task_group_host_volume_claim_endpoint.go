// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	metrics "github.com/hashicorp/go-metrics/compat"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// TaskGroupHostVolumeClaim is the server RPC endpoint for task group volume claims
type TaskGroupHostVolumeClaim struct {
	srv    *Server
	ctx    *RPCContext
	logger hclog.Logger
}

func NewTaskGroupVolumeClaimEndpoint(srv *Server, ctx *RPCContext) *TaskGroupHostVolumeClaim {
	return &TaskGroupHostVolumeClaim{srv: srv, ctx: ctx, logger: srv.logger.Named("task_group_host_volume_claim")}
}

func (tgvc *TaskGroupHostVolumeClaim) List(args *structs.TaskGroupVolumeClaimListRequest, reply *structs.TaskGroupVolumeClaimListResponse) error {
	authErr := tgvc.srv.Authenticate(tgvc.ctx, args)
	if done, err := tgvc.srv.forward(structs.TaskGroupHostVolumeClaimListRPCMethod, args, args, reply); done {
		return err
	}
	tgvc.srv.MeasureRPCRate("task_group_volume_claim", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "task_group_volume_claim", "list"}, time.Now())

	// TODO: should this be a separate ACL capability? (nooo?)
	allowClaim := acl.NamespaceValidator(acl.NamespaceCapabilityHostVolumeRead)
	aclObj, err := tgvc.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if !allowClaim(aclObj, args.RequestNamespace()) {
		return structs.ErrPermissionDenied
	}

	return tgvc.srv.blockingRPC(&blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, stateStore *state.StateStore) error {

			// The iteration below appends directly to the reply object, so in
			// order for blocking queries to work properly we must ensure the
			// Claims are reset. This allows the blocking query run function to
			// work as expected.
			reply.Claims = nil

			iter, err := stateStore.GetTaskGroupHostVolumeClaims(ws)
			if err != nil {
				return err
			}

			// Iterate all the results and add these to our reply object.
			for raw := iter.Next(); raw != nil; raw = iter.Next() {
				reply.Claims = append(reply.Claims, raw.(*structs.TaskGroupHostVolumeClaim))
			}

			// Use the index table to populate the query meta as we have no way
			// of tracking the max index on deletes.
			return tgvc.srv.setReplyQueryMeta(stateStore, state.TableTaskGroupHostVolumeClaim, &reply.QueryMeta)
		},
	})
}

func (tgvc *TaskGroupHostVolumeClaim) Delete(args *structs.TaskGroupVolumeClaimDeleteRequest, reply *structs.TaskGroupVolumeClaimDeleteResponse) error {

	authErr := tgvc.srv.Authenticate(tgvc.ctx, args)
	if done, err := tgvc.srv.forward(structs.TaskGroupHostVolumeClaimDeleteRPCMethod, args, args, reply); done {
		return err
	}
	tgvc.srv.MeasureRPCRate("task_group_host_volume_claim", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "task_group_host_volume_claim", "delete"}, time.Now())

	// Note that all deleted claims need to be in the same namespace
	// TODO: should this be a separate ACL capability? (nooo?)
	allowClaim := acl.NamespaceValidator(acl.NamespaceCapabilityHostVolumeDelete)
	aclObj, err := tgvc.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if !allowClaim(aclObj, args.RequestNamespace()) {
		return structs.ErrPermissionDenied
	}

	if args.ClaimID == "" {
		return fmt.Errorf("missing claim ID to delete")
	}

	// Update via Raft
	_, index, err := tgvc.srv.raftApply(structs.TaskGroupHostVolumeClaimDeleteRequestType, args)
	if err != nil {
		return err
	}

	// Update the index
	reply.Index = index
	return nil
}
