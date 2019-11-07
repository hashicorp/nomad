// +build !windows

package stats

import (
	shelpers "github.com/hashicorp/nomad/helper/stats"
	"github.com/shirou/gopsutil/cpu"
)

func (h *HostStatsCollector) collectCPUStats() (cpus []*CPUStats, totalTicks float64, err error) {

	ticksConsumed := 0.0
	cpuStats, err := cpu.Times(true)
	if err != nil {
		return nil, 0.0, err
	}
	cs := make([]*CPUStats, len(cpuStats))
	for idx, cpuStat := range cpuStats {
		percentCalculator, ok := h.statsCalculator[cpuStat.CPU]
		if !ok {
			percentCalculator = NewHostCpuStatsCalculator()
			h.statsCalculator[cpuStat.CPU] = percentCalculator
		}
		idle, user, system, total := percentCalculator.Calculate(cpuStat)
		cs[idx] = &CPUStats{
			CPU:    cpuStat.CPU,
			User:   user,
			System: system,
			Idle:   idle,
			Total:  total,
		}
		ticksConsumed += (total / 100.0) * (shelpers.TotalTicksAvailable() / float64(len(cpuStats)))
	}

	return cs, ticksConsumed, nil
}
