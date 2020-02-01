package nomad

import (
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/hcl"
	"github.com/pkg/errors"
)

// ConsulServiceRule represents a policy for a service.
type ConsulServiceRule struct {
	Name   string `hcl:",key"`
	Policy string
}

// ConsulPolicy represents the parts of a ConsulServiceRule Policy that are
// relevant to Service Identity authorizations.
type ConsulPolicy struct {
	Services        []*ConsulServiceRule `hcl:"service,expand"`
	ServicePrefixes []*ConsulServiceRule `hcl:"service_prefix,expand"`
}

// IsEmpty returns true if there are no Services or ServicePrefixes defined for
// the ConsulPolicy.
func (cp *ConsulPolicy) IsEmpty() bool {
	if cp == nil {
		return true
	}
	return len(cp.Services) == 0 && len(cp.ServicePrefixes) == 0
}

// ParseConsulPolicy parses raw string s into a ConsulPolicy. An error is
// returned if decoding the policy fails, or if the decoded policy has no
// Services or ServicePrefixes defined.
func ParseConsulPolicy(s string) (*ConsulPolicy, error) {
	cp := new(ConsulPolicy)
	if err := hcl.Decode(cp, s); err != nil {
		return nil, errors.Wrap(err, "failed to parse ACL policy")
	}
	if cp.IsEmpty() {
		// the only use case for now, may as well validate asap
		return nil, errors.New("consul policy contains no service rules")
	}
	return cp, nil
}

func (c *consulACLsAPI) hasSufficientPolicy(task string, token *api.ACLToken) (bool, error) {
	// check each policy directly attached to the token
	for _, policyRef := range token.Policies {
		if allowable, err := c.policyAllowsServiceWrite(task, policyRef.ID); err != nil {
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
			allowable, err := c.policyAllowsServiceWrite(task, policyLink.ID)
			if err != nil {
				return false, err
			}
			if allowable {
				return true, nil
			}
		}
	}

	return false, nil
}

func (c *consulACLsAPI) policyAllowsServiceWrite(task string, policyID string) (bool, error) {
	policy, _, err := c.aclClient.PolicyRead(policyID, &api.QueryOptions{
		AllowStale: false,
	})
	if err != nil {
		return false, err
	}

	// compare policy to the necessary permission for service write
	// e.g. service "db" { policy = "write" }
	// e.g. service_prefix "" { policy == "write" }
	cp, err := ParseConsulPolicy(policy.Rules)
	if err != nil {
		return false, err
	}

	if cp.allowsServiceWrite(task) {
		return true, nil
	}

	return false, nil
}

const (
	serviceNameWildcard = "*"
)

func (cp *ConsulPolicy) allowsServiceWrite(task string) bool {
	for _, service := range cp.Services {
		name := strings.ToLower(service.Name)
		policy := strings.ToLower(service.Policy)
		if policy == ConsulPolicyWrite {
			if name == task || name == serviceNameWildcard {
				return true
			}
		}
	}

	for _, servicePrefix := range cp.ServicePrefixes {
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
