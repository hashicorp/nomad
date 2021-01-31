package stats

import (
	"math"
	"runtime"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"
)

// HostStats represents resource usage stats of the host running a Nomad client
type HostStats struct {
	Memory           *MemoryStats
	CPU              []*CPUStats
	DiskStats        []*DiskStats
	AllocDirStats    *DiskStats
	DeviceStats      []*DeviceGroupStats
	Uptime           uint64
	Timestamp        int64
	CPUTicksConsumed float64
}

// MemoryStats represents stats related to virtual memory usage
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

// DiskStats represents stats related to disk usage
type DiskStats struct {
	Device            string
	Mountpoint        string
	Size              uint64
	Used              uint64
	Available         uint64
	UsedPercent       float64
	InodesUsedPercent float64
}

// DeviceGroupStats represents stats related to device group
type DeviceGroupStats = device.DeviceGroupStats

// DeviceStatsCollector is used to retrieve all the latest statistics for all devices.
type DeviceStatsCollector func() []*DeviceGroupStats

// NodeStatsCollector is an interface which is used for the purposes of mocking
// the HostStatsCollector in the tests
type NodeStatsCollector interface {
	Collect() error
	Stats() *HostStats
}

// HostStatsCollector collects host resource usage stats
type HostStatsCollector struct {
	numCores             int
	statsCalculator      map[string]*HostCpuStatsCalculator
	hostStats            *HostStats
	hostStatsLock        sync.RWMutex
	allocDir             string
	deviceStatsCollector DeviceStatsCollector

	// badParts is a set of partitions whose usage cannot be read; used to
	// squelch logspam.
	badParts map[string]struct{}

	logger hclog.Logger
}

// NewHostStatsCollector returns a HostStatsCollector. The allocDir is passed in
// so that we can present the disk related statistics for the mountpoint where
// the allocation directory lives
func NewHostStatsCollector(logger hclog.Logger, allocDir string, deviceStatsCollector DeviceStatsCollector) *HostStatsCollector {
	logger = logger.Named("host_stats")
	numCores := runtime.NumCPU()
	statsCalculator := make(map[string]*HostCpuStatsCalculator)
	collector := &HostStatsCollector{
		statsCalculator:      statsCalculator,
		numCores:             numCores,
		logger:               logger,
		allocDir:             allocDir,
		badParts:             make(map[string]struct{}),
		deviceStatsCollector: deviceStatsCollector,
	}
	return collector
}

// Collect collects stats related to resource usage of a host
func (h *HostStatsCollector) Collect() error {
	h.hostStatsLock.Lock()
	defer h.hostStatsLock.Unlock()
	return h.collectLocked()
}

// collectLocked collects stats related to resource usage of the host but should
// be called with the lock held.
func (h *HostStatsCollector) collectLocked() error {
	hs := &HostStats{Timestamp: time.Now().UTC().UnixNano()}

	// Determine up-time
	uptime, err := host.Uptime()
	if err != nil {
		h.logger.Error("failed to collect upstime stats", "error", err)
		uptime = 0
	}
	hs.Uptime = uptime

	// Collect memory stats
	mstats, err := h.collectMemoryStats()
	if err != nil {
		h.logger.Error("failed to collect memory stats", "error", err)
		mstats = &MemoryStats{}
	}
	hs.Memory = mstats

	// Collect cpu stats
	cpus, ticks, err := h.collectCPUStats()
	if err != nil {
		h.logger.Error("failed to collect cpu stats", "error", err)
		cpus = []*CPUStats{}
		ticks = 0
	}
	hs.CPU = cpus
	hs.CPUTicksConsumed = ticks

	// Collect disk stats
	diskStats, err := h.collectDiskStats()
	if err != nil {
		h.logger.Error("failed to collect disk stats", "error", err)
		hs.DiskStats = []*DiskStats{}
	}
	hs.DiskStats = diskStats

	// Getting the disk stats for the allocation directory
	usage, err := disk.Usage(h.allocDir)
	if err != nil {
		h.logger.Error("failed to find disk usage of alloc", "alloc_dir", h.allocDir, "error", err)
		hs.AllocDirStats = &DiskStats{}
	} else {
		hs.AllocDirStats = h.toDiskStats(usage, nil)
	}
	// Collect devices stats
	deviceStats := h.collectDeviceGroupStats()
	hs.DeviceStats = deviceStats

	// Update the collected status object.
	h.hostStats = hs

	return nil
}

