package nomad

import (
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/hcl"
	"github.com/pkg/errors"
)

const (
	// consulGlobalManagementPolicyID is the built-in policy ID used by Consul
	// to denote global-management tokens.
	//
	// https://www.consul.io/docs/security/acl/acl-system#builtin-policies
	consulGlobalManagementPolicyID = "00000000-0000-0000-0000-000000000001"
)

// ConsulServiceRule represents a policy for a service.
type ConsulServiceRule struct {
	Name   string `hcl:",key"`
	Policy string
}

// ConsulKeyRule represents a policy for the keystore.
type ConsulKeyRule struct {
	Name   string `hcl:",key"`
	Policy string
}

// ConsulPolicy represents the parts of a ConsulServiceRule Policy that are
// relevant to Service Identity authorizations.
type ConsulPolicy struct {
	Services          []*ConsulServiceRule     `hcl:"service,expand"`
	ServicePrefixes   []*ConsulServiceRule     `hcl:"service_prefix,expand"`
	KeyPrefixes       []*ConsulKeyRule         `hcl:"key_prefix,expand"`
	Namespaces        map[string]*ConsulPolicy `hcl:"namespace,expand"`
	NamespacePrefixes map[string]*ConsulPolicy `hcl:"namespace_prefix,expand"`
}

// parseConsulPolicy parses raw string s into a ConsulPolicy. An error is
// returned if decoding the policy fails, or if the decoded policy has no
// Services or ServicePrefixes defined.
func parseConsulPolicy(s string) (*ConsulPolicy, error) {
	cp := new(ConsulPolicy)
	if err := hcl.Decode(cp, s); err != nil {
		return nil, errors.Wrap(err, "failed to parse ACL policy")
	}
	return cp, nil
}

// isManagementToken returns true if the Consul token is backed by the
// built-in global-management policy. Such a token has complete, unrestricted
// access to all of Consul.
//
// https://www.consul.io/docs/security/acl/acl-system#builtin-policies
func (c *consulACLsAPI) isManagementToken(token *api.ACLToken) bool {
	if token == nil {
		return false
	}

	for _, policy := range token.Policies {
		if policy.ID == consulGlobalManagementPolicyID {
			return true
		}
	}
	return false
}

// namespaceCheck is used to verify the namespace of the object matches the
// namespace of the ACL token provided.
//
// exception: iff token is in the default namespace, it may contain policies
// that extend into other namespaces using namespace_prefix, which must bypass
// this early check and validate in the service/keystore helpers
func namespaceCheck(namespace string, token *api.ACLToken) error {
	if token.Namespace != "default" && token.Namespace != namespace {
		return errors.Errorf("consul ACL token cannot use namespace %q", namespace)
	}
	return nil
}

func (c *consulACLsAPI) canReadKeystore(namespace string, token *api.ACLToken) (bool, error) {
	// early check the token is compatible with desired namespace
	if err := namespaceCheck(namespace, token); err != nil {
		return false, nil
	}

	// determines whether a top-level ACL policy will be applicable
	matches := namespace == token.Namespace

	// check each policy directly attached to the token
	for _, policyRef := range token.Policies {
		if allowable, err := c.policyAllowsKeystoreRead(matches, namespace, policyRef.ID); err != nil {
			return false, err
		} else if allowable {
			return true, nil
		}
	}

	// check each policy on each role attached to the token
	for _, roleLink := range token.Roles {
		role, _, err := c.aclClient.RoleRead(roleLink.ID, &api.QueryOptions{
			AllowStale: false,
		})
		if err != nil {
			return false, err
		}

		for _, policyLink := range role.Policies {
			allowable, err := c.policyAllowsKeystoreRead(matches, namespace, policyLink.ID)
			if err != nil {
				return false, err
			} else if allowable {
				return true, nil
			}
		}
	}

	return false, nil
}

func (c *consulACLsAPI) canWriteService(namespace, service string, token *api.ACLToken) (bool, error) {
	// early check the token is compatible with desired namespace
	if err := namespaceCheck(namespace, token); err != nil {
		return false, nil
	}

	// determines whether a top-level ACL policy will be applicable
	matches := namespace == token.Namespace

	// check each policy directly attached to the token
	for _, policyRef := range token.Policies {
		if allowable, err := c.policyAllowsServiceWrite(matches, namespace, service, policyRef.ID); err != nil {
			return false, err
		} else if allowable {
			return true, nil
		}
	}

	// check each policy on each role attached to the token
	for _, roleLink := range token.Roles {
		role, _, err := c.aclClient.RoleRead(roleLink.ID, &api.QueryOptions{
			AllowStale: false,
		})
		if err != nil {
			return false, err
		}

		for _, policyLink := range role.Policies {
			allowable, wErr := c.policyAllowsServiceWrite(matches, namespace, service, policyLink.ID)
			if wErr != nil {
				return false, wErr
			} else if allowable {
				return true, nil
			}
		}
	}

	return false, nil
}

