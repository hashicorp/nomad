package nomad

import (
	"strings"
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Scaling endpoint is used for listing and retrieving scaling policies
type Scaling struct {
	srv    *Server
	logger log.Logger
}

// ListPolicies is used to list the policies
func (p *Scaling) ListPolicies(args *structs.ScalingPolicyListRequest, reply *structs.ScalingPolicyListResponse) error {

	if done, err := p.srv.forward("Scaling.ListPolicies", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "scaling", "list_policies"}, time.Now())

	if args.RequestNamespace() == structs.AllNamespacesSentinel {
		return p.listAllNamespaces(args, reply)
	}

	if aclObj, err := p.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil {
		hasListScalingPolicies := aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityListScalingPolicies)
		hasListAndReadJobs := aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityListJobs) &&
			aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityReadJob)
		if !(hasListScalingPolicies || hasListAndReadJobs) {
			return structs.ErrPermissionDenied
		}
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Iterate over all the policies
			var err error
			var iter memdb.ResultIterator
			if prefix := args.QueryOptions.Prefix; prefix != "" {
				iter, err = state.ScalingPoliciesByIDPrefix(ws, args.RequestNamespace(), prefix)
			} else if job := args.Job; job != "" {
				iter, err = state.ScalingPoliciesByJob(ws, args.RequestNamespace(), job)
			} else {
				iter, err = state.ScalingPoliciesByNamespace(ws, args.Namespace, args.Type)
			}

			if err != nil {
				return err
			}

			// Convert all the policies to a list stub
			reply.Policies = nil
			for raw := iter.Next(); raw != nil; raw = iter.Next() {
				policy := raw.(*structs.ScalingPolicy)
				reply.Policies = append(reply.Policies, policy.Stub())
			}

			// Use the last index that affected the policy table
			index, err := state.Index("scaling_policy")
			if err != nil {
				return err
			}

			// Don't return index zero, otherwise a blocking query cannot be used.
			if index == 0 {
				index = 1
			}
			reply.Index = index
			return nil
		}}
	return p.srv.blockingRPC(&opts)
}

// GetPolicy is used to get a specific policy
func (p *Scaling) GetPolicy(args *structs.ScalingPolicySpecificRequest,
	reply *structs.SingleScalingPolicyResponse) error {

	if done, err := p.srv.forward("Scaling.GetPolicy", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "scaling", "get_policy"}, time.Now())

	// Check for list-job permissions
	if aclObj, err := p.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil {
		hasReadScalingPolicy := aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityReadScalingPolicy)
		hasListAndReadJobs := aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityListJobs) &&
			aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityReadJob)
		if !(hasReadScalingPolicy || hasListAndReadJobs) {
			return structs.ErrPermissionDenied
		}
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Iterate over all the policies
			p, err := state.ScalingPolicyByID(ws, args.ID)
			if err != nil {
				return err
			}

			reply.Policy = p

			// Use the last index that affected the policy table
			index, err := state.Index("scaling_policy")
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
	return p.srv.blockingRPC(&opts)
}

func (j *Scaling) listAllNamespaces(args *structs.ScalingPolicyListRequest, reply *structs.ScalingPolicyListResponse) error {
	// Check for list-job permissions
	aclObj, err := j.srv.ResolveToken(args.AuthToken)
	if err != nil {
		return err
	}
	prefix := args.QueryOptions.Prefix
	allow := func(ns string) bool {
		return aclObj.AllowNsOp(ns, acl.NamespaceCapabilityListScalingPolicies) ||
			(aclObj.AllowNsOp(ns, acl.NamespaceCapabilityListJobs) && aclObj.AllowNsOp(ns, acl.NamespaceCapabilityReadJob))
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// check if user has permission to all namespaces
			allowedNSes, err := allowedNSes(aclObj, state, allow)
			if err == structs.ErrPermissionDenied {
				// return empty if token isn't authorized for any namespace
				reply.Policies = []*structs.ScalingPolicyListStub{}
				return nil
			} else if err != nil {
				return err
			}

			// Capture all the policies
			var iter memdb.ResultIterator
			if args.Type != "" {
				iter, err = state.ScalingPoliciesByTypePrefix(ws, args.Type)
			} else {
				iter, err = state.ScalingPolicies(ws)
			}
			if err != nil {
				return err
			}

			var policies []*structs.ScalingPolicyListStub
			for raw := iter.Next(); raw != nil; raw = iter.Next() {
				policy := raw.(*structs.ScalingPolicy)
				if allowedNSes != nil && !allowedNSes[policy.Target[structs.ScalingTargetNamespace]] {
					// not permitted to this name namespace
					continue
				}
				if prefix != "" && !strings.HasPrefix(policy.ID, prefix) {
					continue
				}
				policies = append(policies, policy.Stub())
			}
			reply.Policies = policies

			// Use the last index that affected the policies table or summary
			index, err := state.Index("scaling_policy")
			if err != nil {
				return err
			}
			reply.Index = helper.Uint64Max(1, index)

			// Set the query response
			j.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return j.srv.blockingRPC(&opts)
}
