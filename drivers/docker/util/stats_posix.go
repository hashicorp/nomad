// +build !windows

package util

import (
	"runtime"

	docker "github.com/fsouza/go-dockerclient"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/stats"
)

var (
	DockerMeasuredCPUStats = []string{"Throttled Periods", "Throttled Time", "Percent"}
	DockerMeasuredMemStats = []string{"RSS", "Cache", "Swap", "Usage", "Max Usage"}
)

func DockerStatsToTaskResourceUsage(s *docker.Stats) *cstructs.TaskResourceUsage {
	ms := &cstructs.MemoryStats{
		RSS:      s.MemoryStats.Stats.Rss,
		Cache:    s.MemoryStats.Stats.Cache,
		Swap:     s.MemoryStats.Stats.Swap,
		Usage:    s.MemoryStats.Usage,
		MaxUsage: s.MemoryStats.MaxUsage,
		Measured: DockerMeasuredMemStats,
	}

	cs := &cstructs.CpuStats{
		ThrottledPeriods: s.CPUStats.ThrottlingData.ThrottledPeriods,
		ThrottledTime:    s.CPUStats.ThrottlingData.ThrottledTime,
		Measured:         DockerMeasuredCPUStats,
	}

	// Calculate percentage
	cs.Percent = CalculateCPUPercent(
		s.CPUStats.CPUUsage.TotalUsage, s.PreCPUStats.CPUUsage.TotalUsage,
		s.CPUStats.SystemCPUUsage, s.PreCPUStats.SystemCPUUsage, runtime.NumCPU())
	cs.SystemMode = CalculateCPUPercent(
		s.CPUStats.CPUUsage.UsageInKernelmode, s.PreCPUStats.CPUUsage.UsageInKernelmode,
		s.CPUStats.CPUUsage.TotalUsage, s.PreCPUStats.CPUUsage.TotalUsage, runtime.NumCPU())
	cs.UserMode = CalculateCPUPercent(
		s.CPUStats.CPUUsage.UsageInUsermode, s.PreCPUStats.CPUUsage.UsageInUsermode,
		s.CPUStats.CPUUsage.TotalUsage, s.PreCPUStats.CPUUsage.TotalUsage, runtime.NumCPU())
	cs.TotalTicks = (cs.Percent / 100) * stats.TotalTicksAvailable() / float64(runtime.NumCPU())

	return &cstructs.TaskResourceUsage{
		ResourceUsage: &cstructs.ResourceUsage{
			MemoryStats: ms,
			CpuStats:    cs,
		},
		Timestamp: s.Read.UTC().UnixNano(),
	}
}
