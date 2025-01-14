// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
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
	must.False(t, p)
	must.Zero(t, period)

	conf := config.DefaultConfig()
	conf.VaultConfigs[structs.VaultDefaultCluster] = tv.Config

	request := &FingerprintRequest{Config: conf, Node: node}
	var response1 FingerprintResponse
	err := fp.Fingerprint(request, &response1)
	must.NoError(t, err)
	must.True(t, response1.Detected)

	assertNodeAttributeEquals(t, response1.Attributes, "vault.accessible", "true")
	assertNodeAttributeContains(t, response1.Attributes, "vault.version")
	assertNodeAttributeContains(t, response1.Attributes, "vault.cluster_id")
	assertNodeAttributeContains(t, response1.Attributes, "vault.cluster_name")

	// Stop Vault to simulate it being unavailable
	tv.Stop()

	// Not detected this time
	var response2 FingerprintResponse
	err = fp.Fingerprint(request, &response2)
	must.NoError(t, err)
	must.False(t, response2.Detected)
}
