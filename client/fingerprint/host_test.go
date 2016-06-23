package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestHostFingerprint(t *testing.T) {
	f := NewHostFingerprint(testLogger())
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

	// Host info
	for _, key := range []string{"os.name", "os.version", "unique.hostname", "kernel.name"} {
		assertNodeAttributeContains(t, node, key)
	}
}
