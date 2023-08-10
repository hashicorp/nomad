// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package numalib

import (
	"github.com/hashicorp/nomad/client/lib/idset"
)

// A SystemScanner represents one methodology of detecting CPU hardware on a
// system. Detectable information is accumulated into a given Topology.
type SystemScanner interface {
	ScanSystem(*Topology)
}

// Scan each of the given scanners in order and accumulate the results into
// a single Topology, which can then be used to answer questions about the CPU
// topology of the system.
func Scan(scanners []SystemScanner) *Topology {
	top := new(Topology)
	for _, scanner := range scanners {
		scanner.ScanSystem(top)
	}
	return top
}

// ConfigScanner provides override values coming from Nomad Client configuration.
// This scanner must run last as the client configuration has the final say if
// values there are set by an operator.
type ConfigScanner struct {
	// ReservableCores comes from client.reservable_cores.
	//
	// Not yet documented as of 1.6.
	//
	// Only meaningful on Linux, this value can be used to override the set of
	// CPU core IDs we may make use of. Normally these are detected by reading
	// Nomad parent cgroup cpuset interface file.
	ReservableCores *idset.Set[CoreID]

	// TotalCompute comes from client.cpu_total_compute.
	//
	// Used to set the total MHz of available CPU bandwidth on a system. This
	// value is used by the scheduler for fitment, and by the client for computing
	// task / alloc / client resource utilization. Therefor this value:
	//  - Should NOT be set if Nomad was able to fingerprint a value.
	//  - Should NOT be used to over/under provision compute resources.
	TotalCompute MHz

	// ReservedCores comes from client.reserved.cores.
	//
	// Used to withhold a set of cores from being used by Nomad for scheduling.
	ReservedCores *idset.Set[CoreID]

	// ReservedCompute comes from client.reserved.cpu.
	//
	// Used to withhold an amount of MHz of CPU bandwidth from being used by
	// Nomad for scheduling.
	ReservedCompute MHz
}

func (cs *ConfigScanner) ScanSystem(top *Topology) {
	// disable cores that are not reservable (i.e. override effective cpuset)
	if cs.ReservableCores != nil {
		for i := 0; i < len(top.Cores); i++ {
			if !cs.ReservableCores.Contains(top.Cores[i].ID) {
				top.Cores[i].Disable = true
			}
		}
	}

	// disable cores that are not usable (i.e. hide from scheduler)
	for i := 0; i < len(top.Cores); i++ {
		if cs.ReservedCores.Contains(top.Cores[i].ID) {
			top.Cores[i].Disable = true
		}
	}

	// set total compute from client configuration
	top.OverrideTotalCompute = cs.TotalCompute

	// set the reserved compute from client configuration
	top.OverrideWitholdCompute = cs.ReservedCompute
}
