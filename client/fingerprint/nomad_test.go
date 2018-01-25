package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/version"
)

func TestNomadFingerprint(t *testing.T) {
	f := NewNomadFingerprint(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	v := "foo"
	r := "123"
	c := &config.Config{
		Version: &version.VersionInfo{
			Revision: r,
			Version:  v,
		},
	}

	request := &cstructs.FingerprintRequest{Config: c, Node: node}
	response := &cstructs.FingerprintResponse{
		Attributes: make(map[string]string, 0),
		Links:      make(map[string]string, 0),
		Resources:  &structs.Resources{},
	}

	err := f.Fingerprint(request, response)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(response.Attributes) == 0 {
		t.Fatalf("should apply")
	}

	if response.Attributes["nomad.version"] != v {
		t.Fatalf("incorrect version")
	}

	if response.Attributes["nomad.revision"] != r {
		t.Fatalf("incorrect revision")
	}
}
