// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/state/paginator"
	"github.com/hashicorp/nomad/nomad/structs"
)

// NodePool endpoint is used for node pool management and interaction.
type NodePool struct {
	srv *Server
	ctx *RPCContext
}

func NewNodePoolEndpoint(srv *Server, ctx *RPCContext) *NodePool {
	return &NodePool{srv: srv, ctx: ctx}
}

// List is used to retrieve multiple node pools. It supports prefix listing,
// pagination, and filtering.
func (n *NodePool) List(args *structs.NodePoolListRequest, reply *structs.NodePoolListResponse) error {
	authErr := n.srv.Authenticate(n.ctx, args)
	if done, err := n.srv.forward("NodePool.List", args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("node_pool", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "node_pool", "list"}, time.Now())

	// Resolve ACL token to only return node pools it has access to.
	aclObj, err := n.srv.ResolveACL(args)
	if err != nil {
		return err
	}

	// Setup blocking query.
	sort := state.SortOption(args.Reverse)
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, store *state.StateStore) error {
			var err error
			var iter memdb.ResultIterator

			if prefix := args.QueryOptions.Prefix; prefix != "" {
				iter, err = store.NodePoolsByNamePrefix(ws, prefix, sort)
			} else {
				iter, err = store.NodePools(ws, sort)
			}
			if err != nil {
				return err
			}

			pageOpts := paginator.StructsTokenizerOptions{WithID: true}
			tokenizer := paginator.NewStructsTokenizer(iter, pageOpts)
			filters := []paginator.Filter{
				// Filter out node pools based on ACL token capabilities.
				paginator.GenericFilter{
					Allow: func(raw interface{}) (bool, error) {
						pool := raw.(*structs.NodePool)
						return aclObj.AllowNodePoolOperation(pool.Name, acl.NodePoolCapabilityRead), nil
					},
				},
			}

			var pools []*structs.NodePool
			pager, err := paginator.NewPaginator(iter, tokenizer, filters, args.QueryOptions,
				func(raw interface{}) error {
					pool := raw.(*structs.NodePool)
					pools = append(pools, pool)
					return nil
				})
			if err != nil {
				return structs.NewErrRPCCodedf(http.StatusBadRequest, "failed to create result paginator: %v", err)
			}

			nextToken, err := pager.Page()
			if err != nil {
				return structs.NewErrRPCCodedf(http.StatusBadRequest, "failed to read result page: %v", err)
			}

			reply.QueryMeta.NextToken = nextToken
			reply.NodePools = pools

			// Use the last index that affected the node pools table.
			index, err := store.Index("node_pools")
			if err != nil {
				return err
			}
			reply.Index = helper.Max(1, index)

			// Set the query response.
			n.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return n.srv.blockingRPC(&opts)
}

// GetNodePool returns the specific node pool requested or nil if the node pool
// doesn't exist.
func (n *NodePool) GetNodePool(args *structs.NodePoolSpecificRequest, reply *structs.SingleNodePoolResponse) error {
	authErr := n.srv.Authenticate(n.ctx, args)
	if done, err := n.srv.forward("NodePool.GetNodePool", args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("node_pool", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "node_pool", "get_node_pool"}, time.Now())

	// Resolve ACL token and verify it has read capability for the pool.
	aclObj, err := n.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if !aclObj.AllowNodePoolOperation(args.Name, acl.NodePoolCapabilityRead) {
		return structs.ErrPermissionDenied
	}

	// Setup the blocking query.
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, store *state.StateStore) error {
			// Fetch node pool.
			pool, err := store.NodePoolByName(ws, args.Name)
			if err != nil {
				return err
			}

			reply.NodePool = pool
			if pool != nil {
				reply.Index = pool.ModifyIndex
			} else {
				// Return the last index that affected the node pools table if
				// the requested node pool doesn't exist.
				index, err := store.Index(state.TableNodePools)
				if err != nil {
					return err
				}
				reply.Index = helper.Max(1, index)
			}
			return nil
		}}
	return n.srv.blockingRPC(&opts)
}

// UpsertNodePools creates or updates the given node pools. Built-in node pools
// cannot be updated.
func (n *NodePool) UpsertNodePools(args *structs.NodePoolUpsertRequest, reply *structs.GenericResponse) error {
	authErr := n.srv.Authenticate(n.ctx, args)
	args.Region = n.srv.config.AuthoritativeRegion
	if done, err := n.srv.forward("NodePool.UpsertNodePools", args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("node_pool", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "node_pool", "upsert_node_pools"}, time.Now())

	// Resolve ACL token and verify it has write capability to all pools in the
	// request.
	aclObj, err := n.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	for _, pool := range args.NodePools {
		if !aclObj.AllowNodePoolOperation(pool.Name, acl.NodePoolCapabilityWrite) {
			return structs.ErrPermissionDenied
		}
	}

	// Validate request.
	if len(args.NodePools) == 0 {
		return structs.NewErrRPCCodedf(http.StatusBadRequest, "must specify at least one node pool")
	}
	for _, pool := range args.NodePools {
		if err := pool.Validate(); err != nil {
			return structs.NewErrRPCCodedf(http.StatusBadRequest, "invalid node pool %q: %v", pool.Name, err)
		}
		if pool.IsBuiltIn() {
			return structs.NewErrRPCCodedf(http.StatusBadRequest, "modifying node pool %q is not allowed", pool.Name)
		}
	}

	// Update via Raft.
	_, index, err := n.srv.raftApply(structs.NodePoolUpsertRequestType, args)
	if err != nil {
		return err
	}
	reply.Index = index
	return nil
}

