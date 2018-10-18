package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/version"
)

func TestNomadFingerprint(t *testing.T) {
	f := NewNomadFingerprint(testlog.HCLogger(t))

	v := "foo"
	r := "123"
	h := "8.8.8.8:4646"
	c := &config.Config{
		Version: &version.VersionInfo{
			Revision: r,
			Version:  v,
		},
	}
	node := &structs.Node{
		Attributes: make(map[string]string),
		HTTPAddr:   h,
	}

	request := &cstructs.FingerprintRequest{Config: c, Node: node}
	var response cstructs.FingerprintResponse
	err := f.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !response.Detected {
		t.Fatalf("expected response to be applicable")
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

	if response.Attributes["nomad.advertise.address"] != h {
		t.Fatalf("incorrect advertise address")
	}
}
