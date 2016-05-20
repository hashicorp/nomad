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
	procDelta := float64(currentProcessUsage) - float64(c.prevProcessUsage)
	delta := (time.Now().Sub(c.prevTime).Seconds()) * float64(c.totalCpus)
	percent := ((procDelta / delta) * 1000) * float64(c.totalCpus)
	c.prevProcessUsage = currentProcessUsage
	return percent

}
