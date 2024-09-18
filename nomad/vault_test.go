// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	vapi "github.com/hashicorp/vault/api"
)

const (
	// nomadRoleManagementPolicy is a policy that allows nomad to manage tokens
	nomadRoleManagementPolicy = `
path "auth/token/renew-self" {
	capabilities = ["update"]
}

path "auth/token/lookup" {
	capabilities = ["update"]
}

path "auth/token/roles/test" {
	capabilities = ["read"]
}

path "auth/token/revoke-accessor" {
	capabilities = ["update"]
}
`

	// tokenLookupPolicy allows a token to be looked up
	tokenLookupPolicy = `
path "auth/token/lookup" {
	capabilities = ["update"]
}
`

	// nomadRoleCreatePolicy gives the ability to create the role and derive tokens
	// from the test role
	nomadRoleCreatePolicy = `
path "auth/token/create/test" {
	capabilities = ["create", "update"]
}
`

	// secretPolicy gives access to the secret mount
	secretPolicy = `
path "secret/*" {
	capabilities = ["create", "read", "update", "delete", "list"]
}
`
)

// defaultTestVaultAllowlistRoleAndToken creates a test Vault role and returns a token
// created in that role
func defaultTestVaultAllowlistRoleAndToken(v *testutil.TestVault, t *testing.T, rolePeriod int) string {
	vaultPolicies := map[string]string{
		"nomad-role-create":     nomadRoleCreatePolicy,
		"nomad-role-management": nomadRoleManagementPolicy,
	}
	d := make(map[string]interface{}, 2)
	d["allowed_policies"] = "nomad-role-create,nomad-role-management"
	d["period"] = rolePeriod
	return testVaultRoleAndToken(v, t, vaultPolicies, d,
		[]string{"nomad-role-create", "nomad-role-management"})
}

// defaultTestVaultDenylistRoleAndToken creates a test Vault role using
// disallowed_policies and returns a token created in that role
func defaultTestVaultDenylistRoleAndToken(v *testutil.TestVault, t *testing.T, rolePeriod int) string {
	vaultPolicies := map[string]string{
		"nomad-role-create":     nomadRoleCreatePolicy,
		"nomad-role-management": nomadRoleManagementPolicy,
		"secrets":               secretPolicy,
	}

	// Create the role
	d := make(map[string]interface{}, 2)
	d["disallowed_policies"] = "nomad-role-create"
	d["period"] = rolePeriod
	testVaultRoleAndToken(v, t, vaultPolicies, d, []string{"default"})

	// Create a token that can use the role
	a := v.Client.Auth().Token()
	req := &vapi.TokenCreateRequest{
		Policies: []string{"nomad-role-create", "nomad-role-management"},
	}
	s, err := a.Create(req)
	if err != nil {
		t.Fatalf("failed to create child token: %v", err)
	}

	if s == nil || s.Auth == nil {
		t.Fatalf("bad secret response: %+v", s)
	}

	return s.Auth.ClientToken
}

// testVaultRoleAndToken writes the vaultPolicies to vault and then creates a
// test role with the passed data. After that it derives a token from the role
// with the tokenPolicies
func testVaultRoleAndToken(v *testutil.TestVault, t *testing.T, vaultPolicies map[string]string,
	data map[string]interface{}, tokenPolicies []string) string {
	// Write the policies
	sys := v.Client.Sys()
	for p, data := range vaultPolicies {
		if err := sys.PutPolicy(p, data); err != nil {
			t.Fatalf("failed to create %q policy: %v", p, err)
		}
	}

	// Build a role
	l := v.Client.Logical()
	l.Write("auth/token/roles/test", data)

	// Create a new token with the role
	a := v.Client.Auth().Token()
	req := vapi.TokenCreateRequest{
		Policies: tokenPolicies,
	}
	s, err := a.CreateWithRole(&req, "test")
	if err != nil {
		t.Fatalf("failed to create child token: %v", err)
	}

	// Get the client token
	if s == nil || s.Auth == nil {
		t.Fatalf("bad secret response: %+v", s)
	}

	return s.Auth.ClientToken
}

func TestVaultClient_BadConfig(t *testing.T) {
	ci.Parallel(t)
	conf := &config.VaultConfig{}
	logger := testlog.HCLogger(t)

	// Should be no error since Vault is not enabled
	_, err := NewVaultClient(nil, logger, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "valid") {
		t.Fatalf("expected config error: %v", err)
	}

	tr := true
	conf.Enabled = &tr
	_, err = NewVaultClient(conf, logger, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "token must be set") {
		t.Fatalf("Expected token unset error: %v", err)
	}

	conf.Token = "123"
	_, err = NewVaultClient(conf, logger, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "address must be set") {
		t.Fatalf("Expected address unset error: %v", err)
	}
}

// TestVaultClient_WithNamespaceSupport tests that the Vault namespace config, if present, will result in the
// namespace header being set on the created Vault client.
func TestVaultClient_WithNamespaceSupport(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	tr := true
	testNs := "test-namespace"
	conf := &config.VaultConfig{
		Addr:      "https://vault.service.consul:8200",
		Enabled:   &tr,
		Token:     "testvaulttoken",
		Namespace: testNs,
	}
	logger := testlog.HCLogger(t)

	// Should be no error since Vault is not enabled
	c, err := NewVaultClient(conf, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}

	require.Equal(testNs, c.client.Headers().Get(structs.VaultNamespaceHeaderName))
	require.Equal("", c.clientSys.Headers().Get(structs.VaultNamespaceHeaderName))
	require.NotEqual(c.clientSys, c.client)
}

