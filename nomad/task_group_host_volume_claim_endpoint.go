// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	metrics "github.com/hashicorp/go-metrics/compat"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/state/paginator"
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

	allowClaim := acl.NamespaceValidator(acl.NamespaceCapabilityHostVolumeRead)
	aclObj, err := tgvc.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if !allowClaim(aclObj, args.RequestNamespace()) {
		return structs.ErrPermissionDenied
	}

	ns := args.RequestNamespace()

	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, stateStore *state.StateStore) error {
			var iter memdb.ResultIterator
			var err error

			switch {
			case args.JobID != "":
				iter, err = stateStore.TaskGroupHostVolumeClaimsByJobID(ws, args.JobID)
			case args.TaskGroup != "":
				iter, err = stateStore.TaskGroupHostVolumeClaimsByTaskGroup(ws, args.TaskGroup)
			case args.VolumeName != "":
				iter, err = stateStore.TaskGroupHostVolumeClaimsByVolumeName(ws, args.VolumeName)
			default:
				iter, err = stateStore.GetTaskGroupHostVolumeClaims(ws)
			}
			if err != nil {
				return err
			}

			tokenizer := paginator.NewStructsTokenizer(iter,
				paginator.StructsTokenizerOptions{
					WithNamespace: true,
					WithID:        true,
				},
			)

			filters := []paginator.Filter{
				paginator.GenericFilter{
					Allow: func(raw any) (bool, error) {
						claim := raw.(*structs.TaskGroupHostVolumeClaim)
						// empty prefix doesn't filter
						if !strings.HasPrefix(claim.ID, args.Prefix) {
							return false, nil
						}
						if args.JobID != "" && claim.JobID != args.JobID {
							return false, nil
						}
						if args.TaskGroup != "" && claim.TaskGroupName != args.TaskGroup {
							return false, nil
						}
						if args.VolumeName != "" && claim.VolumeName != args.VolumeName {
							return false, nil
						}

						if ns != structs.AllNamespacesSentinel &&
							claim.Namespace != ns {
							return false, nil
						}

						allowClaim := acl.NamespaceValidator(acl.NamespaceCapabilityHostVolumeRead)
						return allowClaim(aclObj, ns), nil
					},
				},
			}

			// Set up our output after we have checked the error.
			var claims []*structs.TaskGroupHostVolumeClaim

			// Build the paginator.
			paginatorImpl, err := paginator.NewPaginator(iter, tokenizer, filters, args.QueryOptions,
				func(raw any) error {
					claim := raw.(*structs.TaskGroupHostVolumeClaim)
					claims = append(claims, claim)
					return nil
				})
			if err != nil {
				return structs.NewErrRPCCodedf(
					http.StatusBadRequest, "failed to create result paginator: %v", err)
			}

			// Calling page populates our output array as well as returns the next token.
			nextToken, err := paginatorImpl.Page()
			if err != nil {
				return structs.NewErrRPCCodedf(
					http.StatusBadRequest, "failed to read result page: %v", err)
			}

			reply.Claims = claims
			reply.NextToken = nextToken

			// Use the index table to populate the query meta as we have no way
			// of tracking the max index on deletes.
			return tgvc.srv.setReplyQueryMeta(stateStore, state.TableTaskGroupHostVolumeClaim, &reply.QueryMeta)
		},
	}
	return tgvc.srv.blockingRPC(&opts)
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
