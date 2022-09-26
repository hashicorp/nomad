package fingerprint

import (
	"strconv"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestCPUFingerprint(t *testing.T) {
	ci.Parallel(t)

	f := NewCPUFingerprint(testlog.HCLogger(t))
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	request := &FingerprintRequest{Config: &config.Config{}, Node: node}
	var response FingerprintResponse
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

	// COMPAT(0.10): Remove in 0.10
	if response.Resources == nil || response.Resources.CPU == 0 {
		t.Fatalf("Expected to find CPU Resources")
	}

	if response.NodeResources == nil || response.NodeResources.Cpu.CpuShares == 0 {
		t.Fatalf("Expected to find CPU Resources")
	}
}

// TestCPUFingerprint_OverrideCompute asserts that setting cpu_total_compute in
// the client config overrides the detected CPU freq (if any).
func TestCPUFingerprint_OverrideCompute(t *testing.T) {
	ci.Parallel(t)

	f := NewCPUFingerprint(testlog.HCLogger(t))
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	cfg := &config.Config{
		ReservableCores: []uint16{0, 1, 2},
	}
	var originalCPU int

	{
		request := &FingerprintRequest{Config: cfg, Node: node}
		var response FingerprintResponse
		err := f.Fingerprint(request, &response)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if !response.Detected {
			t.Fatalf("expected response to be applicable")
		}

		if attr := response.Attributes["cpu.reservablecores"]; attr != "3" {
			t.Fatalf("expected cpu.reservablecores == 3 but found %s", attr)
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
		request := &FingerprintRequest{Config: cfg, Node: node}
		var response FingerprintResponse
		err := f.Fingerprint(request, &response)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// COMPAT(0.10): Remove in 0.10
		if response.Resources.CPU != cfg.CpuCompute {
			t.Fatalf("expected override cpu of %d but found %d", cfg.CpuCompute, response.Resources.CPU)
		}
		if response.NodeResources.Cpu.CpuShares != int64(cfg.CpuCompute) {
			t.Fatalf("expected override cpu of %d but found %d", cfg.CpuCompute, response.NodeResources.Cpu.CpuShares)
		}
		if response.Attributes["cpu.totalcompute"] != strconv.Itoa(cfg.CpuCompute) {
			t.Fatalf("expected override cpu.totalcompute of %d but found %s", cfg.CpuCompute, response.Attributes["cpu.totalcompute"])
		}

		if attr := response.Attributes["cpu.reservablecores"]; attr != "3" {
			t.Fatalf("expected cpu.reservablecores == 3 but found %s", attr)
		}
	}
}