// TestVaultClient_WithoutNamespaceSupport tests that the Vault namespace config, if present, will result in the
// namespace header being set on the created Vault client.
func TestVaultClient_WithoutNamespaceSupport(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	tr := true
	conf := &config.VaultConfig{
		Addr:      "https://vault.service.consul:8200",
		Enabled:   &tr,
		Token:     "testvaulttoken",
		Namespace: "",
	}
	logger := testlog.HCLogger(t)

	// Should be no error since Vault is not enabled
	c, err := NewVaultClient(conf, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}

	require.Equal("", c.client.Headers().Get(structs.VaultNamespaceHeaderName))
	require.Equal("", c.clientSys.Headers().Get(structs.VaultNamespaceHeaderName))
	require.Equal(c.clientSys, c.client)
}

// started separately.
// Test that the Vault Client can establish a connection even if it is started
// before Vault is available.
func TestVaultClient_EstablishConnection(t *testing.T) {
	ci.Parallel(t)
	for i := 10; i >= 0; i-- {
		v := testutil.NewTestVaultDelayed(t)
		logger := testlog.HCLogger(t)
		v.Config.ConnectionRetryIntv = 100 * time.Millisecond
		client, err := NewVaultClient(v.Config, logger, nil, nil)
		if err != nil {
			t.Fatalf("failed to build vault client: %v", err)
		}

		// Sleep a little while and check that no connection has been established.
		time.Sleep(100 * time.Duration(testutil.TestMultiplier()) * time.Millisecond)
		if established, _ := client.ConnectionEstablished(); established {
			t.Fatalf("ConnectionEstablished() returned true before Vault server started")
		}

		// Start Vault
		if err := v.Start(); err != nil {
			v.Stop()
			client.Stop()

			if i == 0 {
				t.Fatalf("Failed to start vault: %v", err)
			}

			wait := time.Duration(rand.Int31n(2000)) * time.Millisecond
			time.Sleep(wait)
			continue
		}

		var waitErr error
		testutil.WaitForResult(func() (bool, error) {
			return client.ConnectionEstablished()
		}, func(err error) {
			waitErr = err
		})

		v.Stop()
		client.Stop()
		if waitErr != nil {
			if i == 0 {
				t.Fatalf("Failed to start vault: %v", err)
			}

			wait := time.Duration(rand.Int31n(2000)) * time.Millisecond
			time.Sleep(wait)
			continue
		}

		break
	}
}

