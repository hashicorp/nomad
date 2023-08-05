// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package stats

import (
	"runtime"
	"time"
)

var (
	cpuTotalTicks uint64
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
	return (percent / 100) * float64(CpuTotalTicks()) / float64(c.totalCpus)
}

func (c *CpuStats) calculatePercent(t1, t2 float64, timeDelta int64) float64 {
	vDelta := t2 - t1
	if timeDelta <= 0 || vDelta <= 0.0 {
		return 0.0
	}

	overall_percent := (vDelta / float64(timeDelta)) * 100.0
	return overall_percent
}

// Set the total ticks available across all cores.
func SetCpuTotalTicks(newCpuTotalTicks uint64) {
	cpuTotalTicks = newCpuTotalTicks
}

// CpuTotalTicks calculates the total MHz available across all cores.
//
// Where asymetric cores are correctly detected, the total ticks is the sum of
// the performance across both core types.
//
// Where asymetric cores are not correctly detected (such as Intel 13th gen),
// the total ticks available is over-estimated, as we assume all cores are P
// cores.
func CpuTotalTicks() uint64 {
	return cpuTotalTicks
}
