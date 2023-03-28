package stats

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

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
	cpuTotalTicks          uint64
	cpuModelName           string
)

var (
	initErr error
	onceLer sync.Once
)

func Init() error {
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
			cpuTotalTicks = bigTicks + littleTicks
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
				cpuPowerCoreMHz = uint64(infoStat.Mhz)
				break
			}

			// compute ticks using only power core, until we add support for
			// detecting little cores on non-apple platforms
			cpuTotalTicks = uint64(cpuPowerCoreCount) * cpuPowerCoreMHz

			initErr = err
		}
	})
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

// TotalTicksAvailable calculates the total MHz available across all cores.
//
// Where asymetric cores are correctly detected, the total ticks is the sum of
// the performance across both core types.
//
// Where asymetric cores are not correctly detected (such as Intel 13th gen),
// the total ticks available is over-estimated, as we assume all cores are P
// cores.
func TotalTicksAvailable() uint64 {
	return cpuTotalTicks
}
