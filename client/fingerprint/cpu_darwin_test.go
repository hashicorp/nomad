// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build darwin && arm64 && cgo

package fingerprint

import (
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

	attributes := response.Attributes
	must.NotNil(t, attributes)
	must.MapContainsKey(t, attributes, "cpu.modelname")
	must.MapContainsKey(t, attributes, "cpu.numcores.power")
	must.MapContainsKey(t, attributes, "cpu.numcores.efficiency")
	must.MapContainsKey(t, attributes, "cpu.frequency.power")
	must.MapContainsKey(t, attributes, "cpu.frequency.efficiency")
	must.MapContainsKey(t, attributes, "cpu.totalcompute")
	must.Positive(t, response.Resources.CPU)
	must.Positive(t, response.NodeResources.Cpu.CpuShares)
	must.Positive(t, response.NodeResources.Cpu.SharesPerCore())
	must.SliceEmpty(t, response.NodeResources.Cpu.ReservableCpuCores)

	// not included for mixed core types (that we can detect)
	must.MapNotContainsKey(t, attributes, "cpu.numcores")
	must.MapNotContainsKey(t, attributes, "cpu.frequency")
}
