// +build ent

package nomad

import (
	"bytes"
	"context"
	"time"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"golang.org/x/time/rate"
)

// establishEnterpriseLeadership is used to instantiate Nomad Pro and Premium
// systems upon acquiring leadership.
func (s *Server) establishEnterpriseLeadership(stopCh chan struct{}) error {
	if err := s.establishProLeadership(stopCh); err != nil {
		return err
	}

	// If we are not the authoritative region, start replicating.
	if s.config.Region != s.config.AuthoritativeRegion {
		// Start replication of Sentinel Policies if ACLs are enabled,
		// and we are not the authoritative region.
		if s.config.ACLEnabled {
			go s.replicateSentinelPolicies(stopCh)
		}

		// Start replciating quota specifications.
		go s.replicateQuotaSpecs(stopCh)
	}
	return nil
}

// revokeEnterpriseLeadership is used to disable Nomad Pro and Premium systems
// upon losing leadership.
func (s *Server) revokeEnterpriseLeadership() error {
	if err := s.revokeProLeadership(); err != nil {
		return err
	}
	return nil
}

// replicateSentinelPolicies is used to replicate Sentinel policies from
// the authoritative region to this region.
func (s *Server) replicateSentinelPolicies(stopCh chan struct{}) {
	req := structs.SentinelPolicyListRequest{
		QueryOptions: structs.QueryOptions{
			Region:     s.config.AuthoritativeRegion,
			AllowStale: true,
		},
	}
	limiter := rate.NewLimiter(replicationRateLimit, int(replicationRateLimit))
	s.logger.Debug("starting Sentinel policy replication from authoritative region", "region", req.Region)

START:
	for {
		select {
		case <-stopCh:
			return
		default:
		}

		// Rate limit how often we attempt replication
		limiter.Wait(context.Background())

		// Fetch the list of policies
		var resp structs.SentinelPolicyListResponse
		req.AuthToken = s.ReplicationToken()
		err := s.forwardRegion(s.config.AuthoritativeRegion,
			"Sentinel.ListPolicies", &req, &resp)
		if err != nil {
			s.logger.Debug("failed to fetch policies from authoritative region", "error", err)
			goto ERR_WAIT
		}

		// Perform a two-way diff
		delete, update := diffSentinelPolicies(s.State(), req.MinQueryIndex, resp.Policies)

		// Delete policies that should not exist
		if len(delete) > 0 {
			args := &structs.SentinelPolicyDeleteRequest{
				Names: delete,
			}
			_, _, err := s.raftApply(structs.SentinelPolicyDeleteRequestType, args)
			if err != nil {
				s.logger.Error("failed to delete policies", "error", err)
				goto ERR_WAIT
			}
		}

		// Fetch any outdated policies
		var fetched []*structs.SentinelPolicy
		if len(update) > 0 {
			req := structs.SentinelPolicySetRequest{
				Names: update,
				QueryOptions: structs.QueryOptions{
					Region:        s.config.AuthoritativeRegion,
					AuthToken:     s.ReplicationToken(),
					AllowStale:    true,
					MinQueryIndex: resp.Index - 1,
				},
			}
			var reply structs.SentinelPolicySetResponse
			if err := s.forwardRegion(s.config.AuthoritativeRegion,
				"Sentinel.GetPolicies", &req, &reply); err != nil {
				s.logger.Error("failed to fetch policies from authoritative region", "error", err)
				goto ERR_WAIT
			}
			for _, policy := range reply.Policies {
				fetched = append(fetched, policy)
			}
		}

		// Update local policies
		if len(fetched) > 0 {
			args := &structs.SentinelPolicyUpsertRequest{
				Policies: fetched,
			}
			_, _, err := s.raftApply(structs.SentinelPolicyUpsertRequestType, args)
			if err != nil {
				s.logger.Error("failed to update policies", "error", err)
				goto ERR_WAIT
			}
		}

		// Update the minimum query index, blocks until there
		// is a change.
		req.MinQueryIndex = resp.Index
	}

ERR_WAIT:
	select {
	case <-time.After(s.config.ReplicationBackoff):
		goto START
	case <-stopCh:
		return
	}
}

// diffSentinelPolicies is used to perform a two-way diff between the local
// policies and the remote policies to determine which policies need to
// be deleted or updated.
func diffSentinelPolicies(state *state.StateStore, minIndex uint64, remoteList []*structs.SentinelPolicyListStub) (delete []string, update []string) {
	// Construct a set of the local and remote policies
	local := make(map[string][]byte)
	remote := make(map[string]struct{})

	// Add all the local policies
	iter, err := state.SentinelPolicies(nil)
	if err != nil {
		panic("failed to iterate local policies")
	}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		policy := raw.(*structs.SentinelPolicy)
		local[policy.Name] = policy.Hash
	}

	// Iterate over the remote policies
	for _, rp := range remoteList {
		remote[rp.Name] = struct{}{}

		// Check if the policy is missing locally
		if localHash, ok := local[rp.Name]; !ok {
			update = append(update, rp.Name)

			// Check if policy is newer remotely and there is a hash mis-match.
		} else if rp.ModifyIndex > minIndex && !bytes.Equal(localHash, rp.Hash) {
			update = append(update, rp.Name)
		}
	}

	// Check if policy should be deleted
	for lp := range local {
		if _, ok := remote[lp]; !ok {
			delete = append(delete, lp)
		}
	}
	return
}

