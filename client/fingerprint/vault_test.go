package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
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

	conf := config.DefaultConfig()
	conf.VaultConfig = tv.Config

	request := &cstructs.FingerprintRequest{Config: conf, Node: node}
	var response cstructs.FingerprintResponse
	err := fp.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("Failed to fingerprint: %s", err)
	}

	attributes := response.GetAttributes()
	assertNodeAttributeContains(t, attributes, "vault.accessible")
	assertNodeAttributeContains(t, attributes, "vault.version")
	assertNodeAttributeContains(t, attributes, "vault.cluster_id")
	assertNodeAttributeContains(t, attributes, "vault.cluster_name")
}
