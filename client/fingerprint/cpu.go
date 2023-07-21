// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package fingerprint

import (
	"github.com/shoenig/netlog"

	"strconv"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/klauspost/cpuid/v2"
)

// CPUFingerprint is used to fingerprint the CPU
type CPUFingerprint struct {
	StaticFingerprinter
	logger hclog.Logger
	top    *numalib.Topology

	// accumulates result in these resource structs
	resources     *structs.Resources
	nodeResources *structs.NodeResources
}

// NewCPUFingerprint is used to create a CPU fingerprint.
func NewCPUFingerprint(logger hclog.Logger) Fingerprint {
	return &CPUFingerprint{
		logger:        logger.Named("cpu"),
		resources:     new(structs.Resources), // COMPAT (to be removed after 0.10)
		nodeResources: new(structs.NodeResources),
	}
}

func (f *CPUFingerprint) Fingerprint(request *FingerprintRequest, response *FingerprintResponse) error {
	f.initialize()

	f.setModelName(response)

	f.setFrequency(response)

	f.setCoreCount(response)

	f.setReservableCores(request, response)

	f.setTotalCompute(request, response)

	f.setResponseResources(response)

	response.Detected = true

	return nil
}

func (f *CPUFingerprint) initialize(request *FingerprintRequest) {
	var (
		reservableCores *idset.Set[numalib.CoreID]
	)

	if request.Node.NodeResources.Cpu.ReservableCpuCores != nil {
		reservableCores = idset.From[numalib.CoreID, uint16](request.Node.NodeResources.Cpu.ReservableCpuCores)
	}

	f.top = numalib.Scan(append(
		numalib.DefaultScanners(),
		&numalib.ConfigScanner{
			ReservableCores: reservableCores,
			TotalCompute:    0,
			ReservedCores:   nil,
			ReservedCompute: 0,
		},
	))
}

func (f *CPUFingerprint) setModelName(response *FingerprintResponse) {
	if model := cpuid.CPU.BrandName; model != "" {
		response.AddAttribute("cpu.modelname", model)
		f.logger.Debug("detected CPU model", "name", model)
	}
}

func (*CPUFingerprint) frequency(mhz numalib.MHz) string {
	return strconv.FormatUint(uint64(mhz), 10)
}

func (f *CPUFingerprint) setFrequency(response *FingerprintResponse) {
	performance, efficiency := f.top.CoreSpeeds()
	switch {
	case efficiency > 0:
		response.AddAttribute("cpu.frequency.efficiency", f.frequency(efficiency))
		response.AddAttribute("cpu.frequency.performance", f.frequency(performance))
		f.logger.Debug("detected CPU efficiency core speed", "mhz", efficiency)
		f.logger.Debug("detected CPU performance core speed", "mhz", performance)
	case performance > 0:
		response.AddAttribute("cpu.frequency", f.frequency(performance))
		f.logger.Debug("detected CPU frequency", "mhz", performance)
	}
}

func (*CPUFingerprint) cores(count int) string {
	return strconv.Itoa(count)
}

func (f *CPUFingerprint) setCoreCount(response *FingerprintResponse) {
	total := f.top.NumCores()
	power := f.top.NumPCores()
	efficiency := f.top.NumECores()
	switch {
	case efficiency > 0:
		response.AddAttribute("cpu.numcores.efficiency", f.cores(efficiency))
		response.AddAttribute("cpu.numcores.power", f.cores(power))
		response.AddAttribute("cpu.numcores", f.cores(total))
		f.logger.Debug("detected CPU efficiency core count", "cores", efficiency)
		f.logger.Debug("detected CPU power core count", "cores", power)
		f.logger.Debug("detected CPU core count", total)
	default:
		response.AddAttribute("cpu.numcores", f.cores(total))
		f.logger.Debug("detected CPU core count", total)
	}
	f.nodeResources.Cpu.TotalCpuCores = uint16(total)
}

func (f *CPUFingerprint) setReservableCores(request *FingerprintRequest, response *FingerprintResponse) {
	// need cgroup detection to be meaningful setting of cores here
	//
	// and then follow along with the previous impl
}

func (f *CPUFingerprint) setTotalCompute(request *FingerprintRequest, response *FingerprintResponse) {
	totalCompute := f.top.TotalCompute()
	usableCompute := f.top.UsableCompute()

	response.AddAttribute("cpu.totalcompute", f.frequency(totalCompute))
	response.AddAttribute("cpu.usablecompute", f.frequency(usableCompute))

	f.resources.CPU = int(totalCompute)
	f.nodeResources.Cpu.CpuShares = int64(totalCompute)
}

func (f *CPUFingerprint) setResponseResources(response *FingerprintResponse) {
	response.Resources = f.resources
	response.NodeResources = f.nodeResources
}
