package nomad

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

// NamespacePolicy is a helper for generating the policy hcl for a given
// namepsace. Either policy or capabilites may be nil but not both.
func NamespacePolicy(namespace string, policy string, capabilities []string) string {
	policyHCL := fmt.Sprintf("namespace %q {", namespace)
	if policy != "" {
		policyHCL += fmt.Sprintf("\n\tpolicy = %q", policy)
	}
	if len(capabilities) != 0 {
		policyHCL += fmt.Sprintf("\n\tcapabilities = %q", capabilities)
	}
	policyHCL += "\n}"
	return policyHCL
}

// NodePolicy is a helper for generating the hcl for a given node policy.
func NodePolicy(policy string) string {
	return fmt.Sprintf("node {\n\tpolicy = %q\n}\n", policy)
}

// CreatePolicy creates a policy with the given name and rule.
func CreatePolicy(t *testing.T, state *state.StateStore, index uint64, name, rule string) {
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
func CreateToken(t *testing.T, state *state.StateStore, index uint64, policies []string) *structs.ACLToken {
	t.Helper()

	// Create the ACLToken
	token := mock.ACLToken()
	token.Policies = policies
	token.SetHash()
	assert.Nil(t, state.UpsertACLTokens(index, []*structs.ACLToken{token}))
	return token
}

// CreatePolicyAndToken creates a policy and then returns a token configured for
// just that policy. CreatePolicyAndToken uses the given index and index+1.
func CreatePolicyAndToken(t *testing.T, state *state.StateStore, index uint64, name, rule string) *structs.ACLToken {
	CreatePolicy(t, state, index, name, rule)
	return CreateToken(t, state, index+1, []string{name})
}
