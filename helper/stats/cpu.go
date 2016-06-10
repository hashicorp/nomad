package stats

import (
	"github.com/shirou/gopsutil/cpu"
)

// TotalTicksAvailable calculates the total frequency available across all cores
func TotalTicksAvailable() (float64, error) {
	var clkSpeed float64
	var cpuInfo []cpu.InfoStat
	var err error

	if cpuInfo, err = cpu.Info(); err != nil {
		return 0.0, err
	}
	for _, cpu := range cpuInfo {
		clkSpeed += cpu.Mhz
	}
	return clkSpeed, nil
}
