// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"slices"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Namespace endpoint is used for manipulating namespaces
type Namespace struct {
	srv *Server
	ctx *RPCContext
}

func NewNamespaceEndpoint(srv *Server, ctx *RPCContext) *Namespace {
	return &Namespace{srv: srv, ctx: ctx}
}

// UpsertNamespaces is used to upsert a set of namespaces
func (n *Namespace) UpsertNamespaces(args *structs.NamespaceUpsertRequest,
	reply *structs.GenericResponse) error {

	authErr := n.srv.Authenticate(n.ctx, args)
	if n.srv.config.ACLEnabled || args.Region == "" {
		// only forward to the authoritative region if ACLs are enabled,
		// otherwise we silently write to the local region
		args.Region = n.srv.config.AuthoritativeRegion
	}
	if done, err := n.srv.forward("Namespace.UpsertNamespaces", args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("namespace", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	defer metrics.MeasureSince([]string{"nomad", "namespace", "upsert_namespaces"}, time.Now())

	// Check management permissions
	if aclObj, err := n.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Validate there is at least one namespace
	if len(args.Namespaces) == 0 {
		return fmt.Errorf("must specify at least one namespace")
	}

	// Validate the namespaces and set the hash
	for _, ns := range args.Namespaces {
		if err := ns.Validate(); err != nil {
			return fmt.Errorf("Invalid namespace %q: %v", ns.Name, err)
		}

		ns.SetHash()
	}

	// Update via Raft
	_, index, err := n.srv.raftApply(structs.NamespaceUpsertRequestType, args)
	if err != nil {
		return err
	}

	// Update the index
	reply.Index = index
	return nil
}

// DeleteNamespaces is used to delete a namespace
func (n *Namespace) DeleteNamespaces(args *structs.NamespaceDeleteRequest, reply *structs.GenericResponse) error {

	authErr := n.srv.Authenticate(n.ctx, args)
	if n.srv.config.ACLEnabled || args.Region == "" {
		// only forward to the authoritative region if ACLs are enabled,
		// otherwise we silently write to the local region
		args.Region = n.srv.config.AuthoritativeRegion
	}
	if done, err := n.srv.forward("Namespace.DeleteNamespaces", args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("namespace", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "namespace", "delete_namespaces"}, time.Now())

	// Check management permissions
	if aclObj, err := n.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Validate at least one namespace
	if len(args.Namespaces) == 0 {
		return fmt.Errorf("must specify at least one namespace to delete")
	}

	for _, ns := range args.Namespaces {
		if ns == structs.DefaultNamespace {
			return fmt.Errorf("can not delete default namespace")
		}
	}

	// Check that the deleting namespaces do not have non-terminal jobs in both
	// this region and all federated regions
	var mErr multierror.Error
	for _, ns := range args.Namespaces {
		nonTerminal, err := n.nonTerminalNamespaces(args.AuthToken, ns)
		if err != nil {
			_ = multierror.Append(&mErr, err)
		} else if len(nonTerminal) != 0 {
			_ = multierror.Append(&mErr, fmt.Errorf("namespace %q has non-terminal jobs in regions: %v", ns, nonTerminal))
		}
	}

	if err := mErr.ErrorOrNil(); err != nil {
		return err
	}

	// Update via Raft
	_, index, err := n.srv.raftApply(structs.NamespaceDeleteRequestType, args)
	if err != nil {
		return err
	}

	// Update the index
	reply.Index = index
	return nil
}

// nonTerminalNamespaces returns whether the set of regions in which the
// namespaces contains non-terminal jobs, allocations, volumes, variables or
// quotas, checking all federated regions including this one.
func (n *Namespace) nonTerminalNamespaces(authToken, namespace string) ([]string, error) {
	regions := n.srv.Regions()
	thisRegion := n.srv.Region()
	terminal := make([]string, 0, len(regions))

	// Check if this region is terminal
	localTerminal, err := n.namespaceTerminalLocally(namespace)
	if err != nil {
		return nil, err
	}
	if !localTerminal {
		terminal = append(terminal, thisRegion)
	}

	for _, region := range regions {
		if region == thisRegion {
			continue
		}

		remoteTerminal, err := n.namespaceTerminalInRegion(authToken, namespace, region)
		if err != nil {
			return nil, err
		}
		if !remoteTerminal {
			terminal = append(terminal, region)
		}
	}

	return terminal, nil
}

// namespaceTerminalLocally returns true if the namespace contains only
// terminal jobs, allocations, volumes or variables in the local region.
func (n *Namespace) namespaceTerminalLocally(namespace string) (bool, error) {
	snap, err := n.srv.fsm.State().Snapshot()
	if err != nil {
		return false, err
	}

	// check for jobs
	iter, err := snap.JobsByNamespace(nil, namespace, state.SortDefault)
	if err != nil {
		return false, err
	}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		job := raw.(*structs.Job)
		if job.Status != structs.JobStatusDead {
			return false, nil
		}
	}

	// check for allocs
	iter, err = snap.AllocsByNamespace(nil, namespace)
	if err != nil {
		return false, err
	}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		alloc := raw.(*structs.Allocation)
		if !slices.Contains(terminalAllocationStatuses, alloc.ClientStatus) {
			return false, nil
		}
	}

	// check for volumes
	iter, err = snap.CSIVolumesByNamespace(nil, namespace, "")
	if err != nil {
		return false, err
	}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		vol := raw.(*structs.CSIVolume)
		if vol.Namespace == namespace {
			return false, nil
		}
	}

	// check for variables
	iter, err = snap.GetVariablesByNamespace(nil, namespace)
	if err != nil {
		return false, err
	}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		v := raw.(*structs.VariableEncrypted)
		if v.VariableMetadata.Namespace == namespace {
			return false, nil
		}
	}

	return true, nil
}

