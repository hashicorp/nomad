// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !darwin || !arm64 || !cgo

package fingerprint

import (
	"strconv"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestCPUFingerprint_Classic(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	// create the fingerprinter
	f := NewCPUFingerprint(logger)
	node := &structs.Node{Attributes: make(map[string]string)}

	request := &FingerprintRequest{Config: &config.Config{}, Node: node}
	var response FingerprintResponse

	// run the fingerprinter
	err := f.Fingerprint(request, &response)
	must.NoError(t, err)

	must.True(t, response.Detected)
	attributes := response.Attributes
	must.NotNil(t, attributes)
	must.MapContainsKey(t, attributes, "cpu.numcores")
	must.MapContainsKey(t, attributes, "cpu.modelname")
	must.MapContainsKey(t, attributes, "cpu.frequency")
	must.MapContainsKey(t, attributes, "cpu.totalcompute")
	must.Positive(t, response.Resources.CPU)
	must.Positive(t, response.NodeResources.Cpu.CpuShares)
	must.Positive(t, response.NodeResources.Cpu.SharesPerCore())
	must.SliceNotEmpty(t, response.NodeResources.Cpu.ReservableCpuCores)

	// asymetric core detection currently only works with apple silicon
	must.MapNotContainsKey(t, attributes, "cpu.numcores.power")
	must.MapNotContainsKey(t, attributes, "cpu.numcores.efficiency")
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
		ReservableCores: []numalib.CoreID{0, 1, 2},
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
