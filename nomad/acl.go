package nomad

import (
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	lru "github.com/hashicorp/golang-lru"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	// tokenNotFound indicates the Token was not found
	tokenNotFound = errors.New("ACL token not found")

	// managementACL is used for all management tokens
	managementACL *acl.ACL
)

func init() {
	// managementACL has management flag enabled
	var err error
	managementACL, err = acl.NewACL(true, nil)
	if err != nil {
		panic(fmt.Errorf("failed to setup management ACL: %v", err))
	}
}

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
	token, err := snap.ACLTokenBySecretID(nil, secretID)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, tokenNotFound
	}

	// Check if this is a management token
	if token.Type == structs.ACLManagementToken {
		return managementACL, nil
	}

	// Get all associated policies
	policies := make([]*structs.ACLPolicy, 0, len(token.Policies))
	cacheKeyHash := sha1.New()
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

		// Update the cache key using (policy name, modify index).
		// This ensures any updates to the policy cause the cache to be busted.
		cacheKeyHash.Write([]byte(policyName))
		binary.Write(cacheKeyHash, binary.BigEndian, policy.ModifyIndex)
	}

	// Finalize the cache key and check for a match
	cacheKey := string(cacheKeyHash.Sum(nil))
	aclRaw, ok := cache.Get(cacheKey)
	if ok {
		return aclRaw.(*acl.ACL), nil
	}

	// Parse the policies
	parsed := make([]*acl.Policy, 0, len(policies))
	for _, policy := range policies {
		p, err := acl.Parse(policy.Rules)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q: %v", policy.Name, err)
		}
		parsed = append(parsed, p)
	}

	// Create the ACL object
	aclObj, err := acl.NewACL(false, parsed)
	if err != nil {
		return nil, fmt.Errorf("failed to construct ACL: %v", err)
	}

	// Update the cache
	cache.Add(cacheKey, aclObj)

	// Return the ACL object
	return aclObj, nil
}
