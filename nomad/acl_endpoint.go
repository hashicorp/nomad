package nomad

import (
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ACL endpoint is used for manipulating ACL tokens and policies
type ACL struct {
	srv *Server
}

// UpsertPolicies is used to create or update a set of policies
func (a *ACL) UpsertPolicies(args *structs.ACLPolicyUpsertRequest, reply *structs.GenericResponse) error {
	if done, err := a.srv.forward("ACL.UpsertPolicies", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "upsert_policies"}, time.Now())

	// Validate non-zero set of policies
	if len(args.Policies) == 0 {
		return fmt.Errorf("must specify as least one policy")
	}

	// Validate each policy
	for idx, policy := range args.Policies {
		if err := policy.Validate(); err != nil {
			return fmt.Errorf("policy %d invalid: %v", idx, err)
		}
	}

	// Update via Raft
	_, index, err := a.srv.raftApply(structs.ACLPolicyUpsertRequestType, args)
	if err != nil {
		return err
	}

	// Update the index
	reply.Index = index
	return nil
}

// DeletePolicies is used to delete policies
func (a *ACL) DeletePolicies(args *structs.ACLPolicyDeleteRequest, reply *structs.GenericResponse) error {
	if done, err := a.srv.forward("ACL.DeletePolicies", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "delete_policies"}, time.Now())

	// Validate non-zero set of policies
	if len(args.Names) == 0 {
		return fmt.Errorf("must specify as least one policy")
	}

	// Update via Raft
	_, index, err := a.srv.raftApply(structs.ACLPolicyDeleteRequestType, args)
	if err != nil {
		return err
	}

	// Update the index
	reply.Index = index
	return nil
}

// ListPolicies is used to list the policies
func (a *ACL) ListPolicies(args *structs.ACLPolicyListRequest, reply *structs.ACLPolicyListResponse) error {
	if done, err := a.srv.forward("ACL.ListPolicies", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "list_policies"}, time.Now())

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Iterate over all the policies
			var err error
			var iter memdb.ResultIterator
			if prefix := args.QueryOptions.Prefix; prefix != "" {
				iter, err = state.ACLPolicyByNamePrefix(ws, prefix)
			} else {
				iter, err = state.ACLPolicies(ws)
			}
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
				policy := raw.(*structs.ACLPolicy)
				reply.Policies = append(reply.Policies, policy.Stub())
			}

			// Use the last index that affected the policy table
			index, err := state.Index("acl_policy")
			if err != nil {
				return err
			}
			reply.Index = index
			return nil
		}}
	return a.srv.blockingRPC(&opts)
}

// GetPolicy is used to get a specific policy
func (a *ACL) GetPolicy(args *structs.ACLPolicySpecificRequest, reply *structs.SingleACLPolicyResponse) error {
	if done, err := a.srv.forward("ACL.GetPolicy", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "get_policy"}, time.Now())

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Look for the policy
			out, err := state.ACLPolicyByName(ws, args.Name)
			if err != nil {
				return err
			}

			// Setup the output
			reply.Policy = out
			if out != nil {
				reply.Index = out.ModifyIndex
			} else {
				// Use the last index that affected the policy table
				index, err := state.Index("acl_policy")
				if err != nil {
					return err
				}
				reply.Index = index
			}
			return nil
		}}
	return a.srv.blockingRPC(&opts)
}
