// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package util

import (
	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/nomad/client/lib/cpustats"
	cstructs "github.com/hashicorp/nomad/client/structs"
)

var (
	DockerMeasuredCPUStats = []string{"Throttled Periods", "Throttled Time", "Percent"}

	// cgroup-v2 only exposes a subset of memory stats
	DockerCgroupV1MeasuredMemStats = []string{"RSS", "Cache", "Swap", "Usage", "Max Usage"}
	DockerCgroupV2MeasuredMemStats = []string{"Cache", "Swap", "Usage"}
)

func DockerStatsToTaskResourceUsage(s *docker.Stats, compute cpustats.Compute) *cstructs.TaskResourceUsage {
	var (
		totalCompute = compute.TotalCompute
		totalCores   = compute.NumCores
	)

	measuredMems := DockerCgroupV1MeasuredMemStats

	// use a simple heuristic to check if cgroup-v2 is used.
	// go-dockerclient doesn't distinguish between 0 and not-present value
	if s.MemoryStats.Stats.Rss == 0 && s.MemoryStats.MaxUsage == 0 && s.MemoryStats.Usage != 0 {
		measuredMems = DockerCgroupV2MeasuredMemStats
	}

	ms := &cstructs.MemoryStats{
		RSS:        s.MemoryStats.Stats.Rss,
		Cache:      s.MemoryStats.Stats.Cache,
		Swap:       s.MemoryStats.Stats.Swap,
		MappedFile: s.MemoryStats.Stats.MappedFile,
		Usage:      s.MemoryStats.Usage,
		MaxUsage:   s.MemoryStats.MaxUsage,
		Measured:   measuredMems,
	}

	cs := &cstructs.CpuStats{
		ThrottledPeriods: s.CPUStats.ThrottlingData.ThrottledPeriods,
		ThrottledTime:    s.CPUStats.ThrottlingData.ThrottledTime,
		Measured:         DockerMeasuredCPUStats,
	}

	// Calculate percentage
	cs.Percent = CalculateCPUPercent(
		s.CPUStats.CPUUsage.TotalUsage, s.PreCPUStats.CPUUsage.TotalUsage,
		s.CPUStats.SystemCPUUsage, s.PreCPUStats.SystemCPUUsage, totalCores)
	cs.SystemMode = CalculateCPUPercent(
		s.CPUStats.CPUUsage.UsageInKernelmode, s.PreCPUStats.CPUUsage.UsageInKernelmode,
		s.CPUStats.CPUUsage.TotalUsage, s.PreCPUStats.CPUUsage.TotalUsage, totalCores)
	cs.UserMode = CalculateCPUPercent(
		s.CPUStats.CPUUsage.UsageInUsermode, s.PreCPUStats.CPUUsage.UsageInUsermode,
		s.CPUStats.CPUUsage.TotalUsage, s.PreCPUStats.CPUUsage.TotalUsage, totalCores)

	cs.TotalTicks = (cs.Percent / 100) * float64(totalCompute) / float64(totalCores)

	return &cstructs.TaskResourceUsage{
		ResourceUsage: &cstructs.ResourceUsage{
			MemoryStats: ms,
			CpuStats:    cs,
		},
		Timestamp: s.Read.UTC().UnixNano(),
	}
}
