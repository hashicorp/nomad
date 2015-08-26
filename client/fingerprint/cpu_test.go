package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestCPUFingerprint(t *testing.T) {
	f := NewCPUFingerprint(testLogger())
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

	// CPU info
	if node.Attributes["cpu.numcores"] == "" {
		t.Fatalf("Missing Num Cores")
	}
	if node.Attributes["cpu.modelname"] == "" {
		t.Fatalf("Missing Model Name")
	}

}
