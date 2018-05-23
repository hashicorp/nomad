package stats

import (
	"fmt"
	"log"
	"math"
	"runtime"
	"sync"
	"time"

	"github.com/mindprince/gonvml"
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
	NvidiaStats      []*NvidiaStats
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

// NvidiaStats represents stats related to Nvidia Usage
type NvidiaStats struct {
	UUID                  string
	PowerUsage            uint
	AveragePowerUsage     uint
	AverageGPUUtilization uint
	GPUUtilization        uint
	MemoryUtilization     uint
	Temperature           uint
	UsedMemoryMiB         uint64
}

// NodeStatsCollector is an interface which is used for the purposes of mocking
// the HostStatsCollector in the tests
type NodeStatsCollector interface {
	Collect() error
	Stats() *HostStats
}

// HostStatsCollector collects host resource usage stats
type HostStatsCollector struct {
	numCores        int
	statsCalculator map[string]*HostCpuStatsCalculator
	logger          *log.Logger
	hostStats       *HostStats
	hostStatsLock   sync.RWMutex
	allocDir        string

	// badParts is a set of partitions whose usage cannot be read; used to
	// squelch logspam.
	badParts map[string]struct{}
}

// NewHostStatsCollector returns a HostStatsCollector. The allocDir is passed in
// so that we can present the disk related statistics for the mountpoint where
// the allocation directory lives
func NewHostStatsCollector(logger *log.Logger, allocDir string) *HostStatsCollector {
	numCores := runtime.NumCPU()
	statsCalculator := make(map[string]*HostCpuStatsCalculator)
	collector := &HostStatsCollector{
		statsCalculator: statsCalculator,
		numCores:        numCores,
		logger:          logger,
		allocDir:        allocDir,
		badParts:        make(map[string]struct{}),
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
		return err
	}
	hs.Uptime = uptime

	// Collect memory stats
	mstats, err := h.collectMemoryStats()
	if err != nil {
		return err
	}
	hs.Memory = mstats

	// Collect cpu stats
	cpus, ticks, err := h.collectCPUStats()
	if err != nil {
		return err
	}
	hs.CPU = cpus
	hs.CPUTicksConsumed = ticks

	// Collect disk stats
	diskStats, err := h.collectDiskStats()
	if err != nil {
		return err
	}
	hs.DiskStats = diskStats

	// Getting the disk stats for the allocation directory
	usage, err := disk.Usage(h.allocDir)
	if err != nil {
		return fmt.Errorf("failed to find disk usage of alloc_dir %q: %v", h.allocDir, err)
	}
	hs.AllocDirStats = h.toDiskStats(usage, nil)

	// Getting Nvidia stats
	// TODO(oleksii-shyman). there is no config available in stats collector
	// flag "RestrictNvidia" has to be used here
	nvidiaStats, err := h.collectNvidiaStats()
	if err != nil {
		return fmt.Errorf("failed to get nvidia stats with error '%v'", err)
	}
	hs.NvidiaStats = nvidiaStats
	// Update the collected status object.
	h.hostStats = hs

	return nil
}

func (h *HostStatsCollector) collectNvidiaStats() ([]*NvidiaStats, error) {
	/*
		TODO(oleksii-shyman) add nvml function calls to each of this rows
		nvml fields to be reported to stats api
		1  - Used Memory
		2  - Utilization of GPU
		3  - Utilization of Memory
		4  - Current GPU Temperature
		5  - Power Draw
		6  - Average Power Usage
		7  - Average GPU Utilization
		8  - UUID


		nvml fields that would be great to report, but they are not supported by mindprince/gonvml yet
		1  - GPU Operation Mode
		2  - Performance State
		3  - Compute Mode (indicates whether individual or multiple compute applications may run on the GPU.)
		4  - GPU Slowdown Temperature
		5  - GPU Shutdown Temperature
		6  - Power limit
		7  - Graphics clocks
		8  - Max Graphics clocks
		9  - SM clocks
		10 - Max SM clocks
		11 - Memory Clocks
		12 - Max Memory Clocks
		13 - Video Clocks
		14 - Max Video Clocks
		15 - Average Power Usage
	*/
	err := gonvml.Initialize()
	if err != nil {
		// There was an error during initialization, this node would not report
		// any functioning GPUs
		h.logger.Printf("[DEBUG] failed to initialize gonvml to get GPU data with error '%v'", err)
		return nil, nil
	}
	defer gonvml.Shutdown()

	numDevices, err := gonvml.DeviceCount()
	if err != nil {
		return nil, fmt.Errorf("nvidia nvml DeviceCount() error: %v\n", err)
	}

	allNvidiaGPUStats := make([]*NvidiaStats, numDevices)
	// TODO(oleksii-shyman) move timeInterval to config
	timeInterval := 10 * time.Second

	for i := 0; i < int(numDevices); i++ {
		dev, err := gonvml.DeviceHandleByIndex(uint(i))
		if err != nil {
			return allNvidiaGPUStats, fmt.Errorf("nvidia nvml DeviceHandleByIndex() error: %v\n", err)
		}

		uuid, err := dev.UUID()
		if err != nil {
			return allNvidiaGPUStats, fmt.Errorf("nvidia nvml dev.UUID() error: %v\n", err)
		}

		powerUsage, err := dev.PowerUsage()
		if err != nil {
			return allNvidiaGPUStats, fmt.Errorf("nvidia nvml dev.PowerUsage() error: %v\n", err)
		}

		averagePowerUsage, err := dev.AveragePowerUsage(timeInterval)
		if err != nil {
			return allNvidiaGPUStats, fmt.Errorf("nvidia nvml dev.AveragePowerUsage() error: %v\n", err)
		}

		averageGpuUtilization, err := dev.AverageGPUUtilization(timeInterval)
		if err != nil {
			return allNvidiaGPUStats, fmt.Errorf("nvidia nvml dev.AverageGPUUtilization() error: %v\n", err)
		}

		utilizationGpu, utilizationMemory, err := dev.UtilizationRates()
		if err != nil {
			return allNvidiaGPUStats, fmt.Errorf("nvidia nvml dev.UtilizationRates() error: %v\n", err)
		}

		temperature, err := dev.Temperature()
		if err != nil {
			return allNvidiaGPUStats, fmt.Errorf("nvidia nvml dev.Temperature() error: %v\n", err)
		}

		_, usedMemory, err := dev.MemoryInfo()
		if err != nil {
			return allNvidiaGPUStats, fmt.Errorf("nvidia nvml dev.MemoryInfo() error: %v\n", err)
		}

		allNvidiaGPUStats[i] = &NvidiaStats{
			UUID:                  uuid,
			PowerUsage:            powerUsage,
			AveragePowerUsage:     averagePowerUsage,
			AverageGPUUtilization: averageGpuUtilization,
			GPUUtilization:        utilizationGpu,
			MemoryUtilization:     utilizationMemory,
			Temperature:           temperature,
			// usedMemory returns amount in bytes
			// to convert in mebibytes -> we need to divide it to 2**20
			UsedMemoryMiB: usedMemory / 1024 / 1024,
		}
	}
	return allNvidiaGPUStats, nil
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
			h.logger.Printf("[WARN] client: error fetching host disk usage stats for %v: %v", partition.Mountpoint, err)
			continue
		}
		delete(h.badParts, partition.Mountpoint)

		ds := h.toDiskStats(usage, &partition)
		diskStats = append(diskStats, ds)
	}

	return diskStats, nil
}

// Stats returns the host stats that has been collected
func (h *HostStatsCollector) Stats() *HostStats {
	h.hostStatsLock.RLock()
	defer h.hostStatsLock.RUnlock()

	if h.hostStats == nil {
		if err := h.collectLocked(); err != nil {
			h.logger.Printf("[WARN] client: error fetching host resource usage stats: %v", err)
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
		times.Softirq + times.Steal + times.Guest + times.GuestNice + times.Stolen

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