func (c *consulACLsAPI) policyAllowsServiceWrite(matches bool, namespace, service string, policyID string) (bool, error) {
	policy, _, err := c.aclClient.PolicyRead(policyID, &api.QueryOptions{
		AllowStale: false,
	})
	if err != nil {
		return false, err
	}

	// compare policy to the necessary permission for service write
	// e.g. service "db" { policy = "write" }
	// e.g. service_prefix "" { policy == "write" }
	cp, err := parseConsulPolicy(policy.Rules)
	if err != nil {
		return false, err
	}

	if cp.allowsServiceWrite(matches, namespace, service) {
		return true, nil
	}
	return false, nil
}

const (
	serviceNameWildcard = "*"
)

func (cp *ConsulPolicy) allowsServiceWrite(matches bool, namespace, task string) bool {
	canWriteService := func(services []*ConsulServiceRule) bool {
		for _, service := range services {
			name := strings.ToLower(service.Name)
			policy := strings.ToLower(service.Policy)
			if policy == ConsulPolicyWrite {
				if name == task || name == serviceNameWildcard {
					return true
				}
			}
		}
		return false
	}

	canWriteServicePrefix := func(services []*ConsulServiceRule) bool {
		for _, servicePrefix := range services {
			prefix := strings.ToLower(servicePrefix.Name)
			policy := strings.ToLower(servicePrefix.Policy)
			if policy == ConsulPolicyWrite {
				if strings.HasPrefix(task, prefix) {
					return true
				}
			}
		}
		return false
	}

	if matches {
		// check the top-level service/service_prefix rules
		if canWriteService(cp.Services) || canWriteServicePrefix(cp.ServicePrefixes) {
			return true
		}
	}

	// for each namespace rule, if that namespace and the desired namespace
	// are a match, we can then check the service/service_prefix policy rules
	for ns, policy := range cp.Namespaces {
		if ns == namespace {
			if canWriteService(policy.Services) || canWriteServicePrefix(policy.ServicePrefixes) {
				return true
			}
		}
	}

	// for each namespace_prefix rule, see if that namespace_prefix applies
	// to this namespace, and if yes, also check those service/service_prefix
	// policy rules
	for prefix, policy := range cp.NamespacePrefixes {
		if strings.HasPrefix(namespace, prefix) {
			if canWriteService(policy.Services) || canWriteServicePrefix(policy.ServicePrefixes) {
				return true
			}
		}
	}

	return false
}

func (c *consulACLsAPI) policyAllowsKeystoreRead(matches bool, namespace, policyID string) (bool, error) {
	policy, _, err := c.aclClient.PolicyRead(policyID, &api.QueryOptions{
		AllowStale: false,
	})
	if err != nil {
		return false, err
	}

	cp, err := parseConsulPolicy(policy.Rules)
	if err != nil {
		return false, err
	}

	if cp.allowsKeystoreRead(matches, namespace) {
		return true, nil
	}

	return false, nil
}

func (cp *ConsulPolicy) allowsKeystoreRead(matches bool, namespace string) bool {
	canReadKeystore := func(prefixes []*ConsulKeyRule) bool {
		for _, keyPrefix := range prefixes {
			name := strings.ToLower(keyPrefix.Name)
			policy := strings.ToLower(keyPrefix.Policy)
			if name == "" {
				if policy == ConsulPolicyWrite || policy == ConsulPolicyRead {
					return true
				}
			}
		}
		return false
	}

	// check the top-level key_prefix rules, but only if the desired namespace
	// matches the namespace of the consul acl token
	if matches && canReadKeystore(cp.KeyPrefixes) {
		return true
	}

	// for each namespace rule, if that namespace matches the desired namespace
	// we chan then check the keystore policy
	for ns, policy := range cp.Namespaces {
		if ns == namespace {
			if canReadKeystore(policy.KeyPrefixes) {
				return true
			}
		}
	}

	// for each namespace_prefix rule, see if that namespace_prefix applies to
	// this namespace, and if yes, also check those key_prefix policy rules
	for prefix, policy := range cp.NamespacePrefixes {
		if strings.HasPrefix(namespace, prefix) {
			if canReadKeystore(policy.KeyPrefixes) {
				return true
			}
		}
	}

	return false
}
