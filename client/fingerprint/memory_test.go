// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"

	"github.com/stretchr/testify/require"
)

func TestMemoryFingerprint(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)

	f := NewMemoryFingerprint(testlog.HCLogger(t))
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	request := &FingerprintRequest{Config: &config.Config{}, Node: node}
	var response FingerprintResponse
	err := f.Fingerprint(request, &response)
	require.NoError(err)

	assertNodeAttributeContains(t, response.Attributes, "memory.totalbytes")
	require.NotNil(response.Resources, "expected response Resources to not be nil")
	require.NotZero(response.Resources.MemoryMB, "expected memory to be non-zero")
	require.NotNil(response.NodeResources, "expected response NodeResources to not be nil")
	require.NotZero(response.NodeResources.Memory.MemoryMB, "expected memory to be non-zero")
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
	require := require.New(t)
	require.NotNil(response.Resources)
	require.EqualValues(response.Resources.MemoryMB, memoryMB)
	require.NotNil(response.NodeResources)
	require.EqualValues(response.NodeResources.Memory.MemoryMB, memoryMB)
}
