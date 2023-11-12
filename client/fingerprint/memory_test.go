// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestMemoryFingerprint(t *testing.T) {
	ci.Parallel(t)

	f := NewMemoryFingerprint(testlog.HCLogger(t))
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	request := &FingerprintRequest{Config: &config.Config{}, Node: node}
	var response FingerprintResponse
	err := f.Fingerprint(request, &response)
	must.NoError(t, err)

	assertNodeAttributeContains(t, response.Attributes, "memory.totalbytes")
	must.Positive(t, response.NodeResources.Memory.MemoryMB)
}

func TestMemoryFingerprint_Override(t *testing.T) {
	ci.Parallel(t)

	f := NewMemoryFingerprint(testlog.HCLogger(t))
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	memoryMB := 15000
	request := &FingerprintRequest{Config: &config.Config{MemoryMB: memoryMB}, Node: node}
	var response FingerprintResponse
	err := f.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	assertNodeAttributeContains(t, response.Attributes, "memory.totalbytes")
	must.Eq(t, response.NodeResources.Memory.MemoryMB, int64(memoryMB))
}
