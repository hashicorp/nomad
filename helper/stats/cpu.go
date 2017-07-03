package stats

import (
	"fmt"
	"math"
	"sync"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/shirou/gopsutil/cpu"
)

var (
	cpuMhzPerCore float64
	cpuModelName  string
	cpuNumCores   int
	cpuTotalTicks float64

	initErr error
	onceLer sync.Once
)

func Init() error {
	onceLer.Do(func() {
		var merrs *multierror.Error
		var err error
		if cpuNumCores, err = cpu.Counts(true); err != nil {
			merrs = multierror.Append(merrs, fmt.Errorf("Unable to determine the number of CPU cores available: %v", err))
		}

		var cpuInfo []cpu.InfoStat
		if cpuInfo, err = cpu.Info(); err != nil {
			merrs = multierror.Append(merrs, fmt.Errorf("Unable to obtain CPU information: %v", initErr))
		}

		for _, cpu := range cpuInfo {
			cpuModelName = cpu.ModelName
			cpuMhzPerCore = cpu.Mhz
			break
		}

		// Floor all of the values such that small difference don't cause the
		// node to fall into a unique computed node class
		cpuMhzPerCore = math.Floor(cpuMhzPerCore)
		cpuTotalTicks = math.Floor(float64(cpuNumCores) * cpuMhzPerCore)

		// Set any errors that occurred
		initErr = merrs.ErrorOrNil()
	})
	return initErr
}

// CPUModelName returns the number of CPU cores available
func CPUNumCores() int {
	return cpuNumCores
}

// CPUMHzPerCore returns the MHz per CPU core
func CPUMHzPerCore() float64 {
	return cpuMhzPerCore
}

// CPUModelName returns the model name of the CPU
func CPUModelName() string {
	return cpuModelName
}

// TotalTicksAvailable calculates the total Mhz available across all cores
func TotalTicksAvailable() float64 {
	return cpuTotalTicks
}
