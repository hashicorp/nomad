// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package vaultclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"text/template"
	"time"

	josejwt "github.com/go-jose/go-jose/v3/jwt"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/widmgr"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/vault/api"
	"github.com/shoenig/test/must"
)

const (
	jwtAuthMountPathTest = "jwt_test"

	jwtAuthConfigTemplate = `
{
  "jwks_url": "<<.JWKSURL>>",
  "jwt_supported_algs": ["EdDSA"],
  "default_role": "nomad-workloads"
}
`

	widVaultPolicyTemplate = `
path "secret/data/{{identity.entity.aliases.<<.JWTAuthAccessorID>>.metadata.nomad_namespace}}/{{identity.entity.aliases.<<.JWTAuthAccessorID>>.metadata.nomad_job_id}}/*" {
  capabilities = ["read"]
}

path "secret/data/{{identity.entity.aliases.<<.JWTAuthAccessorID>>.metadata.nomad_namespace}}/{{identity.entity.aliases.<<.JWTAuthAccessorID>>.metadata.nomad_job_id}}" {
  capabilities = ["read"]
}

path "secret/metadata/{{identity.entity.aliases.<<.JWTAuthAccessorID>>.metadata.nomad_namespace}}/*" {
  capabilities = ["list"]
}

path "secret/metadata/*" {
  capabilities = ["list"]
}
`

	widVaultRole = `
{
  "role_type": "jwt",
  "bound_audiences": "vault.io",
  "user_claim": "/nomad_job_id",
  "user_claim_json_pointer": true,
  "claim_mappings": {
    "nomad_namespace": "nomad_namespace",
    "nomad_job_id": "nomad_job_id"
  },
  "token_ttl": "30m",
  "token_type": "service",
  "token_period": "72h",
  "token_policies": ["nomad-workloads"]
}
`
)

func renderVaultTemplate(tmplStr string, data any) ([]byte, error) {
	var buf bytes.Buffer
	tmpl, err := template.New("policy").
		Delims("<<", ">>").
		Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse policy template: %w", err)
	}

	err = tmpl.Execute(&buf, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render policy template: %w", err)
	}

	return buf.Bytes(), nil
}

func setupVaultForWorkloadIdentity(v *testutil.TestVault, jwksURL string) error {
	logical := v.Client.Logical()
	sys := v.Client.Sys()
	ctx := context.Background()

	// Enable JWT auth method.
	err := sys.EnableAuthWithOptions(jwtAuthMountPathTest, &api.MountInput{
		Type: "jwt",
	})
	if err != nil {
		return fmt.Errorf("failed to enable JWT auth method: %w", err)
	}

	secret, err := logical.Read(fmt.Sprintf("sys/auth/%s", jwtAuthMountPathTest))
	jwtAuthAccessor := secret.Data["accessor"].(string)

	// Write JWT auth method config.
	jwtAuthConfigData := struct {
		JWKSURL string
	}{
		JWKSURL: jwksURL,
	}
	jwtAuthConfig, err := renderVaultTemplate(jwtAuthConfigTemplate, jwtAuthConfigData)
	if err != nil {
		return err
	}

	_, err = logical.WriteBytesWithContext(ctx, fmt.Sprintf("auth/%s/config", jwtAuthMountPathTest), jwtAuthConfig)
	if err != nil {
		return fmt.Errorf("failed to write JWT auth method config: %w", err)
	}

	// Write Nomad workload policy.
	data := struct {
		JWTAuthAccessorID string
	}{
		JWTAuthAccessorID: jwtAuthAccessor,
	}
	policy, err := renderVaultTemplate(widVaultPolicyTemplate, data)
	if err != nil {
		return err
	}

	encoded := base64.StdEncoding.EncodeToString(policy)
	policyReqBody := fmt.Sprintf(`{"policy": "%s"}`, encoded)

	policyPath := "sys/policies/acl/nomad-workloads"
	_, err = logical.WriteBytesWithContext(ctx, policyPath, []byte(policyReqBody))
	if err != nil {
		return fmt.Errorf("failed to write policy: %w", err)
	}

	// Write Nomad workload role.
	rolePath := fmt.Sprintf("auth/%s/role/nomad-workloads", jwtAuthMountPathTest)
	_, err = logical.WriteBytesWithContext(ctx, rolePath, []byte(widVaultRole))
	if err != nil {
		return fmt.Errorf("failed to write role: %w", err)
	}

	return nil
}

