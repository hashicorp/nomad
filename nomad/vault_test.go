package nomad

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/hashicorp/nomad/helper"
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

// defaultTestVaultWhitelistRoleAndToken creates a test Vault role and returns a token
// created in that role
func defaultTestVaultWhitelistRoleAndToken(v *testutil.TestVault, t *testing.T, rolePeriod int) string {
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

// defaultTestVaultBlacklistRoleAndToken creates a test Vault role using
// disallowed_policies and returns a token created in that role
func defaultTestVaultBlacklistRoleAndToken(v *testutil.TestVault, t *testing.T, rolePeriod int) string {
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
	t.Parallel()
	conf := &config.VaultConfig{}
	logger := log.New(os.Stderr, "", log.LstdFlags)

	// Should be no error since Vault is not enabled
	_, err := NewVaultClient(nil, logger, nil)
	if err == nil || !strings.Contains(err.Error(), "valid") {
		t.Fatalf("expected config error: %v", err)
	}

	tr := true
	conf.Enabled = &tr
	_, err = NewVaultClient(conf, logger, nil)
	if err == nil || !strings.Contains(err.Error(), "token must be set") {
		t.Fatalf("Expected token unset error: %v", err)
	}

	conf.Token = "123"
	_, err = NewVaultClient(conf, logger, nil)
	if err == nil || !strings.Contains(err.Error(), "address must be set") {
		t.Fatalf("Expected address unset error: %v", err)
	}
}

// started separately.
// Test that the Vault Client can establish a connection even if it is started
// before Vault is available.
func TestVaultClient_EstablishConnection(t *testing.T) {
	t.Parallel()
	for i := 10; i >= 0; i-- {
		v := testutil.NewTestVaultDelayed(t)
		logger := log.New(os.Stderr, "", log.LstdFlags)
		v.Config.ConnectionRetryIntv = 100 * time.Millisecond
		client, err := NewVaultClient(v.Config, logger, nil)
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
	t.Parallel()
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
		"explicit_max_ttl": 10,
	}
	v.Config.Token = testVaultRoleAndToken(v, t, vaultPolicies, data, nil)

	logger := log.New(os.Stderr, "", log.LstdFlags)
	v.Config.ConnectionRetryIntv = 100 * time.Millisecond
	client, err := NewVaultClient(v.Config, logger, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	defer client.Stop()

	// Wait for an error
	var conn bool
	var connErr error
	testutil.WaitForResult(func() (bool, error) {
		conn, connErr = client.ConnectionEstablished()
		if conn {
			return false, fmt.Errorf("Should not connect")
		}

		if connErr == nil {
			return false, fmt.Errorf("expect an error")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("bad: %v", err)
	})

	errStr := connErr.Error()
	if !strings.Contains(errStr, "not allow orphans") {
		t.Fatalf("Expect orphan error")
	}
	if !strings.Contains(errStr, "explicit max ttl") {
		t.Fatalf("Expect explicit max ttl error")
	}
}

func TestVaultClient_ValidateRole_NonExistant(t *testing.T) {
	t.Parallel()
	v := testutil.NewTestVault(t)
	defer v.Stop()

	v.Config.Token = defaultTestVaultWhitelistRoleAndToken(v, t, 5)
	v.Config.Token = v.RootToken
	logger := log.New(os.Stderr, "", log.LstdFlags)
	v.Config.ConnectionRetryIntv = 100 * time.Millisecond
	v.Config.Role = "test-nonexistant"
	client, err := NewVaultClient(v.Config, logger, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	defer client.Stop()

	// Wait for an error
	var conn bool
	var connErr error
	testutil.WaitForResult(func() (bool, error) {
		conn, connErr = client.ConnectionEstablished()
		if conn {
			return false, fmt.Errorf("Should not connect")
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
		t.Fatalf("Expect orphan error")
	}
}

func TestVaultClient_ValidateToken(t *testing.T) {
	t.Parallel()
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

	logger := log.New(os.Stderr, "", log.LstdFlags)
	v.Config.ConnectionRetryIntv = 100 * time.Millisecond
	client, err := NewVaultClient(v.Config, logger, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	defer client.Stop()

	// Wait for an error
	var conn bool
	var connErr error
	testutil.WaitForResult(func() (bool, error) {
		conn, connErr = client.ConnectionEstablished()
		if conn {
			return false, fmt.Errorf("Should not connect")
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
		t.Fatalf("Expect orphan error")
	}
	if !strings.Contains(errStr, fmt.Sprintf(vaultRoleLookupPath, "test")) {
		t.Fatalf("Expect explicit max ttl error")
	}
	if !strings.Contains(errStr, "token must have one of the following") {
		t.Fatalf("Expect explicit max ttl error")
	}
}

func TestVaultClient_SetActive(t *testing.T) {
	t.Parallel()
	v := testutil.NewTestVault(t)
	defer v.Stop()

	logger := log.New(os.Stderr, "", log.LstdFlags)
	client, err := NewVaultClient(v.Config, logger, nil)
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
	t.Parallel()
	v := testutil.NewTestVault(t)
	defer v.Stop()

	v2 := testutil.NewTestVault(t)
	defer v2.Stop()

	// Set the configs token in a new test role
	v2.Config.Token = defaultTestVaultWhitelistRoleAndToken(v2, t, 20)

	logger := log.New(os.Stderr, "", log.LstdFlags)
	client, err := NewVaultClient(v.Config, logger, nil)
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
}

// Test that we can disable vault
func TestVaultClient_SetConfig_Disable(t *testing.T) {
	t.Parallel()
	v := testutil.NewTestVault(t)
	defer v.Stop()

	logger := log.New(os.Stderr, "", log.LstdFlags)
	client, err := NewVaultClient(v.Config, logger, nil)
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
	t.Parallel()
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Set the configs token in a new test role
	v.Config.Token = defaultTestVaultWhitelistRoleAndToken(v, t, 5)

	// Start the client
	logger := log.New(os.Stderr, "", log.LstdFlags)
	client, err := NewVaultClient(v.Config, logger, nil)
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
	t.Parallel()
	tr := true
	conf := &config.VaultConfig{
		Enabled: &tr,
		Addr:    "http://foobar:12345",
		Token:   structs.GenerateUUID(),
	}

	// Enable vault but use a bad address so it never establishes a conn
	logger := log.New(os.Stderr, "", log.LstdFlags)
	client, err := NewVaultClient(conf, logger, nil)
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
	t.Parallel()
	v := testutil.NewTestVault(t)
	defer v.Stop()

	logger := log.New(os.Stderr, "", log.LstdFlags)
	client, err := NewVaultClient(v.Config, logger, nil)
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

	policies, err := PoliciesFrom(s)
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

	policies, err = PoliciesFrom(s)
	if err != nil {
		t.Fatalf("failed to parse policies: %v", err)
	}

	if !reflect.DeepEqual(policies, expected) {
		t.Fatalf("Unexpected policies; got %v; want %v", policies, expected)
	}
}

func TestVaultClient_LookupToken_Role(t *testing.T) {
	t.Parallel()
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Set the configs token in a new test role
	v.Config.Token = defaultTestVaultWhitelistRoleAndToken(v, t, 5)

	logger := log.New(os.Stderr, "", log.LstdFlags)
	client, err := NewVaultClient(v.Config, logger, nil)
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

	policies, err := PoliciesFrom(s)
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

	policies, err = PoliciesFrom(s)
	if err != nil {
		t.Fatalf("failed to parse policies: %v", err)
	}

	if !reflect.DeepEqual(policies, expected) {
		t.Fatalf("Unexpected policies; got %v; want %v", policies, expected)
	}
}

func TestVaultClient_LookupToken_RateLimit(t *testing.T) {
	t.Parallel()
	v := testutil.NewTestVault(t)
	defer v.Stop()

	logger := log.New(os.Stderr, "", log.LstdFlags)
	client, err := NewVaultClient(v.Config, logger, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	client.SetActive(true)
	defer client.Stop()
	client.setLimit(rate.Limit(1.0))

	waitForConnection(client, t)

	// Spin up many requests. These should block
	ctx, cancel := context.WithCancel(context.Background())

	cancels := 0
	numRequests := 10
	unblock := make(chan struct{})
	for i := 0; i < numRequests; i++ {
		go func() {
			// Lookup ourselves
			_, err := client.LookupToken(ctx, v.Config.Token)
			if err != nil {
				if err == context.Canceled {
					cancels += 1
					return
				}
				t.Fatalf("self lookup failed: %v", err)
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

	desired := numRequests - 1
	testutil.WaitForResult(func() (bool, error) {
		if cancels != desired {
			return false, fmt.Errorf("Incorrect number of cancels; got %d; want %d", cancels, desired)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("Connection not established")
	})
}

func TestVaultClient_CreateToken_Root(t *testing.T) {
	t.Parallel()
	v := testutil.NewTestVault(t)
	defer v.Stop()

	logger := log.New(os.Stderr, "", log.LstdFlags)
	client, err := NewVaultClient(v.Config, logger, nil)
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

func TestVaultClient_CreateToken_Whitelist_Role(t *testing.T) {
	t.Parallel()
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Set the configs token in a new test role
	v.Config.Token = defaultTestVaultWhitelistRoleAndToken(v, t, 5)

	// Start the client
	logger := log.New(os.Stderr, "", log.LstdFlags)
	client, err := NewVaultClient(v.Config, logger, nil)
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
	t.Parallel()
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Create the test role
	defaultTestVaultWhitelistRoleAndToken(v, t, 5)

	// Target the test role
	v.Config.Role = "test"

	// Start the client
	logger := log.New(os.Stderr, "", log.LstdFlags)
	client, err := NewVaultClient(v.Config, logger, nil)
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

func TestVaultClient_CreateToken_Blacklist_Role(t *testing.T) {
	t.Parallel()
	// Need to skip if test is 0.6.4
	version, err := testutil.VaultVersion()
	if err != nil {
		t.Fatalf("failed to determine version: %v", err)
	}

	if strings.Contains(version, "v0.6.4") {
		t.Skipf("Vault has a regression in v0.6.4 that this test hits")
	}

	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Set the configs token in a new test role
	v.Config.Token = defaultTestVaultBlacklistRoleAndToken(v, t, 5)
	v.Config.Role = "test"

	// Start the client
	logger := log.New(os.Stderr, "", log.LstdFlags)
	client, err := NewVaultClient(v.Config, logger, nil)
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
	t.Parallel()
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Set the configs token in a new test role
	defaultTestVaultWhitelistRoleAndToken(v, t, 5)
	v.Config.Token = "foo-bar"

	// Start the client
	logger := log.New(os.Stderr, "", log.LstdFlags)
	client, err := NewVaultClient(v.Config, logger, nil)
	if err != nil {
		t.Fatalf("failed to build vault client: %v", err)
	}
	client.SetActive(true)
	defer client.Stop()

	testutil.WaitForResult(func() (bool, error) {
		established, err := client.ConnectionEstablished()
		if established {
			return false, fmt.Errorf("Shouldn't establish")
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
	if err == nil || !strings.Contains(err.Error(), "Connection to Vault failed") {
		t.Fatalf("CreateToken should have failed: %v", err)
	}
}

func TestVaultClient_CreateToken_Role_Unrecoverable(t *testing.T) {
	t.Parallel()
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Set the configs token in a new test role
	v.Config.Token = defaultTestVaultWhitelistRoleAndToken(v, t, 5)

	// Start the client
	logger := log.New(os.Stderr, "", log.LstdFlags)
	client, err := NewVaultClient(v.Config, logger, nil)
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
	t.Parallel()
	vconfig := &config.VaultConfig{
		Enabled: helper.BoolToPtr(true),
		Token:   structs.GenerateUUID(),
		Addr:    "http://127.0.0.1:0",
	}

	logger := log.New(os.Stderr, "", log.LstdFlags)
	client, err := NewVaultClient(vconfig, logger, nil)
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

func TestVaultClient_RevokeTokens_PreEstablishs(t *testing.T) {
	t.Parallel()
	vconfig := &config.VaultConfig{
		Enabled: helper.BoolToPtr(true),
		Token:   structs.GenerateUUID(),
		Addr:    "http://127.0.0.1:0",
	}
	logger := log.New(os.Stderr, "", log.LstdFlags)
	client, err := NewVaultClient(vconfig, logger, nil)
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

	if client.Stats().TrackedForRevoke != 2 {
		t.Fatalf("didn't add to revoke loop")
	}
}

func TestVaultClient_RevokeTokens_Root(t *testing.T) {
	t.Parallel()
	v := testutil.NewTestVault(t)
	defer v.Stop()

	purged := 0
	purge := func(accessors []*structs.VaultAccessor) error {
		purged += len(accessors)
		return nil
	}

	logger := log.New(os.Stderr, "", log.LstdFlags)
	client, err := NewVaultClient(v.Config, logger, purge)
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
		&structs.VaultAccessor{Accessor: t1.Auth.Accessor},
		&structs.VaultAccessor{Accessor: t2.Auth.Accessor},
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
	t.Parallel()
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Set the configs token in a new test role
	v.Config.Token = defaultTestVaultWhitelistRoleAndToken(v, t, 5)

	purged := 0
	purge := func(accessors []*structs.VaultAccessor) error {
		purged += len(accessors)
		return nil
	}

	logger := log.New(os.Stderr, "", log.LstdFlags)
	client, err := NewVaultClient(v.Config, logger, purge)
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
		&structs.VaultAccessor{Accessor: t1.Auth.Accessor},
		&structs.VaultAccessor{Accessor: t2.Auth.Accessor},
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

func waitForConnection(v *vaultClient, t *testing.T) {
	testutil.WaitForResult(func() (bool, error) {
		return v.ConnectionEstablished()
	}, func(err error) {
		t.Fatalf("Connection not established")
	})
}
