package fingerprint

import (
	"strconv"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
)

func TestStorageFingerprint(t *testing.T) {
	fp := NewStorageFingerprint(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	assertFingerprintOK(t, fp, node)

	assertNodeAttributeContains(t, node, "storage.volume")
	assertNodeAttributeContains(t, node, "storage.bytestotal")
	assertNodeAttributeContains(t, node, "storage.bytesfree")

	total, err := strconv.ParseInt(node.Attributes["storage.bytestotal"], 10, 64)
	if err != nil {
		t.Fatalf("Failed to parse storage.bytestotal: %s", err)
	}
	free, err := strconv.ParseInt(node.Attributes["storage.bytesfree"], 10, 64)
	if err != nil {
		t.Fatalf("Failed to parse storage.bytesfree: %s", err)
	}

	if free > total {
		t.Fatalf("storage.bytesfree %d is larger than storage.bytestotal %d", free, total)
	}

	if node.Resources == nil {
		t.Fatalf("Node Resources was nil")
	}
	if node.Resources.DiskMB == 0 {
		t.Errorf("Expected node.Resources.DiskMB to be non-zero")
	}
}
