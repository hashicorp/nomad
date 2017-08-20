package client

import (
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/structs"
)

// cachedACLValue is used to manage ACL Token or Policy TTLs
type cachedACLValue struct {
	Token     *structs.ACLToken
	Policy    *structs.ACLPolicy
	CacheTime time.Time
}

// Age is the time since the token was cached
func (c *cachedACLValue) Age() time.Duration {
	return time.Now().Sub(c.CacheTime)
}

// resolveToken is used to translate an ACL Token Secret ID into
// an ACL object, nil if ACLs are disabled, or an error.
func (c *Client) resolveToken(secretID string) (*acl.ACL, error) {
	// Fast-path if ACLs are disabled
	if !c.config.ACLEnabled {
		return nil, nil
	}
	defer metrics.MeasureSince([]string{"client", "acl", "resolveToken"}, time.Now())

	// Resolve the token value
	token, err := c.resolveTokenValue(secretID)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, structs.TokenNotFound
	}

	// Check if this is a management token
	if token.Type == structs.ACLManagementToken {
		return acl.ManagementACL, nil
	}

	// Resolve the policies
	policies, err := c.resolvePolicies(token.Policies)
	if err != nil {
		return nil, err
	}

	// Resolve the ACL object
	aclObj, err := structs.CompileACLObject(c.aclCache, policies)
	if err != nil {
		return nil, err
	}
	return aclObj, nil
}

// resolveTokenValue is used to translate a secret ID into an ACL token with caching
// We use a local cache up to the TTL limit, and then resolve via a server. If we cannot
// reach a server, but have a cached value we extend the TTL to gracefully handle outages.
func (c *Client) resolveTokenValue(secretID string) (*structs.ACLToken, error) {
	// Hot-path the anonymous token
	if secretID == "" {
		return structs.AnonymousACLToken, nil
	}

	// Lookup the token in the cache
	raw, ok := c.tokenCache.Get(secretID)
	if ok {
		cached := raw.(*cachedACLValue)
		if cached.Age() <= c.config.ACLTokenTTL {
			return cached.Token, nil
		}
	}

	// Lookup the token
	req := structs.ResolveACLTokenRequest{
		SecretID:     secretID,
		QueryOptions: structs.QueryOptions{Region: c.Region()},
	}
	var resp structs.ResolveACLTokenResponse
	if err := c.RPC("ACL.ResolveToken", &req, &resp); err != nil {
		// If we encounter an error but have a cached value, mask the error and extend the cache
		if ok {
			c.logger.Printf("[WARN] client: failed to resolve token, using expired cached value: %v", err)
			cached := raw.(*cachedACLValue)
			return cached.Token, nil
		}
		return nil, err
	}

	// Cache the response (positive or negative)
	c.tokenCache.Add(secretID, &cachedACLValue{
		Token:     resp.Token,
		CacheTime: time.Now(),
	})
	return resp.Token, nil
}

// resolvePolicies is used to translate a set of named ACL policies into the objects.
// We cache the policies locally, and fault them from a server as necessary. Policies
// are cached for a TTL, and then refreshed. If a server cannot be reached, the cache TTL
// will be ignored to gracefully handle outages.
func (c *Client) resolvePolicies(policies []string) ([]*structs.ACLPolicy, error) {
	var out []*structs.ACLPolicy
	var expired []*structs.ACLPolicy
	var missing []string

	// Scan the cache for each policy
	for _, policyName := range policies {
		// Lookup the policy in the cache
		raw, ok := c.policyCache.Get(policyName)
		if !ok {
			missing = append(missing, policyName)
			continue
		}

		// Check if the cached value is valid or expired
		cached := raw.(*cachedACLValue)
		if cached.Age() <= c.config.ACLPolicyTTL {
			out = append(out, cached.Policy)
		} else {
			expired = append(expired, cached.Policy)
		}
	}

	// Hot-path if we have no missing or expired policies
	if len(missing)+len(expired) == 0 {
		return out, nil
	}

	// Lookup the missing and expired policies
	fetch := missing
	for _, p := range expired {
		fetch = append(fetch, p.Name)
	}
	req := structs.ACLPolicySetRequest{
		Names:        fetch,
		QueryOptions: structs.QueryOptions{Region: c.Region()},
	}
	var resp structs.ACLPolicySetResponse
	if err := c.RPC("ACL.GetPolicies", &req, &resp); err != nil {
		// If we encounter an error but have cached policies, mask the error and extend the cache
		if len(missing) == 0 {
			c.logger.Printf("[WARN] client: failed to resolve policies, using expired cached value: %v", err)
			out = append(out, expired...)
			return out, nil
		}
		return nil, err
	}

	// Handle each output
	for _, policy := range resp.Policies {
		c.policyCache.Add(policy.Name, &cachedACLValue{
			Policy:    policy,
			CacheTime: time.Now(),
		})
		out = append(out, policy)
	}

	// Return the valid policies
	return out, nil
}
