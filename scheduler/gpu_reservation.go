// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	gpuDeviceType = "gpu"

	gpuReservedCPUExhaustion    = "gpu reserved cpu"
	gpuReservedMemoryExhaustion = "gpu reserved memory"
)

func taskGroupUsesGPU(tg *structs.TaskGroup) bool {
	if tg == nil {
		return false
	}

	for _, task := range tg.Tasks {
		if task == nil || task.Resources == nil {
			continue
		}

		for _, device := range task.Resources.Devices {
			id := device.ID()
			if id != nil && id.Type == gpuDeviceType {
				return true
			}
		}
	}

	return false
}

func remainingHealthyGPUCount(node *structs.Node, allocs []*structs.Allocation) int {
	if node == nil || node.NodeResources == nil {
		return 0
	}

	accounter := structs.NewDeviceAccounter(node)
	accounter.AddAllocs(allocs)

	var free int
	for id, device := range accounter.Devices {
		if id.Type != gpuDeviceType {
			continue
		}
		for _, uses := range device.Instances {
			if uses == 0 {
				free++
			}
		}
	}

	return free
}

func gpuReservationCPUShares(
	node *structs.Node,
	available *structs.ComparableResources,
	coresPerGPU, freeGPUs int,
) int64 {
	if coresPerGPU <= 0 || freeGPUs <= 0 {
		return 0
	}
	if node == nil || node.NodeResources == nil ||
		node.NodeResources.Processors.Topology == nil || available == nil {
		return -1
	}

	usableCores := node.NodeResources.Processors.Topology.UsableCores()
	if usableCores == nil || usableCores.Empty() ||
		available.Flattened.Cpu.CpuShares <= 0 {
		return -1
	}

	requiredCores := int64(coresPerGPU * freeGPUs)
	usableCoreCount := int64(usableCores.Size())
	availableCPU := available.Flattened.Cpu.CpuShares

	return (availableCPU*requiredCores + usableCoreCount - 1) / usableCoreCount
}

func gpuReservationViolated(
	node *structs.Node,
	finalAllocs []*structs.Allocation,
	cfg structs.SchedulerGPUResourceReservation,
) (bool, string) {
	if cfg.IsZero() {
		return false, ""
	}

	freeGPUs := remainingHealthyGPUCount(node, finalAllocs)
	if freeGPUs == 0 {
		return false, ""
	}

	used := gpuReservationResourcesUsed(finalAllocs)

	if cfg.CPUCores > 0 {
		available := gpuReservationAvailableComparable(node)
		reservedCPU := gpuReservationCPUShares(node, available, cfg.CPUCores, freeGPUs)
		if reservedCPU < 0 {
			return true, gpuReservedCPUExhaustion
		}

		remainingCPU := available.Flattened.Cpu.CpuShares - used.Flattened.Cpu.CpuShares
		if remainingCPU < reservedCPU {
			return true, gpuReservedCPUExhaustion
		}
	}

	if cfg.MemoryMB > 0 {
		availableMemory, ok := gpuReservationAvailableMemoryMB(node)
		if !ok {
			return true, gpuReservedMemoryExhaustion
		}

		reservedMemory := int64(cfg.MemoryMB * freeGPUs)
		remainingMemory := availableMemory - used.Flattened.Memory.MemoryMB
		if remainingMemory < reservedMemory {
			return true, gpuReservedMemoryExhaustion
		}
	}

	return false, ""
}

func gpuReservationCannotCompute(
	node *structs.Node,
	finalAllocs []*structs.Allocation,
	cfg structs.SchedulerGPUResourceReservation,
) (bool, string) {
	if cfg.IsZero() || remainingHealthyGPUCount(node, finalAllocs) == 0 {
		return false, ""
	}
	if node == nil || node.NodeResources == nil ||
		node.NodeResources.Processors.Topology == nil {
		if cfg.CPUCores > 0 {
			return true, gpuReservedCPUExhaustion
		}
		return true, gpuReservedMemoryExhaustion
	}
	if cfg.CPUCores > 0 {
		available := gpuReservationAvailableComparable(node)
		if gpuReservationCPUShares(node, available, cfg.CPUCores, 1) < 0 {
			return true, gpuReservedCPUExhaustion
		}
	}

	return false, ""
}

func gpuReservationAvailableComparable(node *structs.Node) *structs.ComparableResources {
	if node == nil || node.NodeResources == nil ||
		node.NodeResources.Processors.Topology == nil {
		return nil
	}

	available := node.NodeResources.Comparable()
	if available == nil {
		return nil
	}
	available.Subtract(node.ReservedResources.Comparable())

	return available
}

func gpuReservationAvailableMemoryMB(node *structs.Node) (int64, bool) {
	if node == nil || node.NodeResources == nil {
		return 0, false
	}

	available := node.NodeResources.Memory.MemoryMB
	if node.ReservedResources != nil {
		available -= node.ReservedResources.Memory.MemoryMB
	}

	return available, true
}

func gpuReservationResourcesUsed(allocs []*structs.Allocation) *structs.ComparableResources {
	used := new(structs.ComparableResources)
	for _, alloc := range allocs {
		if alloc == nil || alloc.ClientTerminalStatus() ||
			alloc.AllocatedResources == nil {
			continue
		}

		used.Add(alloc.AllocatedResources.Comparable())
	}

	return used
}
