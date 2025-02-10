// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package util

import (
	containerapi "github.com/docker/docker/api/types/container"
	"github.com/hashicorp/nomad/client/lib/cpustats"
	cstructs "github.com/hashicorp/nomad/client/structs"
)

var (
	// The statistics the Docker driver exposes
	DockerMeasuredCPUStats = []string{"Throttled Periods", "Throttled Time", "Percent"}
	DockerMeasuredMemStats = []string{"RSS", "Usage", "Max Usage"}
)

func DockerStatsToTaskResourceUsage(s *containerapi.Stats, compute cpustats.Compute) *cstructs.TaskResourceUsage {
	var (
		totalCompute = compute.TotalCompute
		totalCores   = compute.NumCores
	)

	ms := &cstructs.MemoryStats{
		RSS:      s.MemoryStats.PrivateWorkingSet,
		Usage:    s.MemoryStats.Commit,
		MaxUsage: s.MemoryStats.CommitPeak,
		Measured: DockerMeasuredMemStats,
	}

	cpuPercent := 0.0

	// https://github.com/moby/moby/blob/cbb885b07af59225eef12a8159e70d1485616d57/integration-cli/docker_api_stats_test.go#L47-L58
	// Max number of 100ns intervals between the previous time read and now
	possIntervals := uint64(s.Read.Sub(s.PreRead).Nanoseconds()) // Start with number of ns intervals
	possIntervals /= 100                                         // Convert to number of 100ns intervals
	possIntervals *= uint64(s.NumProcs)                          // Multiple by the number of processors

	// Intervals used
	intervalsUsed := s.CPUStats.CPUUsage.TotalUsage - s.PreCPUStats.CPUUsage.TotalUsage

	// Percentage avoiding divide-by-zero
	if possIntervals > 0 {
		cpuPercent = float64(intervalsUsed) / float64(possIntervals) * 100.0
	}

	cs := &cstructs.CpuStats{
		ThrottledPeriods: s.CPUStats.ThrottlingData.ThrottledPeriods,
		ThrottledTime:    s.CPUStats.ThrottlingData.ThrottledTime,
		Percent:          cpuPercent,
		TotalTicks:       (cpuPercent / 100) * float64(totalCompute) / float64(totalCores),
		Measured:         DockerMeasuredCPUStats,
	}

	return &cstructs.TaskResourceUsage{
		ResourceUsage: &cstructs.ResourceUsage{
			MemoryStats: ms,
			CpuStats:    cs,
		},
		Timestamp: s.Read.UTC().UnixNano(),
	}
}
