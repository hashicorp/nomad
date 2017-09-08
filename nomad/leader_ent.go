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
	if err := s.establishProLeadership(); err != nil {
		return err
	}

	// Start replication of Sentinel Policies if ACLs are enabled,
	// and we are not the authoritative region.
	if s.config.ACLEnabled && s.config.Region != s.config.AuthoritativeRegion {
		go s.replicateSentinelPolicies(stopCh)
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
	s.logger.Printf("[DEBUG] nomad: starting Sentinel policy replication from authoritative region %q", req.Region)

START:
	for {
		select {
		case <-stopCh:
			return
		default:
			// Rate limit how often we attempt replication
			limiter.Wait(context.Background())

			// Fetch the list of policies
			var resp structs.SentinelPolicyListResponse
			req.SecretID = s.ReplicationToken()
			err := s.forwardRegion(s.config.AuthoritativeRegion,
				"Sentinel.ListPolicies", &req, &resp)
			if err != nil {
				s.logger.Printf("[ERR] nomad: failed to fetch policies from authoritative region: %v", err)
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
					s.logger.Printf("[ERR] nomad: failed to delete policies: %v", err)
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
						SecretID:      s.ReplicationToken(),
						AllowStale:    true,
						MinQueryIndex: resp.Index - 1,
					},
				}
				var reply structs.SentinelPolicySetResponse
				if err := s.forwardRegion(s.config.AuthoritativeRegion,
					"Sentinel.GetPolicies", &req, &reply); err != nil {
					s.logger.Printf("[ERR] nomad: failed to fetch policies from authoritative region: %v", err)
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
					s.logger.Printf("[ERR] nomad: failed to update policies: %v", err)
					goto ERR_WAIT
				}
			}

			// Update the minimum query index, blocks until there
			// is a change.
			req.MinQueryIndex = resp.Index
		}
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
