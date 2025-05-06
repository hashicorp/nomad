// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/go-memdb"
	metrics "github.com/hashicorp/go-metrics/compat"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/acl"
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

	// Only warn for expiration of a read request.
	_ = n.validateLicense(nil)

	// Setup blocking query.
	sort := state.SortOption(args.Reverse)
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, store *state.StateStore) error {
			var iter state.ResultIterator[*structs.NodePool]

			if prefix := args.QueryOptions.Prefix; prefix != "" {
				iter = store.NodePoolsByNamePrefix(ws, prefix, sort)
			} else {
				iter = store.NodePools(ws, sort)
			}

			selector := func(pool *structs.NodePool) bool {
				return aclObj.AllowNodePoolOperation(pool.Name, acl.NodePoolCapabilityRead)
			}

			pager, err := paginator.NewPaginator(iter, args.QueryOptions, selector,
				paginator.IDTokenizer[*structs.NodePool](args.NextToken),
				(*structs.NodePool).Stub)
			if err != nil {
				return structs.NewErrRPCCodedf(http.StatusBadRequest, "failed to create result paginator: %v", err)
			}

			pools, nextToken, err := pager.Page()
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
			reply.Index = max(1, index)

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

	// Only warn for expiration of a read request.
	_ = n.validateLicense(nil)

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
				reply.Index = max(1, index)
			}
			return nil
		}}
	return n.srv.blockingRPC(&opts)
}

