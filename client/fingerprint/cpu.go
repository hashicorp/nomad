// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/nomad/lib/cpuset"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/stats"
	shelpers "github.com/hashicorp/nomad/helper/stats"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// defaultCPUTicks is the default amount of CPU resources assumed to be
	// available if the CPU performance data is unable to be detected. This is
	// common on EC2 instances, where the env_aws fingerprinter will follow up,
	// setting an accurate value.
	defaultCPUTicks = 1000 // 1 core * 1 GHz
)

// CPUFingerprint is used to fingerprint the CPU
type CPUFingerprint struct {
	StaticFingerprinter
	logger hclog.Logger

	// accumulates result in these resource structs
	resources     *structs.Resources
	nodeResources *structs.NodeResources
}

// NewCPUFingerprint is used to create a CPU fingerprint
func NewCPUFingerprint(logger hclog.Logger) Fingerprint {
	return &CPUFingerprint{
		logger:        logger.Named("cpu"),
		resources:     new(structs.Resources), // COMPAT (to be removed after 0.10)
		nodeResources: new(structs.NodeResources),
	}
}

func (f *CPUFingerprint) Fingerprint(request *FingerprintRequest, response *FingerprintResponse) error {
	f.initialize(request)

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
	if err := stats.Init(uint64(request.Config.CpuCompute)); err != nil {
		f.logger.Warn("failed initializing stats collector", "error", err)
	}
}

func (f *CPUFingerprint) setModelName(response *FingerprintResponse) {
	if modelName := stats.CPUModelName(); modelName != "" {
		response.AddAttribute("cpu.modelname", modelName)
		f.logger.Debug("detected CPU model", "name", modelName)
	}
}

func (*CPUFingerprint) frequency(mhz uint64) string {
	return fmt.Sprintf("%.0f", float64(mhz))
}

func (f *CPUFingerprint) setFrequency(response *FingerprintResponse) {
	power, efficiency := stats.CPUMHzPerCore()
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
	power, efficiency := stats.CPUNumCores()
	switch {
	case efficiency > 0:
		response.AddAttribute("cpu.numcores.efficiency", f.cores(efficiency))
		response.AddAttribute("cpu.numcores.power", f.cores(power))
		f.logger.Debug("detected CPU efficiency core count", "cores", efficiency)
		f.logger.Debug("detected CPU power core count", "cores", power)
	case power > 0:
		response.AddAttribute("cpu.numcores", f.cores(power))
		f.logger.Debug("detected CPU core count", power)
	}
	f.nodeResources.Cpu.TotalCpuCores = uint16(power + efficiency)
}

func (f *CPUFingerprint) setReservableCores(request *FingerprintRequest, response *FingerprintResponse) {
	reservable := request.Config.ReservableCores
	if len(reservable) > 0 {
		f.logger.Debug("reservable cores set by config", "cpuset", reservable)
	} else {
		cgroupParent := request.Config.CgroupParent
		if reservable = f.deriveReservableCores(cgroupParent); reservable != nil {
			if request.Node.ReservedResources != nil {
				forNode := request.Node.ReservedResources.Cpu.ReservedCpuCores
				reservable = cpuset.New(reservable...).Difference(cpuset.New(forNode...)).ToSlice()
				f.logger.Debug("client configuration reserves these cores for node", "cores", forNode)
			}
			f.logger.Debug("set of reservable cores available for tasks", "cores", reservable)
		}
	}

	response.AddAttribute("cpu.reservablecores", strconv.Itoa(len(reservable)))
	f.nodeResources.Cpu.ReservableCpuCores = reservable
}

func (f *CPUFingerprint) setTotalCompute(request *FingerprintRequest, response *FingerprintResponse) {
	var ticks uint64
	switch {
	case shelpers.CpuTotalTicks() > 0:
		ticks = shelpers.CpuTotalTicks()
	default:
		ticks = defaultCPUTicks
	}
	response.AddAttribute("cpu.totalcompute", fmt.Sprintf("%d", ticks))
	f.resources.CPU = int(ticks)
	f.nodeResources.Cpu.CpuShares = int64(ticks)
}

func (f *CPUFingerprint) setResponseResources(response *FingerprintResponse) {
	response.Resources = f.resources
	response.NodeResources = f.nodeResources
}
