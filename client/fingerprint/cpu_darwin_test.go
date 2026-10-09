// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build darwin && arm64 && cgo

package fingerprint

import (
	"strconv"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestCPUFingerprint_AppleSilicon(t *testing.T) {
	ci.Parallel(t)

	f := NewCPUFingerprint(testlog.HCLogger(t))
	node := &structs.Node{Attributes: make(map[string]string)}

	request := &FingerprintRequest{Config: new(config.Config), Node: node}
	var response FingerprintResponse

	err := f.Fingerprint(request, &response)
	must.NoError(t, err)

	must.True(t, response.Detected)

	attrs := response.Attributes
	must.NotNil(t, attrs)
	must.MapContainsKey(t, attrs, "cpu.modelname")
	must.MapContainsKey(t, attrs, "cpu.numcores")
	must.MapContainsKey(t, attrs, "cpu.numcores.performance")
	must.MapContainsKey(t, attrs, "cpu.numcores.efficiency")
	must.MapContainsKey(t, attrs, "cpu.frequency.performance")
	must.MapContainsKey(t, attrs, "cpu.frequency.efficiency")
	must.MapContainsKey(t, attrs, "cpu.totalcompute")
	must.Positive(t, response.NodeResources.Cpu.CpuShares)
	must.Positive(t, response.NodeResources.Cpu.TotalCpuCores)
	must.SliceEmpty(t, response.NodeResources.Cpu.ReservableCpuCores)

	nproc, err := strconv.Atoi(attrs["cpu.numcores"])
	must.NoError(t, err)
	must.Greater(t, 0, nproc)
	must.Eq(t, nproc, response.NodeResources.Processors.Topology.NumCores())

	nump, err := strconv.Atoi(attrs["cpu.numcores.performance"])
	must.NoError(t, err)
	must.Greater(t, 0, nump)
	must.Eq(t, nump, response.NodeResources.Processors.Topology.NumPCores())

	nume, err := strconv.Atoi(attrs["cpu.numcores.efficiency"])
	must.NoError(t, err)
	must.Greater(t, 0, nume)
	must.Eq(t, nume, response.NodeResources.Processors.Topology.NumECores())

	must.Eq(t, nproc, nump+nume)

	// not included for mixed core types (that we can detect)
	must.MapNotContainsKey(t, attrs, "cpu.frequency")
}
