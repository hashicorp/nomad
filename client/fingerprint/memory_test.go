package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestMemoryFingerprint(t *testing.T) {
	f := NewMemoryFingerprint(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	request := &cstructs.FingerprintRequest{Config: &config.Config{}, Node: node}
	var response cstructs.FingerprintResponse
	err := f.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	assertNodeAttributeContains(t, response.GetAttributes(), "memory.totalbytes")

	res := response.GetResources()
	if res.MemoryMB == 0 {
		t.Errorf("Expected node.Resources.MemoryMB to be non-zero")
	}

}
