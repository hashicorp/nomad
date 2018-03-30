package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestSwapFingerprint(t *testing.T) {
	f := NewSwapFingerprint(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	request := &cstructs.FingerprintRequest{Config: &config.Config{}, Node: node}

	var response cstructs.FingerprintResponse

	err := f.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	assertNodeAttributeContains(t, response.Attributes, "swap.totalbytes")

	if node.Resources == nil {
		t.Fatalf("Node Resources was nil")
	}
	if node.Resources.SwapMB == 0 {
		t.Errorf("Expected node.Resources.SwapMB to be non-zero")
	}

}
