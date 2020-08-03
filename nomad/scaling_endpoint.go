package nomad

import (
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Scaling endpoint is used for listing and retrieving scaling policies
type Scaling struct {
	srv    *Server
	logger log.Logger
}

// ListPolicies is used to list the policies
func (a *Scaling) ListPolicies(args *structs.ScalingPolicyListRequest,
	reply *structs.ScalingPolicyListResponse) error {

	if done, err := a.srv.forward("Scaling.ListPolicies", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "scaling", "list_policies"}, time.Now())

	// Check for list-job permissions
	if aclObj, err := a.srv.ResolveToken(args.AuthToken); err != nil {
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
			iter, err := state.ScalingPoliciesByNamespace(ws, args.Namespace)
			if err != nil {
				return err
			}

			// Convert all the policies to a list stub
			reply.Policies = nil
			for {
				raw := iter.Next()
				if raw == nil {
					break
				}
				policy := raw.(*structs.ScalingPolicy)
				// if _, ok := policies[policy.Target]; ok || mgt {
				// 	reply.Policies = append(reply.Policies, policy.Stub())
				// }
				reply.Policies = append(reply.Policies, policy.Stub())
			}

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
	return a.srv.blockingRPC(&opts)
}

// GetPolicy is used to get a specific policy
func (a *Scaling) GetPolicy(args *structs.ScalingPolicySpecificRequest,
	reply *structs.SingleScalingPolicyResponse) error {

	if done, err := a.srv.forward("Scaling.GetPolicy", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "scaling", "get_policy"}, time.Now())

	// Check for list-job permissions
	if aclObj, err := a.srv.ResolveToken(args.AuthToken); err != nil {
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
	return a.srv.blockingRPC(&opts)
}
