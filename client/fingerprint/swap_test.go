package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestSwapFingerprint(t *testing.T) {
	f := NewSwapFingerprint(testLogger())
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

	assertNodeAttributeContains(t, node, "swap.totalbytes")

	if node.Resources == nil {
		t.Fatalf("Node Resources was nil")
	}
	if node.Resources.SwapMB == 0 {
		t.Errorf("Expected node.Resources.SwapMB to be non-zero")
	}

}
