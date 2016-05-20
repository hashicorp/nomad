package stats

import (
	"log"
	"runtime"
	"time"
)

type CpuStats struct {
	prevProcessUsage float64
	prevTime         time.Time

	totalCpus int
	logger    *log.Logger
}

func NewCpuStats(logger *log.Logger) *CpuStats {
	numCpus := runtime.NumCPU()
	return &CpuStats{totalCpus: numCpus, logger: logger}
}

func (c *CpuStats) Percent(currentProcessUsage float64) float64 {
	now := time.Now()

	if c.prevProcessUsage == 0.0 {
		// invoked first time
		c.prevProcessUsage = currentProcessUsage
		c.prevTime = now
		return 0.0
	}

	numcpu := runtime.NumCPU()
	delta := (now.Sub(c.prevTime).Seconds()) * float64(numcpu)
	ret := c.calculatePercent(c.prevProcessUsage, currentProcessUsage, delta, numcpu)
	c.prevProcessUsage = currentProcessUsage
	c.prevTime = now
	return ret

}

func (c *CpuStats) calculatePercent(t1, t2 float64, delta float64, numcpu int) float64 {
	if delta == 0 {
		return 0
	}
	delta_proc := t2 - t1
	overall_percent := ((delta_proc / delta) * 100) * float64(numcpu)
	return overall_percent
}
