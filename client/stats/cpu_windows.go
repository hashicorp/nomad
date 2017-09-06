// +build windows

package stats

import (
	"fmt"

	shelpers "github.com/hashicorp/nomad/helper/stats"
	"github.com/shirou/gopsutil/cpu"
)

func (h *HostStatsCollector) collectCPUStats() (cpus []*CPUStats, totalTicks float64, err error) {
	// Get the per cpu stats
	cpuStats, err := cpu.Times(true)
	if err != nil {
		return nil, 0.0, err
	}

	cs := make([]*CPUStats, len(cpuStats))
	for idx, cpuStat := range cpuStats {

		// On windows they are already in percent
		cs[idx] = &CPUStats{
			CPU:    cpuStat.CPU,
			User:   cpuStat.User,
			System: cpuStat.System,
			Idle:   cpuStat.Idle,
			Total:  cpuStat.Total(),
		}
	}

	// Get the number of ticks
	allCpu, err := cpu.Times(false)
	if err != nil {
		return nil, 0.0, err
	}
	if len(allCpu) != 1 {
		return nil, 0.0, fmt.Errorf("unexpected number of cpus (%d)", len(allCpu))
	}

	// We use the calculator because when retrieving against all cpus it is
	// returned as ticks.
	all := allCpu[0]
	percentCalculator, ok := h.statsCalculator[all.CPU]
	if !ok {
		percentCalculator = NewHostCpuStatsCalculator()
		h.statsCalculator[all.CPU] = percentCalculator
	}
	_, _, _, total := percentCalculator.Calculate(all)
	ticks := (total / 100) * shelpers.TotalTicksAvailable()

	return cs, ticks, nil
}
