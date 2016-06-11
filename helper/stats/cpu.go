package stats

import (
	"github.com/shirou/gopsutil/cpu"
	"sync"
)

var (
	clkSpeed  float64
	ticksLock sync.Mutex
)

// TotalTicksAvailable calculates the total frequency available across all cores
func TotalTicksAvailable() float64 {
	ticksLock.Lock()
	defer ticksLock.Unlock()
	if clkSpeed == 0.0 {
		var cpuInfo []cpu.InfoStat
		var err error

		var totalTicks float64
		if cpuInfo, err = cpu.Info(); err == nil {
			for _, cpu := range cpuInfo {
				totalTicks += cpu.Mhz
			}
			clkSpeed = totalTicks
		}
	}
	return clkSpeed
}
