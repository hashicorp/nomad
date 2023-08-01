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

	// Compile and cache the ACL object. For many claims this will result in an
	// ACL object with no policies, which can be efficiently cached.
	aclObj, err := structs.CompileACLObject(s.aclCache, policies)
	if err != nil {
		return nil, err
	}
	return aclObj, nil
}

// resolveTokenFromSnapshotCache is used to resolve an ACL object from a
// snapshot of state, using a cache to avoid parsing and ACL construction when
// possible. It is split from resolveToken to simplify testing.
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
		if token.IsExpired(time.Now().UTC()) {
			return nil, structs.ErrTokenExpired
		}
	}

	// Check if this is a management token
	if token.Type == structs.ACLManagementToken {
		return acl.ManagementACL, nil
	}

	// Store all policies detailed in the token request, this includes the
	// named policies and those referenced within the role link.
	policies := make([]*structs.ACLPolicy, 0, len(token.Policies)+len(token.Roles))

	// Iterate all the token policies and add these to our policy tracking
	// array.
	for _, policyName := range token.Policies {
		policy, err := snap.ACLPolicyByName(nil, policyName)
		if err != nil {
			return nil, err
		}
		if policy == nil {
			// Ignore policies that don't exist, since they don't grant any
			// more privilege.
			continue
		}

		// Add the policy to the tracking array.
		policies = append(policies, policy)
	}

	// Iterate all the token role links, so we can unpack these and identify
	// the ACL policies.
	for _, roleLink := range token.Roles {

		// Any error reading the role means we cannot move forward. We just
		// ignore any roles that have been detailed but are not within our
		// state.
		role, err := snap.GetACLRoleByID(nil, roleLink.ID)
		if err != nil {
			return nil, err
		}
		if role == nil {
			continue
		}

		// Unpack the policies held within the ACL role to form a single list
		// of ACL policies that this token has available.
		for _, policyLink := range role.Policies {
			policy, err := snap.ACLPolicyByName(nil, policyLink.Name)
			if err != nil {
				return nil, err
			}

			// Ignore policies that don't exist, since they don't grant any
			// more privilege.
			if policy == nil {
				continue
			}

			// Add the policy to the tracking array.
			policies = append(policies, policy)
		}
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
		if token.IsExpired(time.Now().UTC()) {
			return nil, structs.ErrTokenExpired
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

	// Find any policies attached to the job
	jobId := alloc.Job.ID
	if alloc.Job.ParentID != "" {
		jobId = alloc.Job.ParentID
	}
	iter, err := snap.ACLPolicyByJob(nil, alloc.Namespace, jobId)
	if err != nil {
		return nil, err
	}
	policies := []*structs.ACLPolicy{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		policy := raw.(*structs.ACLPolicy)
		if policy.JobACL == nil {
			continue
		}

		switch {
		case policy.JobACL.Group == "":
			policies = append(policies, policy)
		case policy.JobACL.Group != alloc.TaskGroup:
			continue // don't bother checking task
		case policy.JobACL.Task == "":
			policies = append(policies, policy)
		case policy.JobACL.Task == claims.TaskName:
			policies = append(policies, policy)
		}
	}

	return policies, nil
}
