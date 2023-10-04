// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package vaultclient_test

import (
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/useragent"
	"github.com/hashicorp/nomad/testutil"
	vaultapi "github.com/hashicorp/vault/api"
	vaultconsts "github.com/hashicorp/vault/sdk/helper/consts"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestVaultClient_TokenRenewals(t *testing.T) {
	ci.Parallel(t)

	v := testutil.NewTestVault(t)
	defer v.Stop()

	logger := testlog.HCLogger(t)
	v.Config.ConnectionRetryIntv = 100 * time.Millisecond
	v.Config.TaskTokenTTL = "4s"
	c, err := vaultclient.NewVaultClient(v.Config, logger, nil)
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
		c.Vault.SetToken(v.Config.Token)

		if err := c.Vault.SetAddress(v.Config.Addr); err != nil {
			t.Fatal(err)
		}

		secret, err := c.Vault.Auth().Token().Create(tcr)
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

	c.Lock.Lock()
	length := c.Heap.Length()
	c.Lock.Unlock()
	if length != num {
		t.Fatalf("bad: Heap length: expected: %d, actual: %d", num, length)
	}

	time.Sleep(time.Duration(testutil.TestMultiplier()) * time.Second)

	for i := 0; i < num; i++ {
		if err := c.StopRenewToken(tokens[i]); err != nil {
			must.NoError(t, err)
		}
	}

	c.Lock.Lock()
	length = c.Heap.Length()
	c.Lock.Unlock()
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

	conf := config.DefaultConfig()
	conf.VaultConfig.Enabled = &tr
	conf.VaultConfig.Token = "testvaulttoken"
	conf.VaultConfig.Namespace = testNs
	c, err := vaultclient.NewVaultClient(conf.VaultConfig, logger, nil)
	must.NoError(t, err)
	must.Eq(t, testNs, c.Vault.Headers().Get(vaultconsts.NamespaceHeaderName))
}

func TestVaultClient_Heap(t *testing.T) {
	ci.Parallel(t)

	tr := true
	conf := config.DefaultConfig()
	conf.VaultConfig.Enabled = &tr
	conf.VaultConfig.Token = "testvaulttoken"
	conf.VaultConfig.TaskTokenTTL = "10s"

	logger := testlog.HCLogger(t)
	c, err := vaultclient.NewVaultClient(conf.VaultConfig, logger, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("failed to create vault Vault")
	}

	now := time.Now()

	renewalReq1 := &vaultclient.RenewalRequest{
		ErrCh:     make(chan error, 1),
		ID:        "id1",
		Increment: 10,
	}
	if err := c.Heap.Push(renewalReq1, now.Add(50*time.Second)); err != nil {
		t.Fatal(err)
	}
	if !c.IsTracked("id1") {
		t.Fatalf("id1 should have been tracked")
	}

	renewalReq2 := &vaultclient.RenewalRequest{
		ErrCh:     make(chan error, 1),
		ID:        "id2",
		Increment: 10,
	}
	if err := c.Heap.Push(renewalReq2, now.Add(40*time.Second)); err != nil {
		t.Fatal(err)
	}
	if !c.IsTracked("id2") {
		t.Fatalf("id2 should have been tracked")
	}

	renewalReq3 := &vaultclient.RenewalRequest{
		ErrCh:     make(chan error, 1),
		ID:        "id3",
		Increment: 10,
	}
	if err := c.Heap.Push(renewalReq3, now.Add(60*time.Second)); err != nil {
		t.Fatal(err)
	}
	if !c.IsTracked("id3") {
		t.Fatalf("id3 should have been tracked")
	}

	// Reading elements should yield id2, id1 and id3 in order
	req, _ := c.NextRenewal()
	if req != renewalReq2 {
		t.Fatalf("bad: expected: %#v, actual: %#v", renewalReq2, req)
	}
	if err := c.Heap.Update(req, now.Add(70*time.Second)); err != nil {
		t.Fatal(err)
	}

	req, _ = c.NextRenewal()
	if req != renewalReq1 {
		t.Fatalf("bad: expected: %#v, actual: %#v", renewalReq1, req)
	}
	if err := c.Heap.Update(req, now.Add(80*time.Second)); err != nil {
		t.Fatal(err)
	}

	req, _ = c.NextRenewal()
	if req != renewalReq3 {
		t.Fatalf("bad: expected: %#v, actual: %#v", renewalReq3, req)
	}
	if err := c.Heap.Update(req, now.Add(90*time.Second)); err != nil {
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

	if c.IsTracked("id1") {
		t.Fatalf("id1 should not have been tracked")
	}

	if c.IsTracked("id1") {
		t.Fatalf("id1 should not have been tracked")
	}

	if c.IsTracked("id1") {
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
	c, err := vaultclient.NewVaultClient(v.Config, logger, nil)
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

	c.Vault.SetToken(v.Config.Token)

	if err := c.Vault.SetAddress(v.Config.Addr); err != nil {
		t.Fatal(err)
	}

	secret, err := c.Vault.Auth().Token().Create(tcr)
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
	c, err := vaultclient.NewVaultClient(v.Config, logger, nil)
	if err != nil {
		t.Fatalf("failed to build vault Vault: %v", err)
	}

	c.Start()
	defer c.Stop()

	// Sleep a little while to ensure that the renewal loop is active
	time.Sleep(time.Duration(testutil.TestMultiplier()) * time.Second)

	c.Vault.SetToken(v.Config.Token)

	if err := c.Vault.SetAddress(v.Config.Addr); err != nil {
		t.Fatal(err)
	}

	_, err = c.RenewToken(c.Vault.Token(), 10)
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

	must.Eq(t, 39*time.Second, vaultclient.RenewalTime(highRoller, 60))
	must.Eq(t, 20*time.Second, vaultclient.RenewalTime(lowRoller, 60))

	must.Eq(t, 309*time.Second, vaultclient.RenewalTime(highRoller, 600))
	must.Eq(t, 290*time.Second, vaultclient.RenewalTime(lowRoller, 600))

	const days3 = 60 * 60 * 24 * 3
	must.Eq(t, (days3/2+9)*time.Second, vaultclient.RenewalTime(highRoller, days3))
	must.Eq(t, (days3/2-10)*time.Second, vaultclient.RenewalTime(lowRoller, days3))
}

// TestVaultClient_RenewalTime_Short asserts that for leases under 1m the renewal
// time is lease/2.
func TestVaultClient_RenewalTime_Short(t *testing.T) {
	ci.Parallel(t)

	dice := func(int) int {
		require.Fail(t, "dice should not have been called")
		panic("unreachable")
	}

	must.Eq(t, 29*time.Second, vaultclient.RenewalTime(dice, 58))
	must.Eq(t, 15*time.Second, vaultclient.RenewalTime(dice, 30))
	must.Eq(t, 1*time.Second, vaultclient.RenewalTime(dice, 2))
}

func TestVaultClient_SetUserAgent(t *testing.T) {
	ci.Parallel(t)

	conf := config.DefaultConfig()
	conf.VaultConfig.Enabled = pointer.Of(true)
	logger := testlog.HCLogger(t)
	c, err := vaultclient.NewVaultClient(conf.VaultConfig, logger, nil)
	must.NoError(t, err)

	ua := c.Vault.Headers().Get("User-Agent")
	must.Eq(t, useragent.String(), ua)
}
