package stats

import (
	"github.com/shirou/gopsutil/cpu"
)

type CpuStats struct {
	prevSystemUsage  float64
	prevProcessUsage uint64

	totalCpus int
}

func NewCpuStats() (*CpuStats, error) {
	cpuInfo, err := cpu.Info()
	if err != nil {
		return nil, err
	}
	return &CpuStats{totalCpus: len(cpuInfo)}, nil
}

func (c *CpuStats) Percent(currentProcessUsage uint64) float64 {
	percent := 0.0

	sysCPUStats, err := cpu.Times(false)
	if err != nil {
		return 0
	}
	currentSysUsage := 0.0
	for _, cpuStat := range sysCPUStats {
		currentSysUsage += cpuStat.Total() * 1000000000
	}

	delta := float64(currentProcessUsage) - float64(c.prevProcessUsage)
	sysDelta := float64(currentSysUsage) - float64(c.prevSystemUsage)

	percent = (delta / sysDelta) * float64(c.totalCpus) * 100.0
	c.prevSystemUsage = currentSysUsage
	c.prevProcessUsage = currentProcessUsage
	return percent
}
