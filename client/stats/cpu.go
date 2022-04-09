package stats

import (
	"runtime"
	"time"

	shelpers "github.com/hashicorp/nomad/helper/stats"
	"github.com/shirou/gopsutil/v3/cpu"
)

// CpuStats calculates cpu usage percentage
type CpuStats struct {
	prevCpuTime float64
	prevTime    time.Time

	totalCpus int
}

// NewCpuStats returns a cpu stats calculator
func NewCpuStats() *CpuStats {
	numCpus := runtime.NumCPU()
	cpuStats := &CpuStats{
		totalCpus: numCpus,
	}
	return cpuStats
}

// Percent calculates the cpu usage percentage based on the current cpu usage
// and the previous cpu usage where usage is given as time in nanoseconds spend
// in the cpu
func (c *CpuStats) Percent(cpuTime float64) float64 {
	now := time.Now()

	if c.prevCpuTime == 0.0 {
		// invoked first time
		c.prevCpuTime = cpuTime
		c.prevTime = now
		return 0.0
	}

	timeDelta := now.Sub(c.prevTime).Nanoseconds()
	ret := c.calculatePercent(c.prevCpuTime, cpuTime, timeDelta)
	c.prevCpuTime = cpuTime
	c.prevTime = now
	return ret
}

// TicksConsumed calculates the total ticks consumes by the process across all
// cpu cores
func (c *CpuStats) TicksConsumed(percent float64) float64 {
	return (percent / 100) * shelpers.TotalTicksAvailable() / float64(c.totalCpus)
}

func (c *CpuStats) calculatePercent(t1, t2 float64, timeDelta int64) float64 {
	vDelta := t2 - t1
	if timeDelta <= 0 || vDelta <= 0.0 {
		return 0.0
	}

	overall_percent := (vDelta / float64(timeDelta)) * 100.0
	return overall_percent
}

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
