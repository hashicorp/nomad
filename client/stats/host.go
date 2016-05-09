package stats

import (
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

type HostStats struct {
	Memory *MemoryStats
	CPU    []*CPUStats
}

type MemoryStats struct {
	Total     uint64
	Available uint64
	Used      uint64
	Free      uint64
}

type CPUStats struct {
	CPU    string
	User   float64
	System float64
	Idle   float64
}

func CollectHostStats() (*HostStats, error) {
	memStats, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	ms := &MemoryStats{
		Total:     memStats.Total,
		Available: memStats.Available,
		Used:      memStats.Used,
		Free:      memStats.Free,
	}

	cpuStats, err := cpu.Times(true)
	cs := make([]*CPUStats, len(cpuStats))
	for idx, cpuStat := range cpuStats {
		cs[idx] = &CPUStats{
			CPU:    cpuStat.CPU,
			User:   cpuStat.User,
			System: cpuStat.System,
			Idle:   cpuStat.Idle,
		}
	}

	hs := &HostStats{
		Memory: ms,
		CPU:    cs,
	}
	return hs, nil
}
