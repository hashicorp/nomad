package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestCPUFingerprint(t *testing.T) {
	require := require.New(t)
	f := NewCPUFingerprint(testlog.HCLogger(t))
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	request := &FingerprintRequest{Config: &config.Config{}, Node: node}
	var response FingerprintResponse
	err := f.Fingerprint(request, &response)
	require.NoError(err)

	require.True(response.Detected, "expected response to be detected")

	// CPU info
	attributes := response.Attributes
	require.NotNil(attributes, "expected attributes to initialized")

	require.NotEqual(attributes["cpu.numcores"], "", "Missing numcores")
	require.NotEqual(attributes["cpu.modelname"], "", "Missing modelname")
	require.NotEqual(attributes["cpu.frequency"], "", "Missing CPU Frequency")
	require.NotEqual(attributes["cpu.totalcompute"], "", "Missing CPU Total Compute")

	if response.NodeResources == nil || response.NodeResources.Cpu.CpuShares == 0 {
		t.Fatalf("Expected to find CPU Resources")
	}
}

// TestCPUFingerprint_OverrideCompute asserts that setting cpu_total_compute in
// the client config overrides the detected CPU freq (if any).
func TestCPUFingerprint_OverrideCompute(t *testing.T) {
	require := require.New(t)
	f := NewCPUFingerprint(testlog.HCLogger(t))
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	cfg := &config.Config{}
	var originalCPU int64

	{
		request := &FingerprintRequest{Config: cfg, Node: node}
		var response FingerprintResponse
		err := f.Fingerprint(request, &response)
		require.NoError(err)
		require.True(response.Detected, "expected response to be detected")

		require.NotEqual(response.NodeResources.Cpu.CpuShares, 0, "expected fingerprint of cpu of but found 0")

		originalCPU = response.NodeResources.Cpu.CpuShares
	}

	{
		// Override it with a setting
		cfg.CpuCompute = int(originalCPU + 123)

		// Make sure the Fingerprinter applies the override to the node resources
		request := &FingerprintRequest{Config: cfg, Node: node}
		var response FingerprintResponse
		err := f.Fingerprint(request, &response)
		require.NoError(err)

		if response.NodeResources.Cpu.CpuShares != int64(cfg.CpuCompute) {
			t.Fatalf("expected override cpu of %d but found %d", cfg.CpuCompute, response.NodeResources.Cpu.CpuShares)
		}
	}
}
