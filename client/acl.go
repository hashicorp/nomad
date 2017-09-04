package client

import (
	"time"

	metrics "github.com/armon/go-metrics"
	lru "github.com/hashicorp/golang-lru"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// policyCacheSize is the number of ACL policies to keep cached. Policies have a fetching cost
	// so we keep the hot policies cached to reduce the ACL token resolution time.
	policyCacheSize = 64

	// aclCacheSize is the number of ACL objects to keep cached. ACLs have a parsing and
	// construction cost, so we keep the hot objects cached to reduce the ACL token resolution time.
	aclCacheSize = 64

	// tokenCacheSize is the number of ACL tokens to keep cached. Tokens have a fetching cost,
	// so we keep the hot tokens cached to reduce the lookups.
	tokenCacheSize = 64
)

// clientACLResolver holds the state required for client resolution
// of ACLs
type clientACLResolver struct {
	// aclCache is used to maintain the parsed ACL objects
	aclCache *lru.TwoQueueCache

	// policyCache is used to maintain the fetched policy objects
	policyCache *lru.TwoQueueCache

	// tokenCache is used to maintain the fetched token objects
	tokenCache *lru.TwoQueueCache
}

// init is used to setup the client resolver state
func (c *clientACLResolver) init() error {
	// Create the ACL object cache
	var err error
	c.aclCache, err = lru.New2Q(aclCacheSize)
	if err != nil {
		return err
	}
	c.policyCache, err = lru.New2Q(policyCacheSize)
	if err != nil {
		return err
	}
	c.tokenCache, err = lru.New2Q(tokenCacheSize)
	if err != nil {
		return err
	}
	return nil
}

// cachedACLValue is used to manage ACL Token or Policy TTLs
type cachedACLValue struct {
	Token     *structs.ACLToken
	Policy    *structs.ACLPolicy
	CacheTime time.Time
}

// Age is the time since the token was cached
func (c *cachedACLValue) Age() time.Duration {
	return time.Since(c.CacheTime)
}

// ResolveToken is used to translate an ACL Token Secret ID into
// an ACL object, nil if ACLs are disabled, or an error.
func (c *Client) ResolveToken(secretID string) (*acl.ACL, error) {
	// Fast-path if ACLs are disabled
	if !c.config.ACLEnabled {
		return nil, nil
	}
	defer metrics.MeasureSince([]string{"client", "acl", "resolve_token"}, time.Now())

	// Resolve the token value
	token, err := c.resolveTokenValue(secretID)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, structs.ErrTokenNotFound
	}

	// Check if this is a management token
	if token.Type == structs.ACLManagementToken {
		return acl.ManagementACL, nil
	}

	// Resolve the policies
	policies, err := c.resolvePolicies(token.SecretID, token.Policies)
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
		SecretID: secretID,
		QueryOptions: structs.QueryOptions{
			Region:     c.Region(),
			AllowStale: true,
		},
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
func (c *Client) resolvePolicies(secretID string, policies []string) ([]*structs.ACLPolicy, error) {
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
		Names: fetch,
		QueryOptions: structs.QueryOptions{
			Region:     c.Region(),
			SecretID:   secretID,
			AllowStale: true,
		},
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
