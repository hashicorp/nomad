// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"github.com/shoenig/netlog"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	cpuPartsHookName = "cpuparts_hook"
)

type cpuPartsHook struct {
	logger     hclog.Logger
	allocID    string
	allocCores *idset.Set[numalib.CoreID]
}

func newCPUPartsHook(
	logger hclog.Logger,
	alloc *structs.Allocation,
) *cpuPartsHook {
	return &cpuPartsHook{
		// logger:     logger,
		logger:     netlog.New("CPUPARTS"),
		allocID:    alloc.ID,
		allocCores: reservations(alloc),
	}
}

func (h *cpuPartsHook) Name() string {
	return cpuPartsHookName
}

// accumulate the cores for the alloc
func reservations(alloc *structs.Allocation) *idset.Set[numalib.CoreID] {
	cores := idset.Empty[numalib.CoreID]()
	for _, recs := range alloc.AllocatedResources.Tasks {
		for _, core := range recs.Cpu.ReservedCores {
			cores.Insert(numalib.CoreID(core))
		}
	}
	return cores
}

// Prerun will ammend the nomad.slice/reserve/cpuset.cores
// cgroup interface with the cores being used for any tasks
// in this alloc making use of reserved resources.cores.
func (h *cpuPartsHook) Prerun() error {
	h.logger.Info("Prerun()", "alloc_id", h.allocID, "rcores", h.allocCores)
	return nil
}

// Destroy will ammend the nomad.slice/reserve/cpuset.cores
// cgroup interface by removing any cores being used for tasks
// in this alloc that were making use of reserved resources.cores.
func (h *cpuPartsHook) Destroy() error {
	h.logger.Info("Destroy()", "alloc_id", h.allocID, "rcores", h.allocCores)
	return nil
}
