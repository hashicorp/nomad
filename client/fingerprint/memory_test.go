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
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	assertNodeAttributeContains(t, response.Attributes, "memory.totalbytes")

	if response.NodeResources == nil {
		t.Fatalf("response noderesources should not be nil")
	}

	if response.NodeResources.Memory.MemoryMB == 0 {
		t.Fatalf("Expected node.Resources.MemoryMB to be non-zero")
	}
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
	require := require.New(t)
	require.NotNil(response.NodeResources)
	require.EqualValues(response.NodeResources.Memory.MemoryMB, memoryMB)
}
