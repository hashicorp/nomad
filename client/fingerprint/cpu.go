// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package fingerprint

import (
	"github.com/shoenig/netlog"

	"fmt"
	"strconv"

	"github.com/hashicorp/go-hclog"
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
	netlog.Cyan("CPUF.Fingerprint", "rp", fmt.Sprintf("%#v", response))

	f.setResponseResources(response)

	response.Detected = true

	return nil
}

func (f *CPUFingerprint) initialize() {
	f.top = numalib.ScanSysfs()

	// todo: refactor into a chooser (?) pattern
}

func (f *CPUFingerprint) setModelName(response *FingerprintResponse) {
	if model := cpuid.CPU.BrandName; model != "" {
		response.AddAttribute("cpu.modelname", model)
		f.logger.Debug("detected CPU model", "name", model)
	}
}

func (*CPUFingerprint) frequency(hz numalib.Hz) string {
	return strconv.FormatUint(uint64(hz.MHz()), 10)
}

func (f *CPUFingerprint) setFrequency(response *FingerprintResponse) {
	power, efficiency := f.top.CoreSpeeds()
	switch {
	case efficiency > 0:
		response.AddAttribute("cpu.frequency.efficiency", f.frequency(efficiency))
		response.AddAttribute("cpu.frequency.power", f.frequency(power))
		f.logger.Debug("detected CPU efficiency core speed", "mhz", efficiency)
		f.logger.Debug("detected CPU power core speed", "mhz", power)
	case power > 0:
		response.AddAttribute("cpu.frequency", f.frequency(power))
		f.logger.Debug("detected CPU frequency", "mhz", power)
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
	f.nodeResources.Cpu.TotalCpuCores = uint16(power + efficiency)
}

func (f *CPUFingerprint) setReservableCores(request *FingerprintRequest, response *FingerprintResponse) {
	// reservable := request.Config.ReservableCores
	// if len(reservable) > 0 {
	// 	f.logger.Debug("reservable cores set by config", "cpuset", reservable)
	// } else {
	// 	// SETH set reservable cores attribute as detected
	// }

	// response.AddAttribute("cpu.reservablecores", strconv.Itoa(len(reservable)))
	// f.nodeResources.Cpu.ReservableCpuCores = reservable
}

func (f *CPUFingerprint) setTotalCompute(request *FingerprintRequest, response *FingerprintResponse) {
	// var ticks uint64
	// switch {
	// case request.Config.CpuCompute > 0:
	// 	ticks = uint64(request.Config.CpuCompute)
	// case stats.TotalTicksAvailable() > 0:
	// 	ticks = stats.TotalTicksAvailable()
	// default:
	// 	ticks = defaultCPUTicks
	// }
	// response.AddAttribute("cpu.totalcompute", fmt.Sprintf("%d", ticks))
	// f.resources.CPU = int(ticks)
	// f.nodeResources.Cpu.CpuShares = int64(ticks)
}

func (f *CPUFingerprint) setResponseResources(response *FingerprintResponse) {
	response.Resources = f.resources
	response.NodeResources = f.nodeResources
}
