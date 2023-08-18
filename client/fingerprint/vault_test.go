// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func TestVaultFingerprint(t *testing.T) {
	ci.Parallel(t)

	tv := testutil.NewTestVault(t)
	defer tv.Stop()

	fp := NewVaultFingerprint(testlog.HCLogger(t))
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	p, period := fp.Periodic()
	if !p {
		t.Fatalf("expected fingerprint to be periodic")
	}
	if period != (15 * time.Second) {
		t.Fatalf("expected period to be 15s but found: %s", period)
	}

	conf := config.DefaultConfig()
	conf.VaultConfig = tv.Config

	request := &FingerprintRequest{Config: conf, Node: node}
	var response FingerprintResponse
	err := fp.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("Failed to fingerprint: %s", err)
	}

	if !response.Detected {
		t.Fatalf("expected response to be applicable")
	}

	assertNodeAttributeContains(t, response.Attributes, "vault.accessible")
	assertNodeAttributeContains(t, response.Attributes, "vault.version")
	assertNodeAttributeContains(t, response.Attributes, "vault.cluster_id")
	assertNodeAttributeContains(t, response.Attributes, "vault.cluster_name")

	// Period should be longer after initial discovery
	p, period = fp.Periodic()
	if !p {
		t.Fatalf("expected fingerprint to be periodic")
	}
	if period < (30*time.Second) || period > (2*time.Minute) {
		t.Fatalf("expected period to be between 30s and 2m but found: %s", period)
	}

	// Stop Vault to simulate it being unavailable
	tv.Stop()

	// Reset the nextCheck time for testing purposes, or we won't pick up the
	// change until the next period, up to 2min from now
	vfp := fp.(*VaultFingerprint)
	vfp.states["default"].nextCheck = time.Now()

	err = fp.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("Failed to fingerprint: %s", err)
	}

	if !response.Detected {
		t.Fatalf("should still show as detected")
	}

	assertNodeAttributeContains(t, response.Attributes, "vault.accessible")
	assertNodeAttributeContains(t, response.Attributes, "vault.version")
	assertNodeAttributeContains(t, response.Attributes, "vault.cluster_id")
	assertNodeAttributeContains(t, response.Attributes, "vault.cluster_name")

	// Period should be original once trying to discover Vault is available again
	p, period = fp.Periodic()
	if !p {
		t.Fatalf("expected fingerprint to be periodic")
	}
	if period != (15 * time.Second) {
		t.Fatalf("expected period to be 15s but found: %s", period)
	}
}