// if an alloc has one of these statuses, we consider it terminal
var terminalAllocationStatuses = []string{
	structs.AllocClientStatusComplete,
	structs.AllocClientStatusLost,
	structs.AllocClientStatusFailed,
	structs.AllocClientStatusUnknown,
}

// namespaceTerminalInRegion returns true if the namespace contains only
// terminal jobs, allocations, volumes or variables in the given region.
func (n *Namespace) namespaceTerminalInRegion(authToken, namespace, region string) (bool, error) {
	jobReq := &structs.JobListRequest{
		QueryOptions: structs.QueryOptions{
			Region:     region,
			Namespace:  namespace,
			AllowStale: false,
			AuthToken:  authToken,
		},
	}

	var jobResp structs.JobListResponse
	done, err := n.srv.forward("Job.List", jobReq, jobReq, &jobResp)
	if !done {
		return false, fmt.Errorf("unexpectedly did not forward Job.List to region %q", region)
	} else if err != nil {
		return false, err
	}

	for _, job := range jobResp.Jobs {
		if job.Status != structs.JobStatusDead {
			return false, nil
		}
	}

	// check allocations
	allocReq := &structs.AllocListRequest{
		QueryOptions: structs.QueryOptions{
			Region:     region,
			Namespace:  namespace,
			AllowStale: false,
			AuthToken:  authToken,
		},
	}

	var allocResp structs.AllocListResponse
	done, err = n.srv.forward("Alloc.List", allocReq, allocReq, &allocResp)
	if !done {
		return false, fmt.Errorf("unexpectedly did not forward Alloc.List to region %q", region)
	} else if err != nil {
		return false, err
	}

	for _, alloc := range allocResp.Allocations {
		if !slices.Contains(terminalAllocationStatuses, alloc.ClientStatus) {
			return false, nil
		}
	}

	// check volumes
	volumesReq := &structs.CSIVolumeListRequest{
		QueryOptions: structs.QueryOptions{
			Region:     region,
			Namespace:  namespace,
			AllowStale: false,
			AuthToken:  authToken,
		},
	}

	var volumesResp structs.CSIVolumeListResponse
	done, err = n.srv.forward("CSIVolume.List", volumesReq, volumesReq, &volumesResp)
	if !done {
		return false, fmt.Errorf("unexpectedly did not forward CSIVolume.List to region %q", region)
	} else if err != nil {
		return false, err
	}

	for _, volume := range volumesResp.Volumes {
		if volume.Namespace == namespace {
			return false, nil
		}
	}

	// check variables
	varReq := &structs.VariablesListRequest{
		QueryOptions: structs.QueryOptions{
			Region:     region,
			Namespace:  namespace,
			AllowStale: false,
			AuthToken:  authToken,
		},
	}

	var varResp structs.VariablesListResponse
	done, err = n.srv.forward("Variables.List", varReq, varReq, &varResp)
	if !done {
		return false, fmt.Errorf("unexpectedly did not forward Variables.List to region %q", region)
	} else if err != nil {
		return false, err
	}

	for _, v := range varResp.Data {
		if v.Namespace == namespace {
			return false, nil
		}
	}

	return true, nil
}

