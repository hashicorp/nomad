// +build ent

package nomad

import (
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Sentinel endpoint is used for manipulating Sentinel policies
type Sentinel struct {
	srv *Server
}

// UpsertPolicies is used to create or update a set of policies
func (s *Sentinel) UpsertPolicies(args *structs.SentinelPolicyUpsertRequest, reply *structs.GenericResponse) error {
	// Ensure Sentinels are enabled, and always flow modification requests to the authoritative region
	if !s.srv.config.ACLEnabled {
		return aclDisabled
	}
	args.Region = s.srv.config.AuthoritativeRegion

	if done, err := s.srv.forward("Sentinel.UpsertPolicies", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "sentinel", "upsert_policies"}, time.Now())

	// Check management level permissions
	if sentinel, err := s.srv.resolveToken(args.SecretID); err != nil {
		return err
	} else if sentinel == nil || !sentinel.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Validate non-zero set of policies
	if len(args.Policies) == 0 {
		return fmt.Errorf("must specify as least one policy")
	}

	// Validate each policy, compute hash
	for idx, policy := range args.Policies {
		if err := policy.Validate(); err != nil {
			return fmt.Errorf("policy %d invalid: %v", idx, err)
		}
		policy.SetHash()
	}

	// Update via Raft
	_, index, err := s.srv.raftApply(structs.SentinelPolicyUpsertRequestType, args)
	if err != nil {
		return err
	}

	// Update the index
	reply.Index = index
	return nil
}

// DeletePolicies is used to delete policies
func (s *Sentinel) DeletePolicies(args *structs.SentinelPolicyDeleteRequest, reply *structs.GenericResponse) error {
	// Ensure Sentinels are enabled, and always flow modification requests to the authoritative region
	if !s.srv.config.ACLEnabled {
		return aclDisabled
	}
	args.Region = s.srv.config.AuthoritativeRegion

	if done, err := s.srv.forward("Sentinel.DeletePolicies", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "sentinel", "delete_policies"}, time.Now())

	// Check management level permissions
	if sentinel, err := s.srv.resolveToken(args.SecretID); err != nil {
		return err
	} else if sentinel == nil || !sentinel.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Validate non-zero set of policies
	if len(args.Names) == 0 {
		return fmt.Errorf("must specify as least one policy")
	}

	// Update via Raft
	_, index, err := s.srv.raftApply(structs.SentinelPolicyDeleteRequestType, args)
	if err != nil {
		return err
	}

	// Update the index
	reply.Index = index
	return nil
}

// ListPolicies is used to list the policies
func (s *Sentinel) ListPolicies(args *structs.SentinelPolicyListRequest, reply *structs.SentinelPolicyListResponse) error {
	if !s.srv.config.ACLEnabled {
		return aclDisabled
	}
	if done, err := s.srv.forward("Sentinel.ListPolicies", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "sentinel", "list_policies"}, time.Now())

	// Check management level permissions
	if sentinel, err := s.srv.resolveToken(args.SecretID); err != nil {
		return err
	} else if sentinel == nil || !sentinel.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, snap *state.StateStore) error {
			// Iterate over all the policies
			var err error
			var iter memdb.ResultIterator
			if prefix := args.QueryOptions.Prefix; prefix != "" {
				iter, err = snap.SentinelPolicyByNamePrefix(ws, prefix)
			} else {
				iter, err = snap.SentinelPolicies(ws)
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
				policy := raw.(*structs.SentinelPolicy)
				reply.Policies = append(reply.Policies, policy.Stub())
			}

			// Use the last index that affected the policy table
			index, err := snap.Index(state.TableSentinelPolicies)
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
	return s.srv.blockingRPC(&opts)
}

// GetPolicy is used to get a specific policy
func (s *Sentinel) GetPolicy(args *structs.SentinelPolicySpecificRequest, reply *structs.SingleSentinelPolicyResponse) error {
	if !s.srv.config.ACLEnabled {
		return aclDisabled
	}
	if done, err := s.srv.forward("Sentinel.GetPolicy", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "sentinel", "get_policy"}, time.Now())

	// Check management level permissions
	if sentinel, err := s.srv.resolveToken(args.SecretID); err != nil {
		return err
	} else if sentinel == nil || !sentinel.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, snap *state.StateStore) error {
			// Look for the policy
			out, err := snap.SentinelPolicyByName(ws, args.Name)
			if err != nil {
				return err
			}

			// Setup the output
			reply.Policy = out
			if out != nil {
				reply.Index = out.ModifyIndex
			} else {
				// Use the last index that affected the policy table
				index, err := snap.Index(state.TableSentinelPolicies)
				if err != nil {
					return err
				}
				reply.Index = index
			}
			return nil
		}}
	return s.srv.blockingRPC(&opts)
}

// GetPolicies is used to get a set of policies
func (s *Sentinel) GetPolicies(args *structs.SentinelPolicySetRequest, reply *structs.SentinelPolicySetResponse) error {
	if !s.srv.config.ACLEnabled {
		return aclDisabled
	}
	if done, err := s.srv.forward("Sentinel.GetPolicies", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "sentinel", "get_policies"}, time.Now())

	// Check management level permissions
	if sentinel, err := s.srv.resolveToken(args.SecretID); err != nil {
		return err
	} else if sentinel == nil || !sentinel.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, snap *state.StateStore) error {
			// Setup the output
			reply.Policies = make(map[string]*structs.SentinelPolicy, len(args.Names))

			// Look for the policy
			for _, policyName := range args.Names {
				out, err := snap.SentinelPolicyByName(ws, policyName)
				if err != nil {
					return err
				}
				if out != nil {
					reply.Policies[policyName] = out
				}
			}

			// Use the last index that affected the policy table
			index, err := snap.Index(state.TableSentinelPolicies)
			if err != nil {
				return err
			}
			reply.Index = index
			return nil
		}}
	return s.srv.blockingRPC(&opts)
}
