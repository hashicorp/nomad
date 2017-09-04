package nomad

import (
	"time"

	metrics "github.com/armon/go-metrics"
	lru "github.com/hashicorp/golang-lru"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// resolveToken is used to translate an ACL Token Secret ID into
// an ACL object, nil if ACLs are disabled, or an error.
func (s *Server) resolveToken(secretID string) (*acl.ACL, error) {
	// Fast-path if ACLs are disabled
	if !s.config.ACLEnabled {
		return nil, nil
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "resolveToken"}, time.Now())

	// Snapshot the state
	snap, err := s.fsm.State().Snapshot()
	if err != nil {
		return nil, err
	}

	// Resolve the ACL
	return resolveTokenFromSnapshotCache(snap, s.aclCache, secretID)
}

// resolveTokenFromSnapshotCache is used to resolve an ACL object from a snapshot of state,
// using a cache to avoid parsing and ACL construction when possible. It is split from resolveToken
// to simplify testing.
func resolveTokenFromSnapshotCache(snap *state.StateSnapshot, cache *lru.TwoQueueCache, secretID string) (*acl.ACL, error) {
	// Lookup the ACL Token
	var token *structs.ACLToken
	var err error

	// Handle anonymous requests
	if secretID == "" {
		token = structs.AnonymousACLToken
	} else {
		token, err = snap.ACLTokenBySecretID(nil, secretID)
		if err != nil {
			return nil, err
		}
		if token == nil {
			return nil, structs.ErrTokenNotFound
		}
	}

	// Check if this is a management token
	if token.Type == structs.ACLManagementToken {
		return acl.ManagementACL, nil
	}

	// Get all associated policies
	policies := make([]*structs.ACLPolicy, 0, len(token.Policies))
	for _, policyName := range token.Policies {
		policy, err := snap.ACLPolicyByName(nil, policyName)
		if err != nil {
			return nil, err
		}
		if policy == nil {
			// Ignore policies that don't exist, since they don't grant any more privilege
			continue
		}

		// Save the policy and update the cache key
		policies = append(policies, policy)
	}

	// Compile and cache the ACL object
	aclObj, err := structs.CompileACLObject(cache, policies)
	if err != nil {
		return nil, err
	}
	return aclObj, nil
}
