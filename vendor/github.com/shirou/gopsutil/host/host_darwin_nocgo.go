// +build darwin
// +build !cgo

package host

import "github.com/shirou/gopsutil/internal/common"

func SensorsTemperatures() ([]TemperatureStat, error) {
	return []TemperatureStat{}, common.ErrNotImplementedError
}