// ListNamespaces is used to list the namespaces
func (n *Namespace) ListNamespaces(args *structs.NamespaceListRequest, reply *structs.NamespaceListResponse) error {

	authErr := n.srv.Authenticate(n.ctx, args)
	if done, err := n.srv.forward("Namespace.ListNamespaces", args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("namespace", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "namespace", "list_namespace"}, time.Now())

	// Resolve token to acl to filter namespace list
	aclObj, err := n.srv.ResolveACL(args)
	if err != nil {
		return err
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, s *state.StateStore) error {
			// Iterate over all the namespaces
			var err error
			var iter memdb.ResultIterator
			if prefix := args.QueryOptions.Prefix; prefix != "" {
				iter, err = s.NamespacesByNamePrefix(ws, prefix)
			} else {
				iter, err = s.Namespaces(ws)
			}
			if err != nil {
				return err
			}

			reply.Namespaces = nil
			for {
				raw := iter.Next()
				if raw == nil {
					break
				}
				ns := raw.(*structs.Namespace)

				// Only return namespaces allowed by acl
				if aclObj.AllowNamespace(ns.Name) {
					reply.Namespaces = append(reply.Namespaces, ns)
				}
			}

			// Use the last index that affected the namespace table
			index, err := s.Index(state.TableNamespaces)
			if err != nil {
				return err
			}

			// Ensure we never set the index to zero, otherwise a blocking query cannot be used.
			// We floor the index at one, since realistically the first write must have a higher index.
			if index == 0 {
				index = 1
			}
			reply.Index = index
			return nil
		}}
	return n.srv.blockingRPC(&opts)
}

// GetNamespace is used to get a specific namespace
func (n *Namespace) GetNamespace(args *structs.NamespaceSpecificRequest, reply *structs.SingleNamespaceResponse) error {

	authErr := n.srv.Authenticate(n.ctx, args)
	if done, err := n.srv.forward("Namespace.GetNamespace", args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("namespace", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "namespace", "get_namespace"}, time.Now())

	// Check capabilities for the given namespace permissions
	if aclObj, err := n.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.AllowNamespace(args.Name) {
		return structs.ErrPermissionDenied
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, s *state.StateStore) error {
			// Look for the namespace
			out, err := s.NamespaceByName(ws, args.Name)
			if err != nil {
				return err
			}

			// Setup the output
			reply.Namespace = out
			if out != nil {
				reply.Index = out.ModifyIndex
			} else {
				// Use the last index that affected the namespace table
				index, err := s.Index(state.TableNamespaces)
				if err != nil {
					return err
				}

				// Ensure we never set the index to zero, otherwise a blocking query cannot be used.
				// We floor the index at one, since realistically the first write must have a higher index.
				if index == 0 {
					index = 1
				}
				reply.Index = index
			}
			return nil
		}}
	return n.srv.blockingRPC(&opts)
}

// GetNamespaces is used to get a set of namespaces
func (n *Namespace) GetNamespaces(args *structs.NamespaceSetRequest, reply *structs.NamespaceSetResponse) error {

	authErr := n.srv.Authenticate(n.ctx, args)
	if done, err := n.srv.forward("Namespace.GetNamespaces", args, args, reply); done {
		return err
	}
	n.srv.MeasureRPCRate("namespace", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "namespace", "get_namespaces"}, time.Now())

	// Check management permissions
	if aclObj, err := n.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, s *state.StateStore) error {
			// Setup the output
			reply.Namespaces = make(map[string]*structs.Namespace, len(args.Namespaces))

			// Look for the namespace
			for _, namespace := range args.Namespaces {
				out, err := s.NamespaceByName(ws, namespace)
				if err != nil {
					return err
				}
				if out != nil {
					reply.Namespaces[namespace] = out
				}
			}

			// Use the last index that affected the policy table
			index, err := s.Index(state.TableNamespaces)
			if err != nil {
				return err
			}

			// Ensure we never set the index to zero, otherwise a blocking query cannot be used.
			// We floor the index at one, since realistically the first write must have a higher index.
			if index == 0 {
				index = 1
			}
			reply.Index = index
			return nil
		}}
	return n.srv.blockingRPC(&opts)
}
