package fingerprint

import (
	"os"
	"strconv"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestStorageFingerprint(t *testing.T) {
	fp := NewStorageFingerprint(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
		Resources:  &structs.Resources{},
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get test working directory: %s", err)
	}
	cfg := &config.Config{
		AllocDir: cwd,
	}

	ok, err := fp.Fingerprint(cfg, node)
	if err != nil {
		t.Fatalf("Failed to fingerprint: `%s`", err)
	}
	if !ok {
		t.Fatal("Failed to apply node attributes")
	}

	assertNodeAttributeContains(t, node, "storage.volume")
	assertNodeAttributeContains(t, node, "storage.bytestotal")
	assertNodeAttributeContains(t, node, "storage.bytesavailable")

	total, err := strconv.ParseInt(node.Attributes["storage.bytestotal"], 10, 64)
	if err != nil {
		t.Fatalf("Failed to parse storage.bytestotal: %s", err)
	}
	available, err := strconv.ParseInt(node.Attributes["storage.bytesavailable"], 10, 64)
	if err != nil {
		t.Fatalf("Failed to parse storage.bytesavailable: %s", err)
	}

	if available > total {
		t.Errorf("storage.bytesavailable %d is larger than storage.bytestotal %d", available, total)
	}

	if node.Resources.DiskMB == 0 {
		t.Errorf("Expected node.Resources.DiskMB to be non-zero")
	}
}