func TestVaultClient_DeriveTokenWithJWT(t *testing.T) {
	ci.Parallel(t)

	// Create signer and signed identities.
	alloc := mock.MinAlloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Identities = []*structs.WorkloadIdentity{
		{
			Name:     "vault_default",
			Audience: []string{"vault.io"},
			TTL:      time.Second,
		},
	}

	signer := widmgr.NewMockWIDSigner(task.Identities)
	signedWIDs, err := signer.SignIdentities(1, []*structs.WorkloadIdentityRequest{
		{
			AllocID: alloc.ID,
			WIHandle: structs.WIHandle{
				IdentityName:       task.Identities[0].Name,
				WorkloadIdentifier: task.Name,
				WorkloadType:       structs.WorkloadTypeTask,
			},
		},
	})
	must.NoError(t, err)
	must.Len(t, 1, signedWIDs)

	// Setup test JWKS server.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out, err := json.Marshal(signer.JSONWebKeySet())
		if err != nil {
			t.Errorf("failed to generate JWKS json response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprintln(w, string(out))
	}))
	defer ts.Close()

	// Start and configure Vault cluster for JWT authentication.
	v := testutil.NewTestVault(t)
	defer v.Stop()

	err = setupVaultForWorkloadIdentity(v, ts.URL)
	must.NoError(t, err)

	// Start Vault client.
	logger := testlog.HCLogger(t)
	v.Config.ConnectionRetryIntv = 100 * time.Millisecond
	v.Config.JWTAuthBackendPath = jwtAuthMountPathTest

	c, err := NewVaultClient(v.Config, logger, nil)
	must.NoError(t, err)

	c.Start()
	defer c.Stop()

	// Derive Vault token using signed JWT.
	jwtStr := signedWIDs[0].JWT
	token, err := c.DeriveTokenWithJWT(context.Background(), JWTLoginRequest{
		JWT: jwtStr,
	})
	must.NoError(t, err)
	must.NotEq(t, "", token)

	// Verify token has expected properties.
	v.Client.SetToken(token)
	s, err := v.Client.Logical().Read("auth/token/lookup-self")
	must.NoError(t, err)

	jwt, err := josejwt.ParseSigned(jwtStr)
	must.NoError(t, err)

	claims := make(map[string]any)
	err = jwt.UnsafeClaimsWithoutVerification(&claims)
	must.NoError(t, err)

	must.Eq(t, "service", s.Data["type"].(string))
	must.True(t, s.Data["renewable"].(bool))
	must.SliceContains(t, s.Data["policies"].([]any), "nomad-workloads")
	must.MapEq(t, map[string]any{
		"nomad_namespace": claims["nomad_namespace"],
		"nomad_job_id":    claims["nomad_job_id"],
		"role":            "nomad-workloads",
	}, s.Data["meta"].(map[string]any))

	// Verify token has the expected permissions.
	pathAllowed := fmt.Sprintf("secret/data/%s/%s/a", claims["nomad_namespace"], claims["nomad_job_id"])
	pathDenied := "secret/data/denied"

	s, err = v.Client.Logical().Write("sys/capabilities-self", map[string]any{
		"paths": []string{pathAllowed, pathDenied},
	})
	must.NoError(t, err)
	must.Eq(t, []any{"read"}, (s.Data[pathAllowed]).([]any))
	must.Eq(t, []any{"deny"}, (s.Data[pathDenied]).([]any))

	// Derive Vault token with non-existing role.
	token, err = c.DeriveTokenWithJWT(context.Background(), JWTLoginRequest{
		JWT:  jwtStr,
		Role: "test",
	})
	must.ErrorContains(t, err, `role "test" could not be found`)
}
