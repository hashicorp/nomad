// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !darwin || !arm64 || !cgo

package fingerprint

import (
	"strconv"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"github.com/hashicorp/nomad/client/testutil"
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
	must.MapContainsKey(t, attributes, "cpu.totalcompute")
	must.Positive(t, response.NodeResources.Processors.Topology.UsableCompute())
	must.Positive(t, response.NodeResources.Processors.Topology.NumCores())
	must.NotEmpty(t, response.NodeResources.Processors.Topology.UsableCores())

	_, frequencyPresent := attributes["cpu.frequency"]
	_, performancePresent := attributes["cpu.frequency.performance"]
	_, efficiencyPresent := attributes["cpu.frequency.efficiency"]
	ok := frequencyPresent || (performancePresent && efficiencyPresent)
	must.True(t, ok, must.Sprint("expected cpu.frequency or cpu.frequency.performance and cpu.frequency.efficiency"))
}

// TestCPUFingerprint_OverrideCompute asserts that setting cpu_total_compute in
// the client config overrides the detected CPU freq (if any).
func TestCPUFingerprint_OverrideCompute(t *testing.T) {
	ci.Parallel(t)
	testutil.MinimumCores(t, 4)

	f := NewCPUFingerprint(testlog.HCLogger(t))
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	cfg := &config.Config{
		ReservableCores: []hw.CoreID{0, 1, 2},
	}
	var originalCompute int

	{
		request := &FingerprintRequest{Config: cfg, Node: node}
		var response FingerprintResponse
		err := f.Fingerprint(request, &response)
		must.NoError(t, err)

		must.True(t, response.Detected)
		must.Eq(t, "3", response.Attributes["cpu.reservablecores"], must.Sprint("override of cpu.reservablecores is incorrect"))
		originalCompute := response.NodeResources.Processors.Topology.UsableCompute()
		must.Positive(t, originalCompute)
	}

	{
		// Override it with a setting
		cfg.CpuCompute = originalCompute + 123

		// Make sure the Fingerprinter applies the override to the node resources
		request := &FingerprintRequest{Config: cfg, Node: node}
		var response FingerprintResponse
		err := f.Fingerprint(request, &response)
		must.NoError(t, err)

		// topology struct
		must.Eq(t, hw.MHz(cfg.CpuCompute), response.NodeResources.Processors.Topology.UsableCompute(), must.Sprint("cpu override did not take affect"))

		// legacy struct
		must.Eq(t, int64(cfg.CpuCompute), response.NodeResources.Cpu.CpuShares, must.Sprint("cpu override did not take affect"))
		must.Eq(t, strconv.Itoa(cfg.CpuCompute), response.Attributes["cpu.totalcompute"], must.Sprint("cpu override did not take affect"))
		must.Eq(t, "3", response.Attributes["cpu.reservablecores"], must.Sprint("cpu override did not take affect"))
	}
}
