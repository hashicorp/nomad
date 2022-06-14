package nomad

import (
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	lru "github.com/hashicorp/golang-lru"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ResolveToken is used to translate an ACL Token Secret ID into
// an ACL object, nil if ACLs are disabled, or an error.
func (s *Server) ResolveToken(secretID string) (*acl.ACL, error) {
	// Fast-path if ACLs are disabled
	if !s.config.ACLEnabled {
		return nil, nil
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "resolveToken"}, time.Now())

	// Check if the secret ID is the leader secret ID, in which case treat it as
	// a management token.
	if leaderAcl := s.getLeaderAcl(); leaderAcl != "" && secretID == leaderAcl {
		return acl.ManagementACL, nil
	}

	// Snapshot the state
	snap, err := s.fsm.State().Snapshot()
	if err != nil {
		return nil, err
	}

	// Resolve the ACL
	return resolveTokenFromSnapshotCache(snap, s.aclCache, secretID)
}

// VerifyClaim asserts that the token is valid and that the resulting
// allocation ID belongs to a non-terminal allocation
func (s *Server) VerifyClaim(token string) (*structs.IdentityClaims, error) {

	claims, err := s.encrypter.VerifyClaim(token)
	if err != nil {
		return nil, err
	}
	snap, err := s.fsm.State().Snapshot()
	if err != nil {
		return nil, err
	}
	alloc, err := snap.AllocByID(nil, claims.AllocationID)
	if err != nil {
		return nil, err
	}
	if alloc == nil || alloc.Job == nil {
		return nil, fmt.Errorf("allocation does not exist")
	}

	// the claims for terminal allocs are always treated as expired
	if alloc.TerminalStatus() {
		return nil, fmt.Errorf("allocation is terminal")
	}

	return claims, nil
}

func (s *Server) ResolveClaims(claims *structs.IdentityClaims) (*acl.ACL, error) {

	policies, err := s.resolvePoliciesForClaims(claims)
	if err != nil {
		return nil, err
	}
	if len(policies) == 0 {
		return nil, nil
	}

	// Compile and cache the ACL object
	aclObj, err := structs.CompileACLObject(s.aclCache, policies)
	if err != nil {
		return nil, err
	}
	return aclObj, nil
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

// ResolveSecretToken is used to translate an ACL Token Secret ID into
// an ACLToken object, nil if ACLs are disabled, or an error.
func (s *Server) ResolveSecretToken(secretID string) (*structs.ACLToken, error) {
	// TODO(Drew) Look into using ACLObject cache or create a separate cache

	// Fast-path if ACLs are disabled
	if !s.config.ACLEnabled {
		return nil, nil
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "resolveSecretToken"}, time.Now())

	snap, err := s.fsm.State().Snapshot()
	if err != nil {
		return nil, err
	}

	// Lookup the ACL Token
	var token *structs.ACLToken
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

	return token, nil
}

func (s *Server) resolvePoliciesForClaims(claims *structs.IdentityClaims) ([]*structs.ACLPolicy, error) {

	snap, err := s.fsm.State().Snapshot()
	if err != nil {
		return nil, err
	}
	alloc, err := snap.AllocByID(nil, claims.AllocationID)
	if err != nil {
		return nil, err
	}
	if alloc == nil || alloc.Job == nil {
		return nil, fmt.Errorf("allocation does not exist")
	}

	// Find any implicit policies associated with this task
	policies := []*structs.ACLPolicy{}
	implicitPolicyNames := []string{
		fmt.Sprintf("_:%s/%s/%s/%s", alloc.Namespace, alloc.Job.ID, alloc.TaskGroup, claims.TaskName),
		fmt.Sprintf("_:%s/%s/%s", alloc.Namespace, alloc.Job.ID, alloc.TaskGroup),
		fmt.Sprintf("_:%s/%s", alloc.Namespace, alloc.Job.ID),
		fmt.Sprintf("_:%s", alloc.Namespace),
	}

	for _, policyName := range implicitPolicyNames {
		policy, err := snap.ACLPolicyByName(nil, policyName)
		if err != nil {
			return nil, err
		}
		if policy == nil {
			// Ignore policies that don't exist, since they don't
			// grant any more privilege
			continue
		}

		policies = append(policies, policy)
	}
	return policies, nil
}
