// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package stats

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/nomad/helper/stats"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shoenig/go-m1cpu"
)

const (
	// cpuInfoTimeout is the timeout used when gathering CPU info. This is used
	// to override the default timeout in gopsutil which has a tendency to
	// timeout on Windows.
	cpuInfoTimeout = 60 * time.Second
)

var (
	cpuPowerCoreCount      int
	cpuPowerCoreMHz        uint64
	cpuEfficiencyCoreCount int
	cpuEfficiencyCoreMHz   uint64
	cpuModelName           string
)

var (
	detectedCpuTotalTicks uint64
	initErr               error
	onceLer               sync.Once
)

func Init(configCpuTotalCompute uint64) error {
	onceLer.Do(func() {
		switch {
		case m1cpu.IsAppleSilicon():
			cpuModelName = m1cpu.ModelName()
			cpuPowerCoreCount = m1cpu.PCoreCount()
			cpuPowerCoreMHz = m1cpu.PCoreHz() / 1_000_000
			cpuEfficiencyCoreCount = m1cpu.ECoreCount()
			cpuEfficiencyCoreMHz = m1cpu.ECoreHz() / 1_000_000
			bigTicks := uint64(cpuPowerCoreCount) * cpuPowerCoreMHz
			littleTicks := uint64(cpuEfficiencyCoreCount) * cpuEfficiencyCoreMHz
			detectedCpuTotalTicks = bigTicks + littleTicks
		default:
			// for now, all other cpu types assume only power cores
			// todo: this is already not true for Intel 13th generation

			var err error
			if cpuPowerCoreCount, err = cpu.Counts(true); err != nil {
				initErr = errors.Join(initErr, fmt.Errorf("failed to detect number of CPU cores: %w", err))
			}

			ctx, cancel := context.WithTimeout(context.Background(), cpuInfoTimeout)
			defer cancel()

			var cpuInfoStats []cpu.InfoStat
			if cpuInfoStats, err = cpu.InfoWithContext(ctx); err != nil {
				initErr = errors.Join(initErr, fmt.Errorf("Unable to obtain CPU information: %w", err))
			}

			for _, infoStat := range cpuInfoStats {
				cpuModelName = infoStat.ModelName
				if uint64(infoStat.Mhz) > cpuPowerCoreMHz {
					cpuPowerCoreMHz = uint64(infoStat.Mhz)
				}
			}

			// compute ticks using only power core, until we add support for
			// detecting little cores on non-apple platforms
			detectedCpuTotalTicks = uint64(cpuPowerCoreCount) * cpuPowerCoreMHz

			initErr = err
		}

		stats.SetCpuTotalTicks(detectedCpuTotalTicks)
	})

	// override the computed value with the config value if it is set
	if configCpuTotalCompute > 0 {
		stats.SetCpuTotalTicks(configCpuTotalCompute)
	}

	return initErr
}

// CPUNumCores returns the number of CPU cores available.
//
// This is represented with two values - (Power (P), Efficiency (E)) so we can
// correctly compute total compute for processors with asymetric cores such as
// Apple Silicon.
//
// For platforms with symetric cores (or where we do not correcly detect asymetric
// cores), all cores are presented as P cores.
func CPUNumCores() (int, int) {
	return cpuPowerCoreCount, cpuEfficiencyCoreCount
}

// CPUMHzPerCore returns the MHz per CPU (P, E) core type.
//
// As with CPUNumCores, asymetric core detection currently only works with
// Apple Silicon CPUs.
func CPUMHzPerCore() (uint64, uint64) {
	return cpuPowerCoreMHz, cpuEfficiencyCoreMHz
}

// CPUModelName returns the model name of the CPU.
func CPUModelName() string {
	return cpuModelName
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
		ticks := (total / 100.0) * (float64(stats.CpuTotalTicks()) / float64(len(cpuStats)))
		cs[idx] = &CPUStats{
			CPU:          cpuStat.CPU,
			User:         user,
			System:       system,
			Idle:         idle,
			TotalPercent: total,
			TotalTicks:   ticks,
		}
		ticksConsumed += ticks
	}

	return cs, ticksConsumed, nil
}
