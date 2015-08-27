package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
)

func TestStorageFingerprint(t *testing.T) {
	f := NewStorageFingerprint(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	assertFingerprintOK(t, f, node)
	assertNodeAttributeContains(t, node, "storage.volume")
	assertNodeAttributeContains(t, node, "storage.bytestotal")
	assertNodeAttributeContains(t, node, "storage.bytesavailable")
}