// replicateQuotaSpecs is used to replicate quota specifications from
// the authoritative region to this region.
func (s *Server) replicateQuotaSpecs(stopCh chan struct{}) {
	req := structs.QuotaSpecListRequest{
		QueryOptions: structs.QueryOptions{
			Region:     s.config.AuthoritativeRegion,
			AllowStale: true,
		},
	}
	limiter := rate.NewLimiter(replicationRateLimit, int(replicationRateLimit))
	s.logger.Debug("starting quota specification replication from authoritative region", "region", req.Region)

START:
	for {
		select {
		case <-stopCh:
			return
		default:
		}

		// Rate limit how often we attempt replication
		limiter.Wait(context.Background())

		// Fetch the list of quotas
		var resp structs.QuotaSpecListResponse
		req.AuthToken = s.ReplicationToken()
		err := s.forwardRegion(s.config.AuthoritativeRegion,
			"Quota.ListQuotaSpecs", &req, &resp)
		if err != nil {
			s.logger.Error("failed to fetch quota specifications from authoritative region", "error", err)
			goto ERR_WAIT
		}

		// Perform a two-way diff
		delete, update := diffQuotaSpecs(s.State(), req.MinQueryIndex, resp.Quotas)

		// Delete policies that should not exist
		if len(delete) > 0 {
			args := &structs.QuotaSpecDeleteRequest{
				Names: delete,
			}
			_, _, err := s.raftApply(structs.QuotaSpecDeleteRequestType, args)
			if err != nil {
				s.logger.Error("failed to delete quota specs", "error", err)
				goto ERR_WAIT
			}
		}

		// Fetch any outdated quota specs
		var fetched []*structs.QuotaSpec
		if len(update) > 0 {
			req := structs.QuotaSpecSetRequest{
				Names: update,
				QueryOptions: structs.QueryOptions{
					Region:        s.config.AuthoritativeRegion,
					AuthToken:     s.ReplicationToken(),
					AllowStale:    true,
					MinQueryIndex: resp.Index - 1,
				},
			}
			var reply structs.QuotaSpecSetResponse
			if err := s.forwardRegion(s.config.AuthoritativeRegion,
				"Quota.GetQuotaSpecs", &req, &reply); err != nil {
				s.logger.Error("failed to fetch quota specifications from authoritative region", "error", err)
				goto ERR_WAIT
			}
			for _, quota := range reply.Quotas {
				fetched = append(fetched, quota)
			}
		}

		// Update local policies
		if len(fetched) > 0 {
			args := &structs.QuotaSpecUpsertRequest{
				Quotas: fetched,
			}
			_, _, err := s.raftApply(structs.QuotaSpecUpsertRequestType, args)
			if err != nil {
				s.logger.Error("failed to update quota specs", "error", err)
				goto ERR_WAIT
			}
		}

		// Update the minimum query index, blocks until there
		// is a change.
		req.MinQueryIndex = resp.Index
	}

ERR_WAIT:
	select {
	case <-time.After(s.config.ReplicationBackoff):
		goto START
	case <-stopCh:
		return
	}
}

// diffQuotaSpecs is used to perform a two-way diff between the local quota
// specifications and the remote quota specifications to determine which
// specs need to be deleted or updated.
func diffQuotaSpecs(state *state.StateStore, minIndex uint64, remoteList []*structs.QuotaSpec) (delete []string, update []string) {
	// Construct a set of the local and remote policies
	local := make(map[string][]byte)
	remote := make(map[string]struct{})

	// Add all the local policies
	iter, err := state.QuotaSpecs(nil)
	if err != nil {
		panic("failed to iterate local quota specifications")
	}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		spec := raw.(*structs.QuotaSpec)
		local[spec.Name] = spec.Hash
	}

	// Iterate over the remote policies
	for _, rp := range remoteList {
		remote[rp.Name] = struct{}{}

		// Check if the policy is missing locally
		if localHash, ok := local[rp.Name]; !ok {
			update = append(update, rp.Name)

			// Check if policy is newer remotely and there is a hash mis-match.
		} else if rp.ModifyIndex > minIndex && !bytes.Equal(localHash, rp.Hash) {
			update = append(update, rp.Name)
		}
	}

	// Check if policy should be deleted
	for lp := range local {
		if _, ok := remote[lp]; !ok {
			delete = append(delete, lp)
		}
	}
	return
}