// DeleteNodePools deletes the given node pools. Built-in node pools cannot be
// deleted.
func (n *NodePool) DeleteNodePools(args *structs.NodePoolDeleteRequest, reply *structs.GenericResponse) error {
	authErr := n.srv.Authenticate(n.ctx, args)
	args.Region = n.srv.config.AuthoritativeRegion
	if done, err := n.srv.forward("NodePool.DeleteNodePools", args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("node_pool", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "node_pool", "delete_node_pools"}, time.Now())

	// Resolve ACL token and verify it has delete capability to all pools in
	// the request.
	aclObj, err := n.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	for _, name := range args.Names {
		if !aclObj.AllowNodePoolOperation(name, acl.NodePoolCapabilityDelete) {
			return structs.ErrPermissionDenied
		}
	}

	// Validate request.
	if len(args.Names) == 0 {
		return structs.NewErrRPCCodedf(http.StatusBadRequest, "must specify at least one node pool to delete")
	}
	for _, name := range args.Names {
		if name == "" {
			return structs.NewErrRPCCodedf(http.StatusBadRequest, "node pool name is empty")
		}
	}

	// TODO(luiz): verify that the node pool is not being used.

	// Delete via Raft.
	_, index, err := n.srv.raftApply(structs.NodePoolDeleteRequestType, args)
	if err != nil {
		return err
	}
	reply.Index = index
	return nil
}

// ListJobs is used to retrieve a list of jobs for a given node pool. It supports
// pagination and filtering.
func (n *NodePool) ListJobs(args *structs.NodePoolJobsRequest, reply *structs.NodePoolJobsResponse) error {
	authErr := n.srv.Authenticate(n.ctx, args)
	if done, err := n.srv.forward("NodePool.ListJobs", args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("node_pool", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "node_pool", "list_jobs"}, time.Now())

	// Resolve ACL token and verify it has read capability for the pool.
	aclObj, err := n.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if !aclObj.AllowNodePoolOperation(args.Name, acl.NodePoolCapabilityRead) {
		return structs.ErrPermissionDenied
	}
	allowNsFunc := aclObj.AllowNsOpFunc(acl.NamespaceCapabilityListJobs)
	namespace := args.RequestNamespace()

	// Setup the blocking query. This largely mirrors the Jobs.List RPC but with
	// an additional paginator filter for the node pool.
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, store *state.StateStore) error {
			// ensure the node pool exists
			pool, err := store.NodePoolByName(ws, args.Name)
			if err != nil {
				return err
			}
			if pool == nil {
				return nil
			}

			var iter memdb.ResultIterator

			// Get the namespaces the user is allowed to access.
			allowableNamespaces, err := allowedNSes(aclObj, store, allowNsFunc)
			if errors.Is(err, structs.ErrPermissionDenied) {
				// return empty jobs if token isn't authorized for any
				// namespace, matching other endpoints
				reply.Jobs = make([]*structs.JobListStub, 0)
			} else if err != nil {
				return err
			} else {

				filters := []paginator.Filter{
					paginator.NamespaceFilter{
						AllowableNamespaces: allowableNamespaces,
					},
				}

				if namespace == structs.AllNamespacesSentinel {
					iter, err = store.JobsByPool(ws, args.Name)
				} else {
					iter, err = store.JobsByNamespace(ws, namespace)
					filters = append(filters,
						paginator.GenericFilter{
							Allow: func(raw interface{}) (bool, error) {
								job := raw.(*structs.Job)
								if job == nil || job.NodePool != args.Name {
									return false, nil
								}
								return true, nil
							},
						})
				}
				if err != nil {
					return err
				}

				tokenizer := paginator.NewStructsTokenizer(
					iter,
					paginator.StructsTokenizerOptions{
						WithNamespace: true,
						WithID:        true,
					},
				)

				var jobs []*structs.JobListStub

				paginator, err := paginator.NewPaginator(iter, tokenizer, filters, args.QueryOptions,
					func(raw interface{}) error {
						job := raw.(*structs.Job)
						summary, err := store.JobSummaryByID(ws, job.Namespace, job.ID)
						if err != nil || summary == nil {
							return fmt.Errorf("unable to look up summary for job: %v", job.ID)
						}
						jobs = append(jobs, job.Stub(summary, args.Fields))
						return nil
					})
				if err != nil {
					return structs.NewErrRPCCodedf(
						http.StatusBadRequest, "failed to create result paginator: %v", err)
				}

				nextToken, err := paginator.Page()
				if err != nil {
					return structs.NewErrRPCCodedf(
						http.StatusBadRequest, "failed to read result page: %v", err)
				}

				reply.QueryMeta.NextToken = nextToken
				reply.Jobs = jobs
			}

			// Use the last index that affected the jobs table or summary
			jindex, err := store.Index("jobs")
			if err != nil {
				return err
			}
			sindex, err := store.Index("job_summary")
			if err != nil {
				return err
			}
			reply.Index = helper.Max(jindex, sindex)

			// Set the query response
			n.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return n.srv.blockingRPC(&opts)
}
