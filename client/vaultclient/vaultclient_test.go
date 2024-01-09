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
	"strings"
	"testing"
	"text/template"
	"time"

	josejwt "github.com/go-jose/go-jose/v3/jwt"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/widmgr"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/useragent"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	structsc "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/vault/api"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/shoenig/test/must"
)

const (
	jwtAuthMountPathTest = "jwt_test"

	jwtAuthConfigTemplate = `
{
  "jwks_url": "<<.JWKSURL>>",
  "jwt_supported_algs": ["RS256", "EdDSA"],
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
	token, renewable, err := c.DeriveTokenWithJWT(context.Background(), JWTLoginRequest{
		JWT:       jwtStr,
		Namespace: "default",
	})
	must.NoError(t, err)
	must.NotEq(t, "", token)
	must.True(t, renewable)

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
	token, _, err = c.DeriveTokenWithJWT(context.Background(), JWTLoginRequest{
		JWT:       jwtStr,
		Role:      "test",
		Namespace: "default",
	})
	must.ErrorContains(t, err, `role "test" could not be found`)
}

func TestVaultClient_TokenRenewals(t *testing.T) {
	ci.Parallel(t)

	v := testutil.NewTestVault(t)
	defer v.Stop()

	logger := testlog.HCLogger(t)
	v.Config.ConnectionRetryIntv = 100 * time.Millisecond
	v.Config.TaskTokenTTL = "4s"
	c, err := NewVaultClient(v.Config, logger, nil)
	if err != nil {
		t.Fatalf("failed to build vault Vault: %v", err)
	}

	c.Start()
	defer c.Stop()

	// Sleep a little while to ensure that the renewal loop is active
	time.Sleep(time.Duration(testutil.TestMultiplier()) * time.Second)

	tcr := &vaultapi.TokenCreateRequest{
		Policies:    []string{"foo", "bar"},
		TTL:         "2s",
		DisplayName: "derived-for-task",
		Renewable:   new(bool),
	}
	*tcr.Renewable = true

	num := 5
	tokens := make([]string, num)
	for i := 0; i < num; i++ {
		c.client.SetToken(v.Config.Token)

		if err := c.client.SetAddress(v.Config.Addr); err != nil {
			t.Fatal(err)
		}

		secret, err := c.client.Auth().Token().Create(tcr)
		if err != nil {
			t.Fatalf("failed to create vault token: %v", err)
		}

		if secret == nil || secret.Auth == nil || secret.Auth.ClientToken == "" {
			t.Fatal("failed to derive a wrapped vault token")
		}

		tokens[i] = secret.Auth.ClientToken

		errCh, err := c.RenewToken(tokens[i], secret.Auth.LeaseDuration)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		go func(errCh <-chan error) {
			for {
				select {
				case err := <-errCh:
					must.NoError(t, err, must.Sprintf("unexpected error while renewing vault token"))
				}
			}
		}(errCh)
	}

	c.lock.Lock()
	length := c.heap.Length()
	c.lock.Unlock()
	if length != num {
		t.Fatalf("bad: Heap length: expected: %d, actual: %d", num, length)
	}

	time.Sleep(time.Duration(testutil.TestMultiplier()) * time.Second)

	for i := 0; i < num; i++ {
		if err := c.StopRenewToken(tokens[i]); err != nil {
			must.NoError(t, err)
		}
	}

	c.lock.Lock()
	length = c.heap.Length()
	c.lock.Unlock()
	if length != 0 {
		t.Fatalf("bad: Heap length: expected: 0, actual: %d", length)
	}
}

// TestVaultClient_NamespaceSupport tests that the Vault namespace Config, if present, will result in the
// namespace header being set on the created Vault Vault.
func TestVaultClient_NamespaceSupport(t *testing.T) {
	ci.Parallel(t)

	tr := true
	testNs := "test-namespace"

	logger := testlog.HCLogger(t)

	conf := structsc.DefaultVaultConfig()
	conf.Enabled = &tr
	conf.Token = "testvaulttoken"
	conf.Namespace = testNs
	c, err := NewVaultClient(conf, logger, nil)
	must.NoError(t, err)
	must.Eq(t, testNs, c.client.Headers().Get(structs.VaultNamespaceHeaderName))
}

func TestVaultClient_Heap(t *testing.T) {
	ci.Parallel(t)

	tr := true
	conf := structsc.DefaultVaultConfig()
	conf.Enabled = &tr
	conf.Token = "testvaulttoken"
	conf.TaskTokenTTL = "10s"

	logger := testlog.HCLogger(t)
	c, err := NewVaultClient(conf, logger, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("failed to create vault Vault")
	}

	now := time.Now()

	renewalReq1 := &vaultClientRenewalRequest{
		errCh:     make(chan error, 1),
		id:        "id1",
		increment: 10,
	}
	if err := c.heap.Push(renewalReq1, now.Add(50*time.Second)); err != nil {
		t.Fatal(err)
	}
	if !c.isTracked("id1") {
		t.Fatalf("id1 should have been tracked")
	}

	renewalReq2 := &vaultClientRenewalRequest{
		errCh:     make(chan error, 1),
		id:        "id2",
		increment: 10,
	}
	if err := c.heap.Push(renewalReq2, now.Add(40*time.Second)); err != nil {
		t.Fatal(err)
	}
	if !c.isTracked("id2") {
		t.Fatalf("id2 should have been tracked")
	}

	renewalReq3 := &vaultClientRenewalRequest{
		errCh:     make(chan error, 1),
		id:        "id3",
		increment: 10,
	}
	if err := c.heap.Push(renewalReq3, now.Add(60*time.Second)); err != nil {
		t.Fatal(err)
	}
	if !c.isTracked("id3") {
		t.Fatalf("id3 should have been tracked")
	}

	// Reading elements should yield id2, id1 and id3 in order
	req, _ := c.nextRenewal()
	if req != renewalReq2 {
		t.Fatalf("bad: expected: %#v, actual: %#v", renewalReq2, req)
	}
	if err := c.heap.Update(req, now.Add(70*time.Second)); err != nil {
		t.Fatal(err)
	}

	req, _ = c.nextRenewal()
	if req != renewalReq1 {
		t.Fatalf("bad: expected: %#v, actual: %#v", renewalReq1, req)
	}
	if err := c.heap.Update(req, now.Add(80*time.Second)); err != nil {
		t.Fatal(err)
	}

	req, _ = c.nextRenewal()
	if req != renewalReq3 {
		t.Fatalf("bad: expected: %#v, actual: %#v", renewalReq3, req)
	}
	if err := c.heap.Update(req, now.Add(90*time.Second)); err != nil {
		t.Fatal(err)
	}

	if err := c.StopRenewToken("id1"); err != nil {
		t.Fatal(err)
	}

	if err := c.StopRenewToken("id2"); err != nil {
		t.Fatal(err)
	}

	if err := c.StopRenewToken("id3"); err != nil {
		t.Fatal(err)
	}

	if c.isTracked("id1") {
		t.Fatalf("id1 should not have been tracked")
	}

	if c.isTracked("id1") {
		t.Fatalf("id1 should not have been tracked")
	}

	if c.isTracked("id1") {
		t.Fatalf("id1 should not have been tracked")
	}

}

func TestVaultClient_RenewNonRenewableLease(t *testing.T) {
	ci.Parallel(t)

	v := testutil.NewTestVault(t)
	defer v.Stop()

	logger := testlog.HCLogger(t)
	v.Config.ConnectionRetryIntv = 100 * time.Millisecond
	v.Config.TaskTokenTTL = "4s"
	c, err := NewVaultClient(v.Config, logger, nil)
	if err != nil {
		t.Fatalf("failed to build vault Vault: %v", err)
	}

	c.Start()
	defer c.Stop()

	// Sleep a little while to ensure that the renewal loop is active
	time.Sleep(time.Duration(testutil.TestMultiplier()) * time.Second)

	tcr := &vaultapi.TokenCreateRequest{
		Policies:    []string{"foo", "bar"},
		TTL:         "2s",
		DisplayName: "derived-for-task",
		Renewable:   new(bool),
	}

	c.client.SetToken(v.Config.Token)

	if err := c.client.SetAddress(v.Config.Addr); err != nil {
		t.Fatal(err)
	}

	secret, err := c.client.Auth().Token().Create(tcr)
	if err != nil {
		t.Fatalf("failed to create vault token: %v", err)
	}

	if secret == nil || secret.Auth == nil || secret.Auth.ClientToken == "" {
		t.Fatal("failed to derive a wrapped vault token")
	}

	_, err = c.RenewToken(secret.Auth.ClientToken, secret.Auth.LeaseDuration)
	if err == nil {
		t.Fatalf("expected error, got nil")
	} else if !strings.Contains(err.Error(), "lease is not renewable") {
		t.Fatalf("expected \"%s\" in error message, got \"%v\"", "lease is not renewable", err)
	}
}

func TestVaultClient_RenewNonexistentLease(t *testing.T) {
	ci.Parallel(t)

	v := testutil.NewTestVault(t)
	defer v.Stop()

	logger := testlog.HCLogger(t)
	v.Config.ConnectionRetryIntv = 100 * time.Millisecond
	v.Config.TaskTokenTTL = "4s"
	c, err := NewVaultClient(v.Config, logger, nil)
	if err != nil {
		t.Fatalf("failed to build vault Vault: %v", err)
	}

	c.Start()
	defer c.Stop()

	// Sleep a little while to ensure that the renewal loop is active
	time.Sleep(time.Duration(testutil.TestMultiplier()) * time.Second)

	c.client.SetToken(v.Config.Token)

	if err := c.client.SetAddress(v.Config.Addr); err != nil {
		t.Fatal(err)
	}

	_, err = c.RenewToken(c.client.Token(), 10)
	if err == nil {
		t.Fatalf("expected error, got nil")
		// The Vault error message changed between 0.10.2 and 1.0.1
	} else if !strings.Contains(err.Error(), "lease not found") && !strings.Contains(err.Error(), "lease is not renewable") {
		t.Fatalf("expected \"%s\" or \"%s\" in error message, got \"%v\"", "lease not found", "lease is not renewable", err.Error())
	}
}

// TestVaultClient_RenewalTime_Long asserts that for leases over 1m the renewal
// time is jittered.
func TestVaultClient_RenewalTime_Long(t *testing.T) {
	ci.Parallel(t)

	// highRoller is a randIntn func that always returns the max value
	highRoller := func(n int) int {
		return n - 1
	}

	// lowRoller is a randIntn func that always returns the min value (0)
	lowRoller := func(int) int {
		return 0
	}

	must.Eq(t, 39*time.Second, renewalTime(highRoller, 60))
	must.Eq(t, 20*time.Second, renewalTime(lowRoller, 60))

	must.Eq(t, 309*time.Second, renewalTime(highRoller, 600))
	must.Eq(t, 290*time.Second, renewalTime(lowRoller, 600))

	const days3 = 60 * 60 * 24 * 3
	must.Eq(t, (days3/2+9)*time.Second, renewalTime(highRoller, days3))
	must.Eq(t, (days3/2-10)*time.Second, renewalTime(lowRoller, days3))
}

// TestVaultClient_RenewalTime_Short asserts that for leases under 1m the renewal
// time is lease/2.
func TestVaultClient_RenewalTime_Short(t *testing.T) {
	ci.Parallel(t)

	dice := func(int) int {
		t.Error("dice should not have been called")
		panic("unreachable")
	}

	must.Eq(t, 29*time.Second, renewalTime(dice, 58))
	must.Eq(t, 15*time.Second, renewalTime(dice, 30))
	must.Eq(t, 1*time.Second, renewalTime(dice, 2))
}

func TestVaultClient_SetUserAgent(t *testing.T) {
	ci.Parallel(t)

	conf := structsc.DefaultVaultConfig()
	conf.Enabled = pointer.Of(true)
	logger := testlog.HCLogger(t)
	c, err := NewVaultClient(conf, logger, nil)
	must.NoError(t, err)

	ua := c.client.Headers().Get("User-Agent")
	must.Eq(t, useragent.String(), ua)
}
