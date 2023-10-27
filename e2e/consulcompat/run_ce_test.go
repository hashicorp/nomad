// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consulcompat

import (
	"fmt"
	"os"
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

func testConsulBuildLegacy(t *testing.T, b build, baseDir string) {
	t.Run("consul-legacy("+b.Version+")", func(t *testing.T) {
		consulHTTPAddr, consulAPI := startConsul(t, b, baseDir, "")

		// smoke test before we continue
		verifyConsulVersion(t, consulAPI, b.Version)

		// we need an ACL policy that allows the Nomad agent to fingerprint
		// Consul, register services, render templates, and mint new SI tokens
		consulToken := setupConsulACLsForServices(t, consulAPI,
			"./input/consul-policy-for-nomad-legacy.hcl")

		// we need service intentions so Connect apps can reach each other
		setupConsulServiceIntentions(t, consulAPI)

		// note: Nomad needs to be live before we can setupConsul because we
		// need it up to serve the JWKS endpoint

		consulCfg := &testutil.Consul{
			Name:    "default",
			Address: consulHTTPAddr,
			Auth:    "",
			Token:   consulToken,
		}

		nc := startNomad(t, consulCfg)

		verifyConsulFingerprint(t, nc, b.Version, "default")
		runConnectJob(t, nc)
	})
}

func testConsulBuild(t *testing.T, b build, baseDir string) {
	t.Run("consul("+b.Version+")", func(t *testing.T) {
		consulHTTPAddr, consulAPI := startConsul(t, b, baseDir, "")

		// smoke test before we continue
		verifyConsulVersion(t, consulAPI, b.Version)

		// we need an ACL policy that only allows the Nomad agent to fingerprint
		// Consul and register itself, and set up service intentions
		consulToken := setupConsulACLsForServices(t, consulAPI,
			"./input/consul-policy-for-nomad.hcl")

		// we need service intentions so Connect apps can reach each other, and
		// an ACL role and policy that tasks will be able to use to render
		// templates
		setupConsulServiceIntentions(t, consulAPI)
		setupConsulACLsForTasks(t, consulAPI, "./input/consul-policy-for-tasks.hcl")

		// note: Nomad needs to be live before we can setup Consul auth methods
		// because we need it up to serve the JWKS endpoint

		consulCfg := &testutil.Consul{
			Name:                      "default",
			Address:                   consulHTTPAddr,
			Auth:                      "",
			Token:                     consulToken,
			ServiceIdentityAuthMethod: "nomad-services",
			ServiceIdentity: &testutil.WorkloadIdentityConfig{
				Audience: []string{"consul.io"},
				TTL:      "1h",
			},
			TaskIdentityAuthMethod: "nomad-tasks",
			TaskIdentity: &testutil.WorkloadIdentityConfig{
				Audience: []string{"consul.io"},
				TTL:      "1h",
			},
		}

		nc := startNomad(t, consulCfg)

		// configure authentication for WI to Consul
		setupConsulJWTAuthForServices(t, consulAPI, nc.Address())
		setupConsulJWTAuthForTasks(t, consulAPI, nc.Address())

		verifyConsulFingerprint(t, nc, b.Version, "default")
		runConnectJob(t, nc)
	})
}

// setupConsulACLsForServices installs a base set of ACL policies and returns a
// token that the Nomad agent can use
func setupConsulACLsForServices(t *testing.T, consulAPI *consulapi.Client, policyFilePath string) string {

	policyRules, err := os.ReadFile(policyFilePath)
	must.NoError(t, err, must.Sprintf("could not open policy file %s", policyFilePath))

	// policy without namespaces, for Consul CE. Note that with this policy we
	// must use Workload Identity for Connect jobs, or we'll get "failed to
	// derive SI token" errors from the client because the Nomad agent's token
	// doesn't have "acl:write"
	policy := &consulapi.ACLPolicy{
		Name:        "nomad-cluster-" + uuid.Short(),
		Description: "policy for nomad agent",
		Rules:       string(policyRules),
	}

	policy, _, err = consulAPI.ACL().PolicyCreate(policy, nil)
	must.NoError(t, err, must.Sprint("could not write policy to Consul"))

	token := &consulapi.ACLToken{
		Description: "token for Nomad agent",
		Policies: []*consulapi.ACLLink{{
			ID:   policy.ID,
			Name: policy.Name,
		}},
	}
	token, _, err = consulAPI.ACL().TokenCreate(token, nil)
	must.NoError(t, err, must.Sprint("could not create token in Consul"))

	return token.SecretID
}

func setupConsulServiceIntentions(t *testing.T, consulAPI *consulapi.Client) {
	ixn := &consulapi.Intention{
		SourceName:      "count-dashboard",
		DestinationName: "count-api",
		Action:          "allow",
	}
	_, err := consulAPI.Connect().IntentionUpsert(ixn, nil)
	must.NoError(t, err, must.Sprint("could not create intention"))
}

// setupConsulACLsForTasks installs a base set of ACL policies and returns a
// token that the Nomad agent can use
func setupConsulACLsForTasks(t *testing.T, consulAPI *consulapi.Client, policyFilePath string) {

	policyRules, err := os.ReadFile(policyFilePath)
	must.NoError(t, err, must.Sprintf("could not open policy file %s", policyFilePath))

	// policy without namespaces, for Consul CE.
	policy := &consulapi.ACLPolicy{
		Name:        "nomad-tasks-" + uuid.Short(),
		Description: "policy for nomad tasks",
		Rules:       string(policyRules),
	}

	policy, _, err = consulAPI.ACL().PolicyCreate(policy, nil)
	must.NoError(t, err, must.Sprint("could not write policy to Consul"))

	role := &consulapi.ACLRole{
		Name:        "nomad-default", // must match Nomad namespace
		Description: "role for nomad tasks",
		Policies: []*consulapi.ACLLink{{
			ID:   policy.ID,
			Name: policy.Name,
		}},
	}
	_, _, err = consulAPI.ACL().RoleCreate(role, nil)
	must.NoError(t, err, must.Sprint("could not create token in Consul"))
}

func setupConsulJWTAuthForServices(t *testing.T, consulAPI *consulapi.Client, address string) {

	authConfig := map[string]any{
		"JWKSURL":          fmt.Sprintf("%s/.well-known/jwks.json", address),
		"JWTSupportedAlgs": []string{"RS256"},
		"BoundAudiences":   "consul.io",
		"ClaimMappings": map[string]string{
			"nomad_namespace": "nomad_namespace",
			"nomad_job_id":    "nomad_job_id",
			"nomad_task":      "nomad_task",
			"nomad_service":   "nomad_service",
		},
	}

	// note: we can't include NamespaceRules here because Consul CE doesn't
	// support namespaces
	_, _, err := consulAPI.ACL().AuthMethodCreate(&consulapi.ACLAuthMethod{
		Name:          "nomad-services",
		Type:          "jwt",
		DisplayName:   "nomad-services",
		Description:   "login method for Nomad workload identities (WI)",
		MaxTokenTTL:   time.Hour,
		TokenLocality: "local",
		Config:        authConfig,
	}, nil)

	must.NoError(t, err, must.Sprint("could not create Consul auth method for services"))

	// note: we can't include Namespace here because Consul CE doesn't support
	// namespaces
	rule := &consulapi.ACLBindingRule{
		ID:          "",
		Description: "binding rule for Nomad workload identities (WI) for services",
		AuthMethod:  "nomad-services",
		Selector:    "",
		BindType:    "service",
		BindName:    "${value.nomad_service}",
	}
	_, _, err = consulAPI.ACL().BindingRuleCreate(rule, nil)
	must.NoError(t, err, must.Sprint("could not create Consul binding rule"))
}

func setupConsulJWTAuthForTasks(t *testing.T, consulAPI *consulapi.Client, address string) {

	authConfig := map[string]any{
		"JWKSURL":          fmt.Sprintf("%s/.well-known/jwks.json", address),
		"JWTSupportedAlgs": []string{"RS256"},
		"BoundAudiences":   "consul.io",
		"ClaimMappings": map[string]string{
			"nomad_namespace": "nomad_namespace",
			"nomad_job_id":    "nomad_job_id",
			"nomad_task":      "nomad_task",
			"nomad_service":   "nomad_service",
		},
	}

	// note: we can't include NamespaceRules here because Consul CE doesn't
	// support namespaces
	_, _, err := consulAPI.ACL().AuthMethodCreate(&consulapi.ACLAuthMethod{
		Name:          "nomad-tasks",
		Type:          "jwt",
		DisplayName:   "nomad-tasks",
		Description:   "login method for Nomad tasks with workload identity (WI)",
		MaxTokenTTL:   time.Hour,
		TokenLocality: "local",
		Config:        authConfig,
	}, nil)
	must.NoError(t, err, must.Sprint("could not create Consul auth method for tasks"))

	// note: we can't include Namespace here because Consul CE doesn't support
	// namespaces
	rule := &consulapi.ACLBindingRule{
		ID:          "",
		Description: "binding rule for Nomad workload identities (WI) for tasks",
		AuthMethod:  "nomad-tasks",
		Selector:    "",
		BindType:    "role",
		BindName:    "nomad-${value.nomad_namespace}",
	}
	_, _, err = consulAPI.ACL().BindingRuleCreate(rule, nil)
	must.NoError(t, err, must.Sprint("could not create Consul binding rule"))
}
