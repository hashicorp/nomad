package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestNomadFingerprint(t *testing.T) {
	f := NewNomadFingerprint(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	v := "foo"
	r := "123"
	c := &config.Config{
		Version:  v,
		Revision: r,
	}
	ok, err := f.Fingerprint(c, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("should apply")
	}
	if node.Attributes["nomad.version"] != v {
		t.Fatalf("incorrect version")
	}
	if node.Attributes["nomad.revision"] != r {
		t.Fatalf("incorrect revision")
	}
}
