package stats

import (
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

// HostStats represents resource usage stats of the host running a Nomad client
type HostStats struct {
	Memory *MemoryStats
	CPU    []*CPUStats
}

// MemoryStats represnts stats related to virtual memory usage
type MemoryStats struct {
	Total     uint64
	Available uint64
	Used      uint64
	Free      uint64
}

// CPUStats represents stats related to cpu usage
type CPUStats struct {
	CPU    string
	User   float64
	System float64
	Idle   float64
	Total  float64
}

// HostStatsCollector collects host resource usage stats
type HostStatsCollector struct {
	statsCalculator map[string]*HostCpuStatsCalculator
}

// NewHostStatsCollector returns a HostStatsCollector
func NewHostStatsCollector() (*HostStatsCollector, error) {
	times, err := cpu.Times(true)
	if err != nil {
		return nil, err
	}
	statsCalculator := make(map[string]*HostCpuStatsCalculator)
	for _, time := range times {
		statsCalculator[time.CPU] = NewHostCpuStatsCalculator()
	}
	return &HostStatsCollector{statsCalculator: statsCalculator}, nil
}

// Collect collects stats related to resource usage of a host
func (h *HostStatsCollector) Collect() (*HostStats, error) {
	memStats, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	ms := &MemoryStats{
		Total:     memStats.Total,
		Available: memStats.Available,
		Used:      memStats.Used,
		Free:      memStats.Free,
	}

	cpuStats, err := cpu.Times(true)
	cs := make([]*CPUStats, len(cpuStats))
	for idx, cpuStat := range cpuStats {
		cs[idx] = &CPUStats{
			CPU:    cpuStat.CPU,
			User:   cpuStat.User,
			System: cpuStat.System,
			Idle:   cpuStat.Idle,
		}
		if percentCalculator, ok := h.statsCalculator[cpuStat.CPU]; ok {
			idle, user, system, total := percentCalculator.Calculate(cpuStat)
			cs[idx].Idle = idle
			cs[idx].System = system
			cs[idx].User = user
			cs[idx].Total = total
		} else {
			h.statsCalculator[cpuStat.CPU] = NewHostCpuStatsCalculator()
		}
	}

	hs := &HostStats{
		Memory: ms,
		CPU:    cs,
	}
	return hs, nil
}

// HostCpuStatsCalculator calculates cpu usage percentages
type HostCpuStatsCalculator struct {
	prevIdle   float64
	prevUser   float64
	prevSystem float64
	prevBusy   float64
	prevTotal  float64
}

// NewHostCpuStatsCalculator returns a HostCpuStatsCalculator
func NewHostCpuStatsCalculator() *HostCpuStatsCalculator {
	return &HostCpuStatsCalculator{}
}

// Calculate calculates the current cpu usage percentages
func (h *HostCpuStatsCalculator) Calculate(times cpu.TimesStat) (idle float64, user float64, system float64, total float64) {
	currentIdle := times.Idle
	currentUser := times.User
	currentSystem := times.System
	currentTotal := times.Total()

	deltaTotal := currentTotal - h.prevTotal
	idle = ((currentIdle - h.prevIdle) / deltaTotal) * 100
	user = ((currentUser - h.prevUser) / deltaTotal) * 100
	system = ((currentSystem - h.prevSystem) / deltaTotal) * 100

	currentBusy := times.User + times.System + times.Nice + times.Iowait + times.Irq +
		times.Softirq + times.Steal + times.Guest + times.GuestNice + times.Stolen

	total = ((currentBusy - h.prevBusy) / deltaTotal) * 100

	h.prevIdle = currentIdle
	h.prevUser = currentUser
	h.prevSystem = currentSystem
	h.prevTotal = currentTotal
	h.prevBusy = currentBusy

	return
}
