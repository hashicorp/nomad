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

	if !response.Detected {
		t.Fatalf("expected response to be applicable")
	}

	assertNodeAttributeContains(t, response.Attributes, "vault.accessible")
	assertNodeAttributeContains(t, response.Attributes, "vault.version")
	assertNodeAttributeContains(t, response.Attributes, "vault.cluster_id")
	assertNodeAttributeContains(t, response.Attributes, "vault.cluster_name")
}
