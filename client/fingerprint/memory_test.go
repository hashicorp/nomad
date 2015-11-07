package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestMemoryFingerprint(t *testing.T) {
	f := NewMemoryFingerprint(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	ok, err := f.Fingerprint(&config.Config{}, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("should apply")
	}

	assertNodeAttributeContains(t, node, "memory.totalbytes")

	if node.Resources == nil {
		t.Fatalf("Node Resources was nil")
	}
	if node.Resources.MemoryMB == 0 {
		t.Errorf("Expected node.Resources.MemoryMB to be non-zero")
	}

}
