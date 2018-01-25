package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestArchFingerprint(t *testing.T) {
	f := NewArchFingerprint(testLogger())
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

	if response.Attributes["cpu.arch"] == "" {
		t.Fatalf("missing arch")
	}
}