func TestVaultClient_ValidateRole(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Set the configs token in a new test role
	vaultPolicies := map[string]string{
		"nomad-role-create":     nomadRoleCreatePolicy,
		"nomad-role-management": nomadRoleManagementPolicy,
	}
	data := map[string]interface{}{
		"allowed_policies":       "default,root",
		"orphan":                 true,
		"renewable":              true,
		"token_explicit_max_ttl": 10,
	}
	v.Config.Token = testVaultRoleAndToken(v, t, vaultPolicies, data, nil)

	logger := testlog.HCLogger(t)
	v.Config.ConnectionRetryIntv = 100 * time.Millisecond
	client, err := NewVaultClient(v.Config, logger, nil, nil)
	require.NoError(t, err)

	defer client.Stop()

	// Wait for an error
	var conn bool
	var connErr error
	testutil.WaitForResult(func() (bool, error) {
		conn, connErr = client.ConnectionEstablished()
		if !conn {
			return false, fmt.Errorf("Should connect")
		}

		if connErr == nil {
			return false, fmt.Errorf("expect an error")
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	require.Contains(t, connErr.Error(), "explicit max ttl")
	require.Contains(t, connErr.Error(), "non-zero period")
}

// TestVaultClient_ValidateRole_Success asserts that a valid token role
// gets marked as valid
func TestVaultClient_ValidateRole_Success(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Set the configs token in a new test role
	vaultPolicies := map[string]string{
		"nomad-role-create":     nomadRoleCreatePolicy,
		"nomad-role-management": nomadRoleManagementPolicy,
	}
	data := map[string]interface{}{
		"allowed_policies": "default,root",
		"orphan":           true,
		"renewable":        true,
		"token_period":     1000,
	}
	v.Config.Token = testVaultRoleAndToken(v, t, vaultPolicies, data, nil)

	logger := testlog.HCLogger(t)
	v.Config.ConnectionRetryIntv = 100 * time.Millisecond
	client, err := NewVaultClient(v.Config, logger, nil, nil)
	require.NoError(t, err)

	defer client.Stop()

	// Wait for an error
	var conn bool
	var connErr error
	testutil.WaitForResult(func() (bool, error) {
		conn, connErr = client.ConnectionEstablished()
		if !conn {
			return false, fmt.Errorf("Should connect")
		}

		if connErr != nil {
			return false, connErr
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}

// TestVaultClient_ValidateRole_Deprecated_Success asserts that a valid token
// role gets marked as valid, even if it uses deprecated field, period
func TestVaultClient_ValidateRole_Deprecated_Success(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Set the configs token in a new test role
	vaultPolicies := map[string]string{
		"nomad-role-create":     nomadRoleCreatePolicy,
		"nomad-role-management": nomadRoleManagementPolicy,
	}
	data := map[string]interface{}{
		"allowed_policies": "default,root",
		"orphan":           true,
		"renewable":        true,
		"period":           1000,
	}
	v.Config.Token = testVaultRoleAndToken(v, t, vaultPolicies, data, nil)

	logger := testlog.HCLogger(t)
	v.Config.ConnectionRetryIntv = 100 * time.Millisecond
	client, err := NewVaultClient(v.Config, logger, nil, nil)
	require.NoError(t, err)

	defer client.Stop()

	// Wait for an error
	var conn bool
	var connErr error
	testutil.WaitForResult(func() (bool, error) {
		conn, connErr = client.ConnectionEstablished()
		if !conn {
			return false, fmt.Errorf("Should connect")
		}

		if connErr != nil {
			return false, connErr
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}

func TestVaultClient_ValidateRole_NonExistent(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	v.Config.Token = defaultTestVaultAllowlistRoleAndToken(v, t, 5)
	v.Config.Token = v.RootToken
	logger := testlog.HCLogger(t)
	v.Config.ConnectionRetryIntv = 100 * time.Millisecond
	v.Config.Role = "test-nonexistent"
	client, err := NewVaultClient(v.Config, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	defer client.Stop()

	// Wait for an error
	var conn bool
	var connErr error
	testutil.WaitForResult(func() (bool, error) {
		conn, connErr = client.ConnectionEstablished()
		if !conn {
			return false, fmt.Errorf("Should connect")
		}

		if connErr == nil {
			return false, fmt.Errorf("expect an error")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("bad: %v", err)
	})

	errStr := connErr.Error()
	if !strings.Contains(errStr, "does not exist") {
		t.Fatalf("Expect does not exist error")
	}
}

func TestVaultClient_ValidateToken(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Set the configs token in a new test role
	vaultPolicies := map[string]string{
		"nomad-role-create": nomadRoleCreatePolicy,
		"token-lookup":      tokenLookupPolicy,
	}
	data := map[string]interface{}{
		"allowed_policies": "token-lookup,nomad-role-create",
		"period":           10,
	}
	v.Config.Token = testVaultRoleAndToken(v, t, vaultPolicies, data, []string{"token-lookup", "nomad-role-create"})

	logger := testlog.HCLogger(t)
	v.Config.ConnectionRetryIntv = 100 * time.Millisecond
	client, err := NewVaultClient(v.Config, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	defer client.Stop()

	// Wait for an error
	var conn bool
	var connErr error
	testutil.WaitForResult(func() (bool, error) {
		conn, connErr = client.ConnectionEstablished()
		if !conn {
			return false, fmt.Errorf("Should connect")
		}

		if connErr == nil {
			return false, fmt.Errorf("expect an error")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("bad: %v", err)
	})

	errStr := connErr.Error()
	if !strings.Contains(errStr, vaultTokenRevokePath) {
		t.Fatalf("Expect revoke error")
	}
	if !strings.Contains(errStr, fmt.Sprintf(vaultRoleLookupPath, "test")) {
		t.Fatalf("Expect explicit max ttl error")
	}
	if !strings.Contains(errStr, "token must have one of the following") {
		t.Fatalf("Expect explicit max ttl error")
	}
}

func TestVaultClient_SetActive(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(v.Config, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	defer client.Stop()

	waitForConnection(client, t)

	// Do a lookup and expect an error about not being active
	_, err = client.LookupToken(context.Background(), "123")
	if err == nil || !strings.Contains(err.Error(), "not active") {
		t.Fatalf("Expected not-active error: %v", err)
	}

	client.SetActive(true)

	// Do a lookup of ourselves
	_, err = client.LookupToken(context.Background(), v.RootToken)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

// Test that we can update the config and things keep working
func TestVaultClient_SetConfig(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	v2 := testutil.NewTestVault(t)
	defer v2.Stop()

	// Set the configs token in a new test role
	v2.Config.Token = defaultTestVaultAllowlistRoleAndToken(v2, t, 20)

	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(v.Config, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	defer client.Stop()

	waitForConnection(client, t)

	if client.tokenData == nil || len(client.tokenData.Policies) != 1 {
		t.Fatalf("unexpected token: %v", client.tokenData)
	}

	// Update the config
	if err := client.SetConfig(v2.Config); err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}

	waitForConnection(client, t)

	if client.tokenData == nil || len(client.tokenData.Policies) != 3 {
		t.Fatalf("unexpected token: %v", client.tokenData)
	}

	// Test that when SetConfig is called with the same configuration, it is a
	// no-op
	failCh := make(chan struct{}, 1)
	go func() {
		tomb := client.tomb
		select {
		case <-tomb.Dying():
			close(failCh)
		case <-time.After(1 * time.Second):
			return
		}
	}()

	// Update the config
	if err := client.SetConfig(v2.Config); err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}

	select {
	case <-failCh:
		t.Fatalf("Tomb shouldn't have exited")
	case <-time.After(1 * time.Second):
		return
	}
}

// TestVaultClient_SetConfig_Deadlock asserts that calling SetConfig
// concurrently with establishConnection does not deadlock.
func TestVaultClient_SetConfig_Deadlock(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	v2 := testutil.NewTestVault(t)
	defer v2.Stop()

	// Set the configs token in a new test role
	v2.Config.Token = defaultTestVaultAllowlistRoleAndToken(v2, t, 20)

	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(v.Config, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	defer client.Stop()

	for i := 0; i < 100; i++ {
		// Alternate configs to cause updates
		conf := v.Config
		if i%2 == 0 {
			conf = v2.Config
		}
		if err := client.SetConfig(conf); err != nil {
			t.Fatalf("SetConfig failed: %v", err)
		}
	}
}

// Test that we can disable vault
func TestVaultClient_SetConfig_Disable(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(v.Config, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	defer client.Stop()

	waitForConnection(client, t)

	if client.tokenData == nil || len(client.tokenData.Policies) != 1 {
		t.Fatalf("unexpected token: %v", client.tokenData)
	}

	// Disable vault
	f := false
	config := config.VaultConfig{
		Enabled: &f,
	}

	// Update the config
	if err := client.SetConfig(&config); err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}

	if client.Enabled() || client.Running() {
		t.Fatalf("SetConfig should have stopped client")
	}
}

func TestVaultClient_RenewalLoop(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Set the configs token in a new test role
	v.Config.Token = defaultTestVaultAllowlistRoleAndToken(v, t, 5)

	// Start the client
	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(v.Config, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	defer client.Stop()

	// Sleep 8 seconds and ensure we have a non-zero TTL
	time.Sleep(8 * time.Second)

	// Get the current TTL
	a := v.Client.Auth().Token()
	s2, err := a.Lookup(v.Config.Token)
	if err != nil {
		t.Fatalf("failed to lookup token: %v", err)
	}

	ttl := parseTTLFromLookup(s2, t)
	if ttl == 0 {
		t.Fatalf("token renewal failed; ttl %v", ttl)
	}

	if client.currentExpiration.Before(time.Now()) {
		t.Fatalf("found current expiration to be in past %s", time.Until(client.currentExpiration))
	}
}

func TestVaultClientRenewUpdatesExpiration(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Set the configs token in a new test role
	v.Config.Token = defaultTestVaultAllowlistRoleAndToken(v, t, 5)

	// Start the client
	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(v.Config, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	defer client.Stop()

	// Get the current TTL
	a := v.Client.Auth().Token()
	s2, err := a.Lookup(v.Config.Token)
	if err != nil {
		t.Fatalf("failed to lookup token: %v", err)
	}
	exp0 := time.Now().Add(time.Duration(parseTTLFromLookup(s2, t)) * time.Second)

	time.Sleep(1 * time.Second)

	_, err = client.renew()
	require.NoError(t, err)
	exp1 := client.currentExpiration
	require.True(t, exp0.Before(exp1))

	time.Sleep(1 * time.Second)

	_, err = client.renew()
	require.NoError(t, err)
	exp2 := client.currentExpiration
	require.True(t, exp1.Before(exp2))
}

func TestVaultClient_StopsAfterPermissionError(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Set the configs token in a new test role
	v.Config.Token = defaultTestVaultAllowlistRoleAndToken(v, t, 2)

	// Start the client
	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(v.Config, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	defer client.Stop()

	time.Sleep(500 * time.Millisecond)

	assert.True(t, client.isRenewLoopActive())

	// Get the current TTL
	a := v.Client.Auth().Token()
	assert.NoError(t, a.RevokeSelf(""))

	testutil.WaitForResult(func() (bool, error) {
		if !client.isRenewLoopActive() {
			return true, nil
		} else {
			return false, errors.New("renew loop should terminate after token is revoked")
		}
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}
func TestVaultClient_LoopsUntilCannotRenew(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Set the configs token in a new test role
	v.Config.Token = defaultTestVaultAllowlistRoleAndToken(v, t, 5)

	// Start the client
	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(v.Config, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	defer client.Stop()

	// Sleep 8 seconds and ensure we have a non-zero TTL
	time.Sleep(8 * time.Second)

	// Get the current TTL
	a := v.Client.Auth().Token()
	s2, err := a.Lookup(v.Config.Token)
	if err != nil {
		t.Fatalf("failed to lookup token: %v", err)
	}

	ttl := parseTTLFromLookup(s2, t)
	if ttl == 0 {
		t.Fatalf("token renewal failed; ttl %v", ttl)
	}

	if client.currentExpiration.Before(time.Now()) {
		t.Fatalf("found current expiration to be in past %s", time.Until(client.currentExpiration))
	}
}

func parseTTLFromLookup(s *vapi.Secret, t *testing.T) int64 {
	if s == nil {
		t.Fatalf("nil secret")
	} else if s.Data == nil {
		t.Fatalf("nil data block in secret")
	}

	ttlRaw, ok := s.Data["ttl"]
	if !ok {
		t.Fatalf("no ttl")
	}

	ttlNumber, ok := ttlRaw.(json.Number)
	if !ok {
		t.Fatalf("failed to convert ttl %q to json Number", ttlRaw)
	}

	ttl, err := ttlNumber.Int64()
	if err != nil {
		t.Fatalf("Failed to get ttl from json.Number: %v", err)
	}

	return ttl
}

func TestVaultClient_LookupToken_Invalid(t *testing.T) {
	ci.Parallel(t)
	tr := true
	conf := &config.VaultConfig{
		Enabled: &tr,
		Addr:    "http://foobar:12345",
		Token:   uuid.Generate(),
	}

	// Enable vault but use a bad address so it never establishes a conn
	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(conf, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	client.SetActive(true)
	defer client.Stop()

	_, err = client.LookupToken(context.Background(), "foo")
	if err == nil || !strings.Contains(err.Error(), "established") {
		t.Fatalf("Expected error because connection to Vault hasn't been made: %v", err)
	}
}

func TestVaultClient_LookupToken_Root(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(v.Config, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	client.SetActive(true)
	defer client.Stop()

	waitForConnection(client, t)

	// Lookup ourselves
	s, err := client.LookupToken(context.Background(), v.Config.Token)
	if err != nil {
		t.Fatalf("self lookup failed: %v", err)
	}

	policies, err := s.TokenPolicies()
	if err != nil {
		t.Fatalf("failed to parse policies: %v", err)
	}

	expected := []string{"root"}
	if !reflect.DeepEqual(policies, expected) {
		t.Fatalf("Unexpected policies; got %v; want %v", policies, expected)
	}

	// Create a token with a different set of policies
	expected = []string{"default"}
	req := vapi.TokenCreateRequest{
		Policies: expected,
	}
	s, err = v.Client.Auth().Token().Create(&req)
	if err != nil {
		t.Fatalf("failed to create child token: %v", err)
	}

	// Get the client token
	if s == nil || s.Auth == nil {
		t.Fatalf("bad secret response: %+v", s)
	}

	// Lookup new child
	s, err = client.LookupToken(context.Background(), s.Auth.ClientToken)
	if err != nil {
		t.Fatalf("self lookup failed: %v", err)
	}

	policies, err = s.TokenPolicies()
	if err != nil {
		t.Fatalf("failed to parse policies: %v", err)
	}

	if !reflect.DeepEqual(policies, expected) {
		t.Fatalf("Unexpected policies; got %v; want %v", policies, expected)
	}
}

func TestVaultClient_LookupToken_Role(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Set the configs token in a new test role
	v.Config.Token = defaultTestVaultAllowlistRoleAndToken(v, t, 5)

	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(v.Config, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	client.SetActive(true)
	defer client.Stop()

	waitForConnection(client, t)

	// Lookup ourselves
	s, err := client.LookupToken(context.Background(), v.Config.Token)
	if err != nil {
		t.Fatalf("self lookup failed: %v", err)
	}

	policies, err := s.TokenPolicies()
	if err != nil {
		t.Fatalf("failed to parse policies: %v", err)
	}

	expected := []string{"default", "nomad-role-create", "nomad-role-management"}
	if !reflect.DeepEqual(policies, expected) {
		t.Fatalf("Unexpected policies; got %v; want %v", policies, expected)
	}

	// Create a token with a different set of policies
	expected = []string{"default"}
	req := vapi.TokenCreateRequest{
		Policies: expected,
	}
	s, err = v.Client.Auth().Token().Create(&req)
	if err != nil {
		t.Fatalf("failed to create child token: %v", err)
	}

	// Get the client token
	if s == nil || s.Auth == nil {
		t.Fatalf("bad secret response: %+v", s)
	}

	// Lookup new child
	s, err = client.LookupToken(context.Background(), s.Auth.ClientToken)
	if err != nil {
		t.Fatalf("self lookup failed: %v", err)
	}

	policies, err = s.TokenPolicies()
	if err != nil {
		t.Fatalf("failed to parse policies: %v", err)
	}

	if !reflect.DeepEqual(policies, expected) {
		t.Fatalf("Unexpected policies; got %v; want %v", policies, expected)
	}
}

func TestVaultClient_LookupToken_RateLimit(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(v.Config, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	client.SetActive(true)
	defer client.Stop()

	waitForConnection(client, t)

	client.setLimit(rate.Limit(1.0))
	testRateLimit(t, 20, client, func(ctx context.Context) error {
		// Lookup ourselves
		_, err := client.LookupToken(ctx, v.Config.Token)
		return err
	})
}

func TestVaultClient_CreateToken_Root(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(v.Config, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	client.SetActive(true)
	defer client.Stop()

	waitForConnection(client, t)

	// Create an allocation that requires a Vault policy
	a := mock.Alloc()
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Vault = &structs.Vault{Policies: []string{"default"}}

	s, err := client.CreateToken(context.Background(), a, task.Name)
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	// Ensure that created secret is a wrapped token
	if s == nil || s.WrapInfo == nil {
		t.Fatalf("Bad secret: %#v", s)
	}

	d, err := time.ParseDuration(vaultTokenCreateTTL)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	if s.WrapInfo.WrappedAccessor == "" {
		t.Fatalf("Bad accessor: %v", s.WrapInfo.WrappedAccessor)
	} else if s.WrapInfo.Token == "" {
		t.Fatalf("Bad token: %v", s.WrapInfo.WrappedAccessor)
	} else if s.WrapInfo.TTL != int(d.Seconds()) {
		t.Fatalf("Bad ttl: %v", s.WrapInfo.WrappedAccessor)
	}
}

func TestVaultClient_CreateToken_Allowlist_Role(t *testing.T) {
	ci.Parallel(t)

	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Set the configs token in a new test role
	v.Config.Token = defaultTestVaultAllowlistRoleAndToken(v, t, 5)

	// Start the client
	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(v.Config, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	client.SetActive(true)
	defer client.Stop()

	waitForConnection(client, t)

	// Create an allocation that requires a Vault policy
	a := mock.Alloc()
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Vault = &structs.Vault{Policies: []string{"default"}}

	s, err := client.CreateToken(context.Background(), a, task.Name)
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	// Ensure that created secret is a wrapped token
	if s == nil || s.WrapInfo == nil {
		t.Fatalf("Bad secret: %#v", s)
	}

	d, err := time.ParseDuration(vaultTokenCreateTTL)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	if s.WrapInfo.WrappedAccessor == "" {
		t.Fatalf("Bad accessor: %v", s.WrapInfo.WrappedAccessor)
	} else if s.WrapInfo.Token == "" {
		t.Fatalf("Bad token: %v", s.WrapInfo.WrappedAccessor)
	} else if s.WrapInfo.TTL != int(d.Seconds()) {
		t.Fatalf("Bad ttl: %v", s.WrapInfo.WrappedAccessor)
	}
}

func TestVaultClient_CreateToken_Root_Target_Role(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Create the test role
	defaultTestVaultAllowlistRoleAndToken(v, t, 5)

	// Target the test role
	v.Config.Role = "test"

	// Start the client
	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(v.Config, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	client.SetActive(true)
	defer client.Stop()

	waitForConnection(client, t)

	// Create an allocation that requires a Vault policy
	a := mock.Alloc()
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Vault = &structs.Vault{Policies: []string{"default"}}

	s, err := client.CreateToken(context.Background(), a, task.Name)
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	// Ensure that created secret is a wrapped token
	if s == nil || s.WrapInfo == nil {
		t.Fatalf("Bad secret: %#v", s)
	}

	d, err := time.ParseDuration(vaultTokenCreateTTL)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	if s.WrapInfo.WrappedAccessor == "" {
		t.Fatalf("Bad accessor: %v", s.WrapInfo.WrappedAccessor)
	} else if s.WrapInfo.Token == "" {
		t.Fatalf("Bad token: %v", s.WrapInfo.WrappedAccessor)
	} else if s.WrapInfo.TTL != int(d.Seconds()) {
		t.Fatalf("Bad ttl: %v", s.WrapInfo.WrappedAccessor)
	}
}

func TestVaultClient_CreateToken_Denylist_Role(t *testing.T) {
	ci.Parallel(t)

	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Need to skip if test is 0.6.4
	version, err := testutil.VaultVersion()
	if err != nil {
		t.Fatalf("failed to determine version: %v", err)
	}

	if strings.Contains(version, "v0.6.4") {
		t.Skipf("Vault has a regression in v0.6.4 that this test hits")
	}

	// Set the configs token in a new test role
	v.Config.Token = defaultTestVaultDenylistRoleAndToken(v, t, 5)
	v.Config.Role = "test"

	// Start the client
	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(v.Config, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	client.SetActive(true)
	defer client.Stop()

	waitForConnection(client, t)

	// Create an allocation that requires a Vault policy
	a := mock.Alloc()
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Vault = &structs.Vault{Policies: []string{"secrets"}}

	s, err := client.CreateToken(context.Background(), a, task.Name)
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	// Ensure that created secret is a wrapped token
	if s == nil || s.WrapInfo == nil {
		t.Fatalf("Bad secret: %#v", s)
	}

	d, err := time.ParseDuration(vaultTokenCreateTTL)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	if s.WrapInfo.WrappedAccessor == "" {
		t.Fatalf("Bad accessor: %v", s.WrapInfo.WrappedAccessor)
	} else if s.WrapInfo.Token == "" {
		t.Fatalf("Bad token: %v", s.WrapInfo.WrappedAccessor)
	} else if s.WrapInfo.TTL != int(d.Seconds()) {
		t.Fatalf("Bad ttl: %v", s.WrapInfo.WrappedAccessor)
	}
}

func TestVaultClient_CreateToken_Role_InvalidToken(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Set the configs token in a new test role
	defaultTestVaultAllowlistRoleAndToken(v, t, 5)
	v.Config.Token = "foo-bar"

	// Start the client
	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(v.Config, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	client.SetActive(true)
	defer client.Stop()

	testutil.WaitForResult(func() (bool, error) {
		established, err := client.ConnectionEstablished()
		if !established {
			return false, fmt.Errorf("Should establish")
		}
		return err != nil, nil
	}, func(err error) {
		t.Fatalf("Connection not established")
	})

	// Create an allocation that requires a Vault policy
	a := mock.Alloc()
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Vault = &structs.Vault{Policies: []string{"default"}}

	_, err = client.CreateToken(context.Background(), a, task.Name)
	if err == nil || !strings.Contains(err.Error(), "failed to establish connection to Vault") {
		t.Fatalf("CreateToken should have failed: %v", err)
	}
}

func TestVaultClient_CreateToken_Role_Unrecoverable(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Set the configs token in a new test role
	v.Config.Token = defaultTestVaultAllowlistRoleAndToken(v, t, 5)

	// Start the client
	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(v.Config, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	client.SetActive(true)
	defer client.Stop()

	waitForConnection(client, t)

	// Create an allocation that requires a Vault policy
	a := mock.Alloc()
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Vault = &structs.Vault{Policies: []string{"unknown_policy"}}

	_, err = client.CreateToken(context.Background(), a, task.Name)
	if err == nil {
		t.Fatalf("CreateToken should have failed: %v", err)
	}

	_, ok := err.(structs.Recoverable)
	if ok {
		t.Fatalf("CreateToken should not be a recoverable error type: %v (%T)", err, err)
	}
}

func TestVaultClient_CreateToken_Prestart(t *testing.T) {
	ci.Parallel(t)
	vconfig := &config.VaultConfig{
		Enabled: pointer.Of(true),
		Token:   uuid.Generate(),
		Addr:    "http://127.0.0.1:0",
	}

	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(vconfig, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	client.SetActive(true)
	defer client.Stop()

	// Create an allocation that requires a Vault policy
	a := mock.Alloc()
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Vault = &structs.Vault{Policies: []string{"default"}}

	_, err = client.CreateToken(context.Background(), a, task.Name)
	if err == nil {
		t.Fatalf("CreateToken should have failed: %v", err)
	}

	if rerr, ok := err.(*structs.RecoverableError); !ok {
		t.Fatalf("Err should have been type recoverable error")
	} else if ok && !rerr.IsRecoverable() {
		t.Fatalf("Err should have been recoverable")
	}
}

func TestVaultClient_MarkForRevocation(t *testing.T) {
	vconfig := &config.VaultConfig{
		Enabled: pointer.Of(true),
		Token:   uuid.Generate(),
		Addr:    "http://127.0.0.1:0",
	}
	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(vconfig, logger, nil, nil)
	require.NoError(t, err)

	client.SetActive(true)
	defer client.Stop()

	// Create some VaultAccessors
	vas := []*structs.VaultAccessor{
		mock.VaultAccessor(),
		mock.VaultAccessor(),
	}

	err = client.MarkForRevocation(vas)
	require.NoError(t, err)

	// Wasn't committed
	require.Len(t, client.revoking, 2)
	require.Equal(t, 2, client.stats().TrackedForRevoke)

}
func TestVaultClient_RevokeTokens_PreEstablishs(t *testing.T) {
	ci.Parallel(t)
	vconfig := &config.VaultConfig{
		Enabled: pointer.Of(true),
		Token:   uuid.Generate(),
		Addr:    "http://127.0.0.1:0",
	}
	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(vconfig, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	client.SetActive(true)
	defer client.Stop()

	// Create some VaultAccessors
	vas := []*structs.VaultAccessor{
		mock.VaultAccessor(),
		mock.VaultAccessor(),
	}

	if err := client.RevokeTokens(context.Background(), vas, false); err != nil {
		t.Fatalf("RevokeTokens failed: %v", err)
	}

	// Wasn't committed
	if len(client.revoking) != 0 {
		t.Fatalf("didn't add to revoke loop")
	}

	if err := client.RevokeTokens(context.Background(), vas, true); err != nil {
		t.Fatalf("RevokeTokens failed: %v", err)
	}

	// Was committed
	if len(client.revoking) != 2 {
		t.Fatalf("didn't add to revoke loop")
	}

	if client.stats().TrackedForRevoke != 2 {
		t.Fatalf("didn't add to revoke loop")
	}
}

// TestVaultClient_RevokeTokens_Failures_TTL asserts that
// the registered TTL doesn't get extended on retries
func TestVaultClient_RevokeTokens_Failures_TTL(t *testing.T) {
	ci.Parallel(t)
	vconfig := &config.VaultConfig{
		Enabled: pointer.Of(true),
		Token:   uuid.Generate(),
		Addr:    "http://127.0.0.1:0",
	}
	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(vconfig, logger, nil, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	client.SetActive(true)
	defer client.Stop()

	// Create some VaultAccessors
	vas := []*structs.VaultAccessor{
		mock.VaultAccessor(),
		mock.VaultAccessor(),
	}

	err = client.RevokeTokens(context.Background(), vas, true)
	require.NoError(t, err)

	// Was committed
	require.Len(t, client.revoking, 2)

	// set TTL
	ttl := time.Now().Add(50 * time.Second)
	client.revoking[vas[0]] = ttl
	client.revoking[vas[1]] = ttl

	// revoke again and ensure that TTL isn't extended
	err = client.RevokeTokens(context.Background(), vas, true)
	require.NoError(t, err)

	require.Len(t, client.revoking, 2)
	expected := map[*structs.VaultAccessor]time.Time{
		vas[0]: ttl,
		vas[1]: ttl,
	}
	require.Equal(t, expected, client.revoking)
}

func TestVaultClient_RevokeTokens_Root(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	purged := 0
	purge := func(accessors []*structs.VaultAccessor) error {
		purged += len(accessors)
		return nil
	}

	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(v.Config, logger, purge, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	client.SetActive(true)
	defer client.Stop()

	waitForConnection(client, t)

	// Create some vault tokens
	auth := v.Client.Auth().Token()
	req := vapi.TokenCreateRequest{
		Policies: []string{"default"},
	}
	t1, err := auth.Create(&req)
	if err != nil {
		t.Fatalf("Failed to create vault token: %v", err)
	}
	if t1 == nil || t1.Auth == nil {
		t.Fatalf("bad secret response: %+v", t1)
	}
	t2, err := auth.Create(&req)
	if err != nil {
		t.Fatalf("Failed to create vault token: %v", err)
	}
	if t2 == nil || t2.Auth == nil {
		t.Fatalf("bad secret response: %+v", t2)
	}

	// Create two VaultAccessors
	vas := []*structs.VaultAccessor{
		{Accessor: t1.Auth.Accessor},
		{Accessor: t2.Auth.Accessor},
	}

	// Issue a token revocation
	if err := client.RevokeTokens(context.Background(), vas, true); err != nil {
		t.Fatalf("RevokeTokens failed: %v", err)
	}

	// Lookup the token and make sure we get an error
	if s, err := auth.Lookup(t1.Auth.ClientToken); err == nil {
		t.Fatalf("Revoked token lookup didn't fail: %+v", s)
	}
	if s, err := auth.Lookup(t2.Auth.ClientToken); err == nil {
		t.Fatalf("Revoked token lookup didn't fail: %+v", s)
	}

	if purged != 2 {
		t.Fatalf("Expected purged 2; got %d", purged)
	}
}

func TestVaultClient_RevokeTokens_Role(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Set the configs token in a new test role
	v.Config.Token = defaultTestVaultAllowlistRoleAndToken(v, t, 5)

	purged := 0
	purge := func(accessors []*structs.VaultAccessor) error {
		purged += len(accessors)
		return nil
	}

	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(v.Config, logger, purge, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	client.SetActive(true)
	defer client.Stop()

	waitForConnection(client, t)

	// Create some vault tokens
	auth := v.Client.Auth().Token()
	req := vapi.TokenCreateRequest{
		Policies: []string{"default"},
	}
	t1, err := auth.Create(&req)
	if err != nil {
		t.Fatalf("Failed to create vault token: %v", err)
	}
	if t1 == nil || t1.Auth == nil {
		t.Fatalf("bad secret response: %+v", t1)
	}
	t2, err := auth.Create(&req)
	if err != nil {
		t.Fatalf("Failed to create vault token: %v", err)
	}
	if t2 == nil || t2.Auth == nil {
		t.Fatalf("bad secret response: %+v", t2)
	}

	// Create two VaultAccessors
	vas := []*structs.VaultAccessor{
		{Accessor: t1.Auth.Accessor},
		{Accessor: t2.Auth.Accessor},
	}

	// Issue a token revocation
	if err := client.RevokeTokens(context.Background(), vas, true); err != nil {
		t.Fatalf("RevokeTokens failed: %v", err)
	}

	// Lookup the token and make sure we get an error
	if purged != 2 {
		t.Fatalf("Expected purged 2; got %d", purged)
	}
	if s, err := auth.Lookup(t1.Auth.ClientToken); err == nil {
		t.Fatalf("Revoked token lookup didn't fail: %+v", s)
	}
	if s, err := auth.Lookup(t2.Auth.ClientToken); err == nil {
		t.Fatalf("Revoked token lookup didn't fail: %+v", s)
	}
}

// TestVaultClient_RevokeTokens_Idempotent asserts that token revocation
// is idempotent, and can cope with cases if token was deleted out of band.
func TestVaultClient_RevokeTokens_Idempotent(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Set the configs token in a new test role
	v.Config.Token = defaultTestVaultAllowlistRoleAndToken(v, t, 5)

	purged := map[string]struct{}{}
	purge := func(accessors []*structs.VaultAccessor) error {
		for _, accessor := range accessors {
			purged[accessor.Accessor] = struct{}{}
		}
		return nil
	}

	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(v.Config, logger, purge, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	client.SetActive(true)
	defer client.Stop()

	waitForConnection(client, t)

	// Create some vault tokens
	auth := v.Client.Auth().Token()
	req := vapi.TokenCreateRequest{
		Policies: []string{"default"},
	}
	t1, err := auth.Create(&req)
	require.NoError(t, err)
	require.NotNil(t, t1)
	require.NotNil(t, t1.Auth)

	t2, err := auth.Create(&req)
	require.NoError(t, err)
	require.NotNil(t, t2)
	require.NotNil(t, t2.Auth)

	t3, err := auth.Create(&req)
	require.NoError(t, err)
	require.NotNil(t, t3)
	require.NotNil(t, t3.Auth)

	// revoke t3 out of band
	err = auth.RevokeAccessor(t3.Auth.Accessor)
	require.NoError(t, err)

	// Create two VaultAccessors
	vas := []*structs.VaultAccessor{
		{Accessor: t1.Auth.Accessor},
		{Accessor: t2.Auth.Accessor},
		{Accessor: t3.Auth.Accessor},
	}

	// Issue a token revocation
	err = client.RevokeTokens(context.Background(), vas, true)
	require.NoError(t, err)
	require.Empty(t, client.revoking)

	// revoke token again
	err = client.RevokeTokens(context.Background(), vas, true)
	require.NoError(t, err)
	require.Empty(t, client.revoking)

	// Lookup the token and make sure we get an error
	require.Len(t, purged, 3)
	require.Contains(t, purged, t1.Auth.Accessor)
	require.Contains(t, purged, t2.Auth.Accessor)
	require.Contains(t, purged, t3.Auth.Accessor)
	s, err := auth.Lookup(t1.Auth.ClientToken)
	require.Errorf(t, err, "failed to purge token: %v", s)
	s, err = auth.Lookup(t2.Auth.ClientToken)
	require.Errorf(t, err, "failed to purge token: %v", s)
}

// TestVaultClient_RevokeDaemon_Bounded asserts that token revocation
// batches are bounded in size.
func TestVaultClient_RevokeDaemon_Bounded(t *testing.T) {
	ci.Parallel(t)
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Set the configs token in a new test role
	v.Config.Token = defaultTestVaultAllowlistRoleAndToken(v, t, 5)

	// Disable client until we can change settings for testing
	conf := v.Config.Copy()
	conf.Enabled = pointer.Of(false)

	const (
		batchSize = 100
		batches   = 3
	)
	resultCh := make(chan error, batches)
	var totalPurges int64

	// Purge function asserts batches are always < batchSize
	purge := func(vas []*structs.VaultAccessor) error {
		if len(vas) > batchSize {
			resultCh <- fmt.Errorf("too many Vault accessors in batch: %d > %d", len(vas), batchSize)
		} else {
			resultCh <- nil
		}
		atomic.AddInt64(&totalPurges, int64(len(vas)))

		return nil
	}

	logger := testlog.HCLogger(t)
	client, err := NewVaultClient(conf, logger, purge, nil)
	require.NoError(t, err)

	// Override settings for testing and then enable client
	client.maxRevokeBatchSize = batchSize
	client.revocationIntv = 3 * time.Millisecond
	conf = v.Config.Copy()
	conf.Enabled = pointer.Of(true)
	require.NoError(t, client.SetConfig(conf))

	client.SetActive(true)
	defer client.Stop()

	waitForConnection(client, t)

	// Create more tokens in Nomad than can fit in a batch; they don't need
	// to exist in Vault.
	accessors := make([]*structs.VaultAccessor, batchSize*batches)
	for i := 0; i < len(accessors); i++ {
		accessors[i] = &structs.VaultAccessor{Accessor: "abcd"}
	}

	// Mark for revocation
	require.NoError(t, client.MarkForRevocation(accessors))

	// Wait for tokens to be revoked
	for i := 0; i < batches; i++ {
		select {
		case err := <-resultCh:
			require.NoError(t, err)
		case <-time.After(10 * time.Second):
			// 10 seconds should be plenty long to process 3
			// batches at a 3ms tick interval!
			t.Errorf("timed out processing %d batches. %d/%d complete in 10s",
				batches, i, batches)
		}
	}

	require.Equal(t, int64(len(accessors)), atomic.LoadInt64(&totalPurges))
}

func waitForConnection(v *vaultClient, t *testing.T) {
	testutil.WaitForResult(func() (bool, error) {
		return v.ConnectionEstablished()
	}, func(err error) {
		t.Fatalf("Connection not established")
	})
}

func TestVaultClient_nextBackoff(t *testing.T) {
	ci.Parallel(t)

	simpleCases := []struct {
		name        string
		initBackoff float64

		// define range of acceptable backoff values accounting for random factor
		rangeMin float64
		rangeMax float64
	}{
		{"simple case", 7.0, 8.7, 17.60},
		{"too low", 2.0, 5.0, 10.0},
		{"too large", 100, 30.0, 60.0},
	}

	for _, c := range simpleCases {
		t.Run(c.name, func(t *testing.T) {
			b := nextBackoff(c.initBackoff, time.Now().Add(10*time.Hour))
			if !(c.rangeMin <= b && b <= c.rangeMax) {
				t.Fatalf("Expected backoff within [%v, %v] but found %v", c.rangeMin, c.rangeMax, b)
			}
		})
	}

	// some edge cases
	t.Run("close to expiry", func(t *testing.T) {
		b := nextBackoff(20, time.Now().Add(1100*time.Millisecond))
		if b != 5.0 {
			t.Fatalf("Expected backoff is 5 but found %v", b)
		}
	})

	t.Run("past expiry", func(t *testing.T) {
		b := nextBackoff(20, time.Now().Add(-1100*time.Millisecond))
		if !(60 <= b && b <= 120) {
			t.Fatalf("Expected backoff within [%v, %v] but found %v", 60, 120, b)
		}
	})
}

func testRateLimit(t *testing.T, count int, client *vaultClient, fn func(context.Context) error) {
	// Spin up many requests. These should block
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cancels := 0
	unblock := make(chan struct{})
	for i := 0; i < count; i++ {
		go func() {
			err := fn(ctx)
			if err != nil {
				if err == context.Canceled {
					cancels += 1
					return
				}
				t.Errorf("request failed: %v", err)
				return
			}

			// Cancel the context
			close(unblock)
		}()
	}

	select {
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout")
	case <-unblock:
		cancel()
	}

	desired := count - 1
	testutil.WaitForResult(func() (bool, error) {
		if desired-cancels > 2 {
			return false, fmt.Errorf("Incorrect number of cancels; got %d; want %d", cancels, desired)
		}

		return true, nil
	}, func(err error) {
		t.Fatal(err)
	})
}
