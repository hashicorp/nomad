package mock

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/nomad/helper/uuid"
	testing "github.com/mitchellh/go-testing-interface"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

// StateStore defines the methods required from state.StateStore but avoids a
// circular dependency.
type StateStore interface {
	UpsertACLPolicies(msgType structs.MessageType, index uint64, policies []*structs.ACLPolicy) error
	UpsertACLTokens(msgType structs.MessageType, index uint64, tokens []*structs.ACLToken) error
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

// NamespacePolicyWithVariables is a helper for generating the policy hcl for a given
// namespace. Either policy or capabilities may be nil but not both.
func NamespacePolicyWithVariables(namespace string, policy string, capabilities []string, svars map[string][]string) string {
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

	policyHCL += VariablePolicy(svars)
	policyHCL += "\n}"
	return policyHCL
}

// VariablePolicy is a helper for generating the policy hcl for a given
// variable block inside of a namespace.
func VariablePolicy(svars map[string][]string) string {
	policyHCL := ""
	if len(svars) > 0 {
		policyHCL = "\n\n\tvariables {"
		for p, c := range svars {
			for i, s := range c {
				if !strings.HasPrefix(s, "\"") {
					c[i] = strconv.Quote(s)
				}
			}
			policyHCL += fmt.Sprintf("\n\t\tpath %q { capabilities = [%v]}", p, strings.Join(c, ","))
		}
		policyHCL += "\n\t}"
	}
	return policyHCL
}

// HostVolumePolicy is a helper for generating the policy hcl for a given
// host-volume. Either policy or capabilities may be nil but not both.
func HostVolumePolicy(vol string, policy string, capabilities []string) string {
	policyHCL := fmt.Sprintf("host_volume %q {", vol)
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

// PluginPolicy is a helper for generating the hcl for a given plugin policy.
func PluginPolicy(policy string) string {
	return fmt.Sprintf("plugin {\n\tpolicy = %q\n}\n", policy)
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
	assert.Nil(t, state.UpsertACLPolicies(structs.MsgTypeTestSetup, index, []*structs.ACLPolicy{policy}))
}

// CreateToken creates a local, client token for the given policies
func CreateToken(t testing.T, state StateStore, index uint64, policies []string) *structs.ACLToken {
	t.Helper()

	// Create the ACLToken
	token := ACLToken()
	token.Policies = policies
	token.SetHash()
	assert.Nil(t, state.UpsertACLTokens(structs.MsgTypeTestSetup, index, []*structs.ACLToken{token}))
	return token
}

// CreatePolicyAndToken creates a policy and then returns a token configured for
// just that policy. CreatePolicyAndToken uses the given index and index+1.
func CreatePolicyAndToken(t testing.T, state StateStore, index uint64, name, rule string) *structs.ACLToken {
	CreatePolicy(t, state, index, name, rule)
	return CreateToken(t, state, index+1, []string{name})
}

func ACLRole() *structs.ACLRole {
	role := structs.ACLRole{
		ID:          uuid.Generate(),
		Name:        fmt.Sprintf("acl-role-%s", uuid.Short()),
		Description: "mocked-test-acl-role",
		Policies: []*structs.ACLRolePolicyLink{
			{Name: "mocked-test-policy-1"},
			{Name: "mocked-test-policy-2"},
		},
		CreateIndex: 10,
		ModifyIndex: 10,
	}
	role.SetHash()
	return &role
}

func ACLPolicy() *structs.ACLPolicy {
	ap := &structs.ACLPolicy{
		Name:        fmt.Sprintf("policy-%s", uuid.Generate()),
		Description: "Super cool policy!",
		Rules: `
		namespace "default" {
			policy = "write"
		}
		node {
			policy = "read"
		}
		agent {
			policy = "read"
		}
		`,
		CreateIndex: 10,
		ModifyIndex: 20,
	}
	ap.SetHash()
	return ap
}

func ACLToken() *structs.ACLToken {
	tk := &structs.ACLToken{
		AccessorID:  uuid.Generate(),
		SecretID:    uuid.Generate(),
		Name:        "my cool token " + uuid.Generate(),
		Type:        "client",
		Policies:    []string{"foo", "bar"},
		Global:      false,
		CreateTime:  time.Now().UTC(),
		CreateIndex: 10,
		ModifyIndex: 20,
	}
	tk.SetHash()
	return tk
}

func ACLManagementToken() *structs.ACLToken {
	return &structs.ACLToken{
		AccessorID:  uuid.Generate(),
		SecretID:    uuid.Generate(),
		Name:        "management " + uuid.Generate(),
		Type:        "management",
		Global:      true,
		CreateTime:  time.Now().UTC(),
		CreateIndex: 10,
		ModifyIndex: 20,
	}
}
