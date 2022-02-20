package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"

	"github.com/stretchr/testify/require"
)

func TestMemoryFingerprint(t *testing.T) {

	f := NewMemoryFingerprint(testlog.HCLogger(t))
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	request := &FingerprintRequest{Config: &config.Config{}, Node: node}
	var response FingerprintResponse
	err := f.Fingerprint(request, &response)
	require.NoError(t, err)

	assertNodeAttributeContains(t, response.Attributes, "memory.totalbytes")
	require.NotNil(t, response.Resources, "expected response Resources to not be nil")
	require.NotZero(t, response.Resources.MemoryMB, "expected memory to be non-zero")
	require.NotNil(t, response.NodeResources, "expected response NodeResources to not be nil")
	require.NotZero(t, response.NodeResources.Memory.MemoryMB, "expected memory to be non-zero")
}

func TestMemoryFingerprint_Override(t *testing.T) {
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
	require.NotNil(t, response.Resources)
	require.EqualValues(t, response.Resources.MemoryMB, memoryMB)
	require.NotNil(t, response.NodeResources)
	require.EqualValues(t, response.NodeResources.Memory.MemoryMB, memoryMB)
}
