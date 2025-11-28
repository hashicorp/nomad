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

	_, err := strconv.ParseInt(response.Attributes["unique.storage.bytestotal"], 10, 64)
	if err != nil {
		t.Fatalf("Failed to parse unique.storage.bytestotal: %s", err)
	}

	if response.NodeResources == nil || response.NodeResources.Disk.DiskMB == 0 {
		t.Errorf("Expected node.NodeResources.DiskMB.DiskMB to be non-zero")
	}
}
