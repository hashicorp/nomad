// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"github.com/shoenig/netlog"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/cgroupslib"
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	cpuPartsHookName = "cpuparts_hook"
)

type cpuPartsHook struct {
	logger  hclog.Logger
	allocID string

	reservations *idset.Set[idset.CoreID]
	partitions   cgroupslib.Partition
}

func newCPUPartsHook(
	logger hclog.Logger,
	partitions cgroupslib.Partition,
	alloc *structs.Allocation,
) *cpuPartsHook {
	return &cpuPartsHook{
		// logger:     logger,
		logger:       netlog.New("CPUPARTS"),
		allocID:      alloc.ID,
		partitions:   partitions,
		reservations: alloc.ReservedCores(),
	}
}

func (h *cpuPartsHook) Name() string {
	return cpuPartsHookName
}

func (h *cpuPartsHook) Prerun() error {
	netlog.Cyan("cpuPartsHook.Prerun()")
	h.partitions.Reserve(h.reservations)
	return nil
}

func (h *cpuPartsHook) Destroy() error {
	netlog.Cyan("cpuPartsHook.Destroy()")
	h.partitions.Release(h.reservations)
	return nil
}
