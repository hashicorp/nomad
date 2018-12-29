package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func TestVaultFingerprint(t *testing.T) {
	tv := testutil.NewTestVault(t)
	defer tv.Stop()

	fp := NewVaultFingerprint(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	config := config.DefaultConfig()
	config.VaultConfig = tv.Config

	ok, err := fp.Fingerprint(config, node)
	if err != nil {
		t.Fatalf("Failed to fingerprint: %s", err)
	}
	if !ok {
		t.Fatalf("Failed to apply node attributes")
	}

	assertNodeAttributeContains(t, node, "vault.accessible")
	assertNodeAttributeContains(t, node, "vault.version")
	assertNodeAttributeContains(t, node, "vault.cluster_id")
	assertNodeAttributeContains(t, node, "vault.cluster_name")
}
