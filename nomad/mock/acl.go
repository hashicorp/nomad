package mock

import (
	"fmt"
	"strconv"
	"strings"

	testing "github.com/mitchellh/go-testing-interface"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

// StateStore defines the methods required from state.StateStore but avoids a
// circular dependency.
type StateStore interface {
	UpsertACLPolicies(index uint64, policies []*structs.ACLPolicy) error
	UpsertACLTokens(index uint64, tokens []*structs.ACLToken) error
}

// NamespacePolicy is a helper for generating the policy hcl for a given
// namespace. Either policy or capabilities may be nil but not both.
func NamespacePolicy(namespace string, policy string, capabilities []string) string {
	policyHCL := fmt.Sprintf("namespace %q {", namespace)
	if policy != "" {
		policyHCL += fmt.Sprintf("\n\tpolicy = %q", policy)
	}
	if len(capabilities) != 0 {
		for i, s := range capabilities {
			if !strings.HasPrefix(s, "\"") {
				capabilities[i] = strconv.Quote(s)
			}
		}

		policyHCL += fmt.Sprintf("\n\tcapabilities = [%v]", strings.Join(capabilities, ","))
	}
	policyHCL += "\n}"
	return policyHCL
}

// AgentPolicy is a helper for generating the hcl for a given agent policy.
func AgentPolicy(policy string) string {
	return fmt.Sprintf("agent {\n\tpolicy = %q\n}\n", policy)
}

// NodePolicy is a helper for generating the hcl for a given node policy.
func NodePolicy(policy string) string {
	return fmt.Sprintf("node {\n\tpolicy = %q\n}\n", policy)
}

// QuotaPolicy is a helper for generating the hcl for a given quota policy.
func QuotaPolicy(policy string) string {
	return fmt.Sprintf("quota {\n\tpolicy = %q\n}\n", policy)
}

// CreatePolicy creates a policy with the given name and rule.
func CreatePolicy(t testing.T, state StateStore, index uint64, name, rule string) {
	t.Helper()

	// Create the ACLPolicy
	policy := &structs.ACLPolicy{
		Name:  name,
		Rules: rule,
	}
	policy.SetHash()
	assert.Nil(t, state.UpsertACLPolicies(index, []*structs.ACLPolicy{policy}))
}

// CreateToken creates a local, client token for the given policies
func CreateToken(t testing.T, state StateStore, index uint64, policies []string) *structs.ACLToken {
	t.Helper()

	// Create the ACLToken
	token := ACLToken()
	token.Policies = policies
	token.SetHash()
	assert.Nil(t, state.UpsertACLTokens(index, []*structs.ACLToken{token}))
	return token
}

// CreatePolicyAndToken creates a policy and then returns a token configured for
// just that policy. CreatePolicyAndToken uses the given index and index+1.
func CreatePolicyAndToken(t testing.T, state StateStore, index uint64, name, rule string) *structs.ACLToken {
	CreatePolicy(t, state, index, name, rule)
	return CreateToken(t, state, index+1, []string{name})
}
