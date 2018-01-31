package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestCPUFingerprint(t *testing.T) {
	f := NewCPUFingerprint(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	request := &cstructs.FingerprintRequest{Config: &config.Config{}, Node: node}
	var response cstructs.FingerprintResponse
	err := f.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !response.Detected {
		t.Fatalf("expected response to be applicable")
	}

	// CPU info
	attributes := response.Attributes
	if attributes == nil {
		t.Fatalf("expected attributes to be initialized")
	}
	if attributes["cpu.numcores"] == "" {
		t.Fatalf("Missing Num Cores")
	}
	if attributes["cpu.modelname"] == "" {
		t.Fatalf("Missing Model Name")
	}

	if attributes["cpu.frequency"] == "" {
		t.Fatalf("Missing CPU Frequency")
	}
	if attributes["cpu.totalcompute"] == "" {
		t.Fatalf("Missing CPU Total Compute")
	}

	if response.Resources == nil || response.Resources.CPU == 0 {
		t.Fatalf("Expected to find CPU Resources")
	}
}

// TestCPUFingerprint_OverrideCompute asserts that setting cpu_total_compute in
// the client config overrides the detected CPU freq (if any).
func TestCPUFingerprint_OverrideCompute(t *testing.T) {
	f := NewCPUFingerprint(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	cfg := &config.Config{}
	var originalCPU int

	{
		request := &cstructs.FingerprintRequest{Config: cfg, Node: node}
		var response cstructs.FingerprintResponse
		err := f.Fingerprint(request, &response)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if !response.Detected {
			t.Fatalf("expected response to be applicable")
		}

		if response.Resources.CPU == 0 {
			t.Fatalf("expected fingerprint of cpu of but found 0")
		}

		originalCPU = response.Resources.CPU
	}

	{
		// Override it with a setting
		cfg.CpuCompute = originalCPU + 123

		// Make sure the Fingerprinter applies the override to the node resources
		request := &cstructs.FingerprintRequest{Config: cfg, Node: node}
		var response cstructs.FingerprintResponse
		err := f.Fingerprint(request, &response)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if response.Resources.CPU != cfg.CpuCompute {
			t.Fatalf("expected override cpu of %d but found %d", cfg.CpuCompute, response.Resources.CPU)
		}
	}
}
