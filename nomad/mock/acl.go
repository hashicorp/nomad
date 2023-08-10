// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mock

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/golang-jwt/jwt/v5"
	testing "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
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

func NodePoolPolicy(pool string, policy string, capabilities []string) string {
	tmplStr := `
node_pool "{{.Label}}" {
  {{- if .Policy}}
  policy = "{{.Policy}}"
  {{end -}}

  {{if gt (len .Capabilities) 0}}
  capabilities = [
    {{- range .Capabilities}}
    "{{.}}",
    {{- end}}
  ]
  {{- end}}
}`

	tmpl, err := template.New(pool).Parse(tmplStr)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, struct {
		Label        string
		Policy       string
		Capabilities []string
	}{pool, policy, capabilities})
	if err != nil {
		panic(err)
	}

	return buf.String()
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

func ACLOIDCAuthMethod() *structs.ACLAuthMethod {
	maxTokenTTL, _ := time.ParseDuration("3600s")
	method := structs.ACLAuthMethod{
		Name:          fmt.Sprintf("acl-auth-method-%s", uuid.Short()),
		Type:          "OIDC",
		TokenLocality: "local",
		MaxTokenTTL:   maxTokenTTL,
		Default:       false,
		Config: &structs.ACLAuthMethodConfig{
			OIDCDiscoveryURL:    "http://example.com",
			OIDCClientID:        "mock",
			OIDCClientSecret:    "very secret secret",
			OIDCScopes:          []string{"groups"},
			BoundAudiences:      []string{"sales", "engineering"},
			AllowedRedirectURIs: []string{"foo", "bar"},
			DiscoveryCaPem:      []string{"foo"},
			SigningAlgs:         []string{"RS256"},
			ClaimMappings:       map[string]string{"foo": "bar"},
			ListClaimMappings:   map[string]string{"foo": "bar"},
		},
		CreateTime:  time.Now().UTC(),
		CreateIndex: 10,
		ModifyIndex: 10,
	}
	method.Canonicalize()
	method.SetHash()
	return &method
}

func ACLJWTAuthMethod() *structs.ACLAuthMethod {
	maxTokenTTL, _ := time.ParseDuration("3600s")
	method := structs.ACLAuthMethod{
		Name:          fmt.Sprintf("acl-auth-method-%s", uuid.Short()),
		Type:          "JWT",
		TokenLocality: "local",
		MaxTokenTTL:   maxTokenTTL,
		Default:       false,
		Config: &structs.ACLAuthMethodConfig{
			JWTValidationPubKeys: []string{},
			OIDCDiscoveryURL:     "http://example.com",
			BoundAudiences:       []string{"sales", "engineering"},
			DiscoveryCaPem:       []string{"foo"},
			SigningAlgs:          []string{"RS256"},
			ClaimMappings:        map[string]string{"foo": "bar"},
			ListClaimMappings:    map[string]string{"foo": "bar"},
		},
		CreateTime:  time.Now().UTC(),
		CreateIndex: 10,
		ModifyIndex: 10,
	}
	method.Canonicalize()
	method.SetHash()
	return &method
}

// SampleJWTokenWithKeys takes a set of claims (can be nil) and optionally
// a private RSA key that should be used for signing the JWT, and returns:
// - a JWT signed with a randomly generated RSA key
// - PEM string of the public part of that key that can be used for validation.
func SampleJWTokenWithKeys(claims jwt.Claims, rsaKey *rsa.PrivateKey) (string, string, error) {
	var token, pubkeyPem string

	if rsaKey == nil {
		var err error
		rsaKey, err = rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return token, pubkeyPem, err
		}
	}

	pubkeyBytes, err := x509.MarshalPKIXPublicKey(rsaKey.Public())
	if err != nil {
		return token, pubkeyPem, err
	}
	pubkeyPem = string(pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: pubkeyBytes,
		},
	))

	var rawToken *jwt.Token
	if claims != nil {
		rawToken = jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	} else {
		rawToken = jwt.New(jwt.SigningMethodRS256)
	}

	token, err = rawToken.SignedString(rsaKey)
	if err != nil {
		return token, pubkeyPem, err
	}

	return token, pubkeyPem, nil
}

func ACLBindingRule() *structs.ACLBindingRule {
	return &structs.ACLBindingRule{
		ID:          uuid.Short(),
		Description: "mocked-acl-binding-rule",
		AuthMethod:  "auth0",
		Selector:    "engineering in list.roles",
		BindType:    "role",
		BindName:    "eng-ro",
		CreateTime:  time.Now().UTC(),
		ModifyTime:  time.Now().UTC(),
		CreateIndex: 10,
		ModifyIndex: 10,
	}
}
