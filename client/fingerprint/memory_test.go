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
	response := &cstructs.FingerprintResponse{
		Attributes: make(map[string]string, 0),
		Links:      make(map[string]string, 0),
		Resources:  &structs.Resources{},
	}
	err := f.Fingerprint(request, response)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	assertNodeAttributeContains(t, response.Attributes, "memory.totalbytes")

	if response.Resources == nil {
		t.Fatalf("Node Resources was nil")
	}
	if response.Resources.MemoryMB == 0 {
		t.Errorf("Expected node.Resources.MemoryMB to be non-zero")
	}

}
