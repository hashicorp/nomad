// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/klauspost/cpuid/v2"
)

// CPUFingerprint is used to fingerprint the CPU
type CPUFingerprint struct {
	StaticFingerprinter
	logger hclog.Logger
}

// NewCPUFingerprint is used to create a CPU fingerprint.
func NewCPUFingerprint(logger hclog.Logger) Fingerprint {
	return &CPUFingerprint{
		logger: logger.Named("cpu"),
	}
}

func (f *CPUFingerprint) Fingerprint(request *FingerprintRequest, response *FingerprintResponse) error {
	// if this was a reload, wipe what was there before
	if request.Node != nil {
		for k := range request.Node.Attributes {
			if k != "cpu.arch" && // set by arch fingerprinter
				(strings.HasPrefix(k, "cpu.") || strings.HasPrefix(k, "numa.node")) {
				response.RemoveAttribute(k)
			}
		}
	}

	top := f.initialize(request)
	f.setModelName(response)
	f.setFrequency(response, top)
	f.setCoreCount(response, top)
	f.setReservableCores(response, top)
	f.setTotalCompute(response, top)
	f.setNUMA(response, top)
	f.setResponseResources(response, top)

	// indicate we successfully detected the system cpu / memory configuration
	response.Detected = true

	// pass the topology back up to the client
	response.UpdateInitialResult = func(ir *InitialResult) {
		ir.Topology = top
	}

	return nil
}

func (*CPUFingerprint) reservedCompute(request *FingerprintRequest) structs.NodeReservedCpuResources {
	switch {
	case request.Config.Node == nil:
		return structs.NodeReservedCpuResources{}
	case request.Config.Node.ReservedResources == nil:
		return structs.NodeReservedCpuResources{}
	default:
		return request.Config.Node.ReservedResources.Cpu
	}
}

func (f *CPUFingerprint) initialize(request *FingerprintRequest) *numalib.Topology {
	var (
		reservableCores *idset.Set[hw.CoreID]
		totalCompute    = request.Config.CpuCompute
		reservedCompute = f.reservedCompute(request)
		reservedCores   = idset.From[hw.CoreID](reservedCompute.ReservedCpuCores)
	)

	if rc := request.Config.ReservableCores; rc != nil {
		reservableCores = idset.From[hw.CoreID](rc)
	}

	return numalib.Scan(append(
		numalib.PlatformScanners(request.Config.CpuDisableDmidecode),
		&numalib.ConfigScanner{
			ReservableCores: reservableCores,
			ReservedCores:   reservedCores,
			TotalCompute:    hw.MHz(totalCompute),
			ReservedCompute: hw.MHz(reservedCompute.CpuShares),
		},
	))
}

func (f *CPUFingerprint) setModelName(response *FingerprintResponse) {
	if model := cpuid.CPU.BrandName; model != "" {
		response.AddAttribute("cpu.modelname", model)
		f.logger.Debug("detected CPU model", "name", model)
	}
}

func (*CPUFingerprint) frequency(mhz hw.MHz) string {
	return strconv.FormatUint(uint64(mhz), 10)
}

func (f *CPUFingerprint) setFrequency(response *FingerprintResponse, top *numalib.Topology) {
	performance, efficiency := top.CoreSpeeds()
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

func (*CPUFingerprint) nodes(count int) string {
	return strconv.Itoa(count)
}

func (f *CPUFingerprint) setCoreCount(response *FingerprintResponse, top *numalib.Topology) {
	total := top.NumCores()
	performance := top.NumPCores()
	efficiency := top.NumECores()
	switch {
	case efficiency > 0:
		response.AddAttribute("cpu.numcores.efficiency", f.cores(efficiency))
		response.AddAttribute("cpu.numcores.performance", f.cores(performance))
		response.AddAttribute("cpu.numcores", f.cores(total))
		f.logger.Debug("detected CPU efficiency core count", "cores", efficiency)
		f.logger.Debug("detected CPU performance core count", "cores", performance)
		f.logger.Debug("detected CPU core count", "cores", total)
	default:
		response.AddAttribute("cpu.numcores", f.cores(total))
		f.logger.Debug("detected CPU core count", "cores", total)
	}
}

func (f *CPUFingerprint) setReservableCores(response *FingerprintResponse, top *numalib.Topology) {
	switch runtime.GOOS {
	case "linux":
		// topology has already reduced to the intersection of usable cores
		usable := top.UsableCores()
		response.AddAttribute("cpu.reservablecores", f.cores(usable.Size()))
	default:
		response.AddAttribute("cpu.reservablecores", "0")
	}
}

func (f *CPUFingerprint) setTotalCompute(response *FingerprintResponse, top *numalib.Topology) {
	totalCompute := top.TotalCompute()
	usableCompute := top.UsableCompute()

	if totalCompute == 0 {
		f.logger.Error("cpu.totalcompute is zero, CPU fingerprinting likely failed", "cpu.totalcompute", totalCompute)
	}

	response.AddAttribute("cpu.totalcompute", f.frequency(totalCompute))
	response.AddAttribute("cpu.usablecompute", f.frequency(usableCompute))
}

func (f *CPUFingerprint) setNUMA(response *FingerprintResponse, top *numalib.Topology) {
	if !top.SupportsNUMA() {
		return
	}

	nodes := top.GetNodes()
	response.AddAttribute("numa.node.count", f.nodes(nodes.Size()))

	nodes.ForEach(func(id hw.NodeID) error {
		key := fmt.Sprintf("numa.node%d.cores", id)
		cores := top.NodeCores(id)
		response.AddAttribute(key, cores.String())
		return nil
	})
}

func (f *CPUFingerprint) setResponseResources(response *FingerprintResponse, top *numalib.Topology) {
	nodeResources := new(structs.NodeResources)
	nodeResources.Processors = structs.NewNodeProcessorResources(top)
	nodeResources.Compatibility()
	response.NodeResources = nodeResources
}

// Reload is a no-op but implements ReloadableFingerprint
func (f *CPUFingerprint) Reload() {}
