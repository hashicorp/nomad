package drivers

import (
	"errors"
	"slices"

	"github.com/hashicorp/nomad/plugin-interface/device"
)

// MemoryStats holds memory usage related stats
type MemoryStats struct {
	RSS            uint64
	Cache          uint64
	Swap           uint64
	MappedFile     uint64
	Usage          uint64
	MaxUsage       uint64
	KernelUsage    uint64
	KernelMaxUsage uint64

	// A list of fields whose values were actually sampled
	Measured []string
}

func (ms *MemoryStats) Add(other *MemoryStats) {
	if other == nil {
		return
	}

	ms.RSS += other.RSS
	ms.Cache += other.Cache
	ms.Swap += other.Swap
	ms.MappedFile += other.MappedFile
	ms.Usage += other.Usage
	ms.MaxUsage += other.MaxUsage
	ms.KernelUsage += other.KernelUsage
	ms.KernelMaxUsage += other.KernelMaxUsage
	ms.Measured = slices.Compact(slices.Concat(ms.Measured, other.Measured))
}

// CpuStats holds cpu usage related stats
type CpuStats struct {
	SystemMode       float64
	UserMode         float64
	TotalTicks       float64
	ThrottledPeriods uint64
	ThrottledTime    uint64
	Percent          float64

	// A list of fields whose values were actually sampled
	Measured []string
}

func (cs *CpuStats) Add(other *CpuStats) {
	if other == nil {
		return
	}

	cs.SystemMode += other.SystemMode
	cs.UserMode += other.UserMode
	cs.TotalTicks += other.TotalTicks
	cs.ThrottledPeriods += other.ThrottledPeriods
	cs.ThrottledTime += other.ThrottledTime
	cs.Percent += other.Percent
	cs.Measured = slices.Compact(slices.Concat(cs.Measured, other.Measured))
}

// ResourceUsage holds information related to cpu and memory stats
type ResourceUsage struct {
	MemoryStats *MemoryStats
	CpuStats    *CpuStats
	DeviceStats []*device.DeviceGroupStats
}

func (ru *ResourceUsage) Add(other *ResourceUsage) {
	ru.MemoryStats.Add(other.MemoryStats)
	ru.CpuStats.Add(other.CpuStats)
	ru.DeviceStats = append(ru.DeviceStats, other.DeviceStats...)
}

// TaskResourceUsage holds aggregated resource usage of all processes in a Task
// and the resource usage of the individual pids
type TaskResourceUsage struct {
	ResourceUsage *ResourceUsage
	Timestamp     int64 // UnixNano
	Pids          map[string]*ResourceUsage
}

// CheckBufSize is the size of the buffer that is used for job output
const CheckBufSize = 4 * 1024

// DriverStatsNotImplemented is the error to be returned if a driver doesn't
// implement stats.
var DriverStatsNotImplemented = errors.New("stats not implemented for driver")