// UpsertNodePools creates or updates the given node pools. Built-in node pools
// cannot be updated.
func (n *NodePool) UpsertNodePools(args *structs.NodePoolUpsertRequest, reply *structs.GenericResponse) error {
	authErr := n.srv.Authenticate(n.ctx, args)
	if n.srv.config.ACLEnabled || args.Region == "" {
		// only forward to the authoritative region if ACLs are enabled,
		// otherwise we silently write to the local region
		args.Region = n.srv.config.AuthoritativeRegion
	}
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

		// Strict enforcement for write requests.
		// If not licensed then requests will be denied.
		if err := n.validateLicense(pool); err != nil {
			return err
		}
	}

	if !ServersMeetMinimumVersion(
		n.srv.serf.Members(), n.srv.Region(), minNodePoolsVersion, true) {
		return fmt.Errorf("all servers must be running version %v or later to upsert node pools", minNodePoolsVersion)
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

		pool.SetHash()
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
	if n.srv.config.ACLEnabled || args.Region == "" {
		// only forward to the authoritative region if ACLs are enabled,
		// otherwise we silently write to the local region
		args.Region = n.srv.config.AuthoritativeRegion
	}
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

	// Only warn for expiration on delete because just parts of node pools are
	// licensed, so they are allowed to be deleted.
	_ = n.validateLicense(nil)

	if !ServersMeetMinimumVersion(
		n.srv.serf.Members(), n.srv.Region(), minNodePoolsVersion, true) {
		return fmt.Errorf("all servers must be running version %v or later to delete node pools", minNodePoolsVersion)
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

	// Verify that the node pools we're deleting do not have nodes or
	// non-terminal jobs in this region or in any federated region.
	var mErr multierror.Error
	for _, name := range args.Names {
		regionsWithNonTerminal, regionsWithNodes, err := n.nodePoolRegionsInUse(args.AuthToken, name)
		if err != nil {
			_ = multierror.Append(&mErr, err)
		}
		if len(regionsWithNonTerminal) != 0 {
			_ = multierror.Append(&mErr, fmt.Errorf(
				"node pool %q has non-terminal jobs in regions: %v", name, regionsWithNonTerminal))
		}
		if len(regionsWithNodes) != 0 {
			_ = multierror.Append(&mErr, fmt.Errorf(
				"node pool %q has nodes in regions: %v", name, regionsWithNodes))
		}
	}

	if err := mErr.ErrorOrNil(); err != nil {
		return err
	}

	// Delete via Raft.
	_, index, err := n.srv.raftApply(structs.NodePoolDeleteRequestType, args)
	if err != nil {
		return err
	}

	reply.Index = index
	return nil
}

// nodePoolRegionsInUse returns a list of regions where the node pool is still
// in use for non-terminal jobs, and a list of regions where it is in use by
// nodes.
func (n *NodePool) nodePoolRegionsInUse(token, poolName string) ([]string, []string, error) {
	regions := n.srv.Regions()
	thisRegion := n.srv.Region()
	hasNodes := make([]string, 0, len(regions))
	hasNonTerminal := make([]string, 0, len(regions))

	// Check if the pool in use in this region
	snap, err := n.srv.State().Snapshot()
	if err != nil {
		return nil, nil, err
	}
	iter := snap.NodesByNodePool(nil, poolName)
	found := iter.Next()
	if found != nil {
		hasNodes = append(hasNodes, thisRegion)
	}
	jobs := snap.JobsByPool(nil, poolName)
	for job := range jobs.All() {
		if job.Status != structs.JobStatusDead {
			hasNonTerminal = append(hasNonTerminal, thisRegion)
			break
		}
	}

	for _, region := range regions {
		if region == thisRegion {
			continue
		}

		nodesReq := &structs.NodePoolNodesRequest{
			Name: poolName,
			QueryOptions: structs.QueryOptions{
				Region:    region,
				AuthToken: token,
				PerPage:   1, // we only care if there are any
			},
		}
		var nodesResp structs.NodePoolNodesResponse
		err := n.srv.RPC("NodePool.ListNodes", nodesReq, &nodesResp)
		if err != nil {
			return hasNodes, hasNonTerminal, err
		}
		if len(nodesResp.Nodes) != 0 {
			hasNodes = append(hasNodes, region)
		}

		jobsReq := &structs.NodePoolJobsRequest{
			Name: poolName,
			QueryOptions: structs.QueryOptions{
				Region:    region,
				AuthToken: token,
				PerPage:   1, // we only care if there are any
				Filter:    `Status != "dead"`,
			},
		}
		var jobsResp structs.NodePoolJobsResponse
		err = n.srv.RPC("NodePool.ListJobs", jobsReq, &jobsResp)
		if err != nil {
			return hasNodes, hasNonTerminal, err
		}

		if len(jobsResp.Jobs) != 0 {
			hasNonTerminal = append(hasNonTerminal, region)
		}

	}

	return hasNonTerminal, hasNodes, err
}

// ListJobs is used to retrieve a list of jobs for a given node pool. It supports
// pagination and filtering.
func (n *NodePool) ListJobs(args *structs.NodePoolJobsRequest, reply *structs.NodePoolJobsResponse) error {
	authErr := n.srv.Authenticate(n.ctx, args)
	if done, err := n.srv.forward("NodePool.ListJobs", args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("node_pool", structs.RateMetricList, args)
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
	sort := state.QueryOptionSort(args.QueryOptions)

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

			var iter state.ResultIterator[*structs.Job]

			// Get the namespaces the user is allowed to access.
			allowableNamespaces, err := allowedNSes(aclObj, store, allowNsFunc)
			if errors.Is(err, structs.ErrPermissionDenied) {
				// return empty jobs if token isn't authorized for any
				// namespace, matching other endpoints
				reply.Jobs = make([]*structs.JobListStub, 0)
			} else if err != nil {
				return err
			} else {
				var selector paginator.SelectorFunc[*structs.Job]

				if namespace == structs.AllNamespacesSentinel {
					iter = store.JobsByPool(ws, args.Name)
					selector = paginator.NamespaceSelectorFunc[*structs.Job](allowableNamespaces)
				} else {
					iter = store.JobsByNamespace(ws, namespace, sort)
					selector = func(job *structs.Job) bool {
						if allowableNamespaces != nil && !allowableNamespaces[job.Namespace] {
							return false
						}
						if job.NodePool != args.Name {
							return false
						}
						return true
					}
				}

				pager, err := paginator.NewPaginator(iter, args.QueryOptions, selector,
					paginator.NamespaceIDTokenizer[*structs.Job](args.NextToken),
					func(job *structs.Job) (*structs.JobListStub, error) {
						summary, err := store.JobSummaryByID(ws, job.Namespace, job.ID)
						if err != nil || summary == nil {
							return nil, fmt.Errorf("unable to look up summary for job: %v", job.ID)
						}
						return job.Stub(summary, args.Fields), nil
					})
				if err != nil {
					return structs.NewErrRPCCodedf(
						http.StatusBadRequest, "failed to create result paginator: %v", err)
				}

				jobs, nextToken, err := pager.Page()
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
			reply.Index = max(jindex, sindex)

			// Set the query response
			n.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return n.srv.blockingRPC(&opts)
}

// ListNodes is used to retrieve a list of nodes for a give node pool. It
// supports pagination and filtering.
func (n *NodePool) ListNodes(args *structs.NodePoolNodesRequest, reply *structs.NodePoolNodesResponse) error {
	authErr := n.srv.Authenticate(n.ctx, args)
	if done, err := n.srv.forward("NodePool.ListNodes", args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("node_pool", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "node_pool", "list_nodes"}, time.Now())

	// Resolve ACL token and verify it has read capability for nodes and the
	// node pool.
	aclObj, err := n.srv.ResolveACL(args)
	if err != nil {
		return err
	}

	allowed := aclObj.AllowNodeRead() &&
		aclObj.AllowNodePoolOperation(args.Name, acl.NodePoolCapabilityRead)
	if !allowed {
		return structs.ErrPermissionDenied
	}

	// Setup blocking query.
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, store *state.StateStore) error {
			// Verify node pool exists.
			pool, err := store.NodePoolByName(ws, args.Name)
			if err != nil {
				return err
			}
			if pool == nil {
				return nil
			}

			// Fetch nodes in the pool.
			var iter state.ResultIterator[*structs.Node]
			if args.Name == structs.NodePoolAll {
				iter = store.Nodes(ws)
			} else {
				iter = store.NodesByNodePool(ws, args.Name)
			}

			pager, err := paginator.NewPaginator(iter, args.QueryOptions, nil,
				paginator.IDTokenizer[*structs.Node](args.NextToken),
				func(node *structs.Node) (*structs.NodeListStub, error) {
					return node.Stub(args.Fields), nil
				})
			if err != nil {
				return structs.NewErrRPCCodedf(http.StatusBadRequest, "failed to create result paginator: %v", err)
			}

			nodes, nextToken, err := pager.Page()
			if err != nil {
				return structs.NewErrRPCCodedf(http.StatusBadRequest, "failed to read result page: %v", err)
			}

			reply.QueryMeta.NextToken = nextToken
			reply.Nodes = nodes

			// Use the last index that affected the nodes table.
			index, err := store.Index("nodes")
			if err != nil {
				return err
			}
			reply.Index = max(1, index)

			// Set the query response.
			n.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return n.srv.blockingRPC(&opts)
}