func (h *HostStatsCollector) collectMemoryStats() (*MemoryStats, error) {
	memStats, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	mem := &MemoryStats{
		Total:     memStats.Total,
		Available: memStats.Available,
		Used:      memStats.Used,
		Free:      memStats.Free,
	}

	return mem, nil
}

func (h *HostStatsCollector) collectDiskStats() ([]*DiskStats, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}

	var diskStats []*DiskStats
	for _, partition := range partitions {
		usage, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			if _, ok := h.badParts[partition.Mountpoint]; ok {
				// already known bad, don't log again
				continue
			}

			h.badParts[partition.Mountpoint] = struct{}{}
			h.logger.Warn("error fetching host disk usage stats", "error", err, "partition", partition.Mountpoint)
			continue
		}
		delete(h.badParts, partition.Mountpoint)

		ds := h.toDiskStats(usage, &partition)
		diskStats = append(diskStats, ds)
	}

	return diskStats, nil
}

func (h *HostStatsCollector) collectDeviceGroupStats() []*DeviceGroupStats {
	if h.deviceStatsCollector == nil {
		return []*DeviceGroupStats{}
	}

	return h.deviceStatsCollector()
}

// Stats returns the host stats that has been collected
func (h *HostStatsCollector) Stats() *HostStats {
	h.hostStatsLock.RLock()
	defer h.hostStatsLock.RUnlock()

	if h.hostStats == nil {
		if err := h.collectLocked(); err != nil {
			h.logger.Warn("error fetching host resource usage stats", "error", err)
		}
	}

	return h.hostStats
}

// toDiskStats merges UsageStat and PartitionStat to create a DiskStat
func (h *HostStatsCollector) toDiskStats(usage *disk.UsageStat, partitionStat *disk.PartitionStat) *DiskStats {
	ds := DiskStats{
		Size:              usage.Total,
		Used:              usage.Used,
		Available:         usage.Free,
		UsedPercent:       usage.UsedPercent,
		InodesUsedPercent: usage.InodesUsedPercent,
	}
	if math.IsNaN(ds.UsedPercent) {
		ds.UsedPercent = 0.0
	}
	if math.IsNaN(ds.InodesUsedPercent) {
		ds.InodesUsedPercent = 0.0
	}

	if partitionStat != nil {
		ds.Device = partitionStat.Device
		ds.Mountpoint = partitionStat.Mountpoint
	}

	return &ds
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
	currentBusy := times.User + times.System + times.Nice + times.Iowait + times.Irq +
		times.Softirq + times.Steal + times.Guest + times.GuestNice

	deltaTotal := currentTotal - h.prevTotal
	idle = ((currentIdle - h.prevIdle) / deltaTotal) * 100
	user = ((currentUser - h.prevUser) / deltaTotal) * 100
	system = ((currentSystem - h.prevSystem) / deltaTotal) * 100
	total = ((currentBusy - h.prevBusy) / deltaTotal) * 100

	// Protect against any invalid values
	if math.IsNaN(idle) || math.IsInf(idle, 0) {
		idle = 100.0
	}
	if math.IsNaN(user) || math.IsInf(user, 0) {
		user = 0.0
	}
	if math.IsNaN(system) || math.IsInf(system, 0) {
		system = 0.0
	}
	if math.IsNaN(total) || math.IsInf(total, 0) {
		total = 0.0
	}

	h.prevIdle = currentIdle
	h.prevUser = currentUser
	h.prevSystem = currentSystem
	h.prevTotal = currentTotal
	h.prevBusy = currentBusy
	return
}
