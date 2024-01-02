// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"strconv"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestStorageFingerprint(t *testing.T) {
	ci.Parallel(t)

	fp := NewStorageFingerprint(testlog.HCLogger(t))
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	response := assertFingerprintOK(t, fp, node)

	if !response.Detected {
		t.Fatalf("expected response to be applicable")
	}

	assertNodeAttributeContains(t, response.Attributes, "unique.storage.volume")
	assertNodeAttributeContains(t, response.Attributes, "unique.storage.bytestotal")
	assertNodeAttributeContains(t, response.Attributes, "unique.storage.bytesfree")

	total, err := strconv.ParseInt(response.Attributes["unique.storage.bytestotal"], 10, 64)
	if err != nil {
		t.Fatalf("Failed to parse unique.storage.bytestotal: %s", err)
	}
	free, err := strconv.ParseInt(response.Attributes["unique.storage.bytesfree"], 10, 64)
	if err != nil {
		t.Fatalf("Failed to parse unique.storage.bytesfree: %s", err)
	}

	if free > total {
		t.Fatalf("unique.storage.bytesfree %d is larger than unique.storage.bytestotal %d", free, total)
	}

	// COMPAT(0.10): Remove in 0.10
	if response.Resources == nil {
		t.Fatalf("Node Resources was nil")
	}
	if response.Resources.DiskMB == 0 {
		t.Errorf("Expected node.Resources.DiskMB to be non-zero")
	}

	if response.NodeResources == nil || response.NodeResources.Disk.DiskMB == 0 {
		t.Errorf("Expected node.Resources.DiskMB to be non-zero")
	}
}
