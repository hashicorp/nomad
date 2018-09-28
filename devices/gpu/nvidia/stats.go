package nvidia

import (
	"context"
	"time"

	"github.com/hashicorp/nomad/devices/gpu/nvidia/nvml"
	"github.com/hashicorp/nomad/plugins/device"
)

const (
	// Attribute names for reporting stats output
	PowerUsageAttr = "Power usage"
	PowerUsageUnit = "W"
	PowerUsageDesc = "Power usage for this GPU in watts and " +
		"its associated circuitry (e.g. memory) / Maximum GPU Power"
	GPUUtilizationAttr = "GPU utilization"
	GPUUtilizationUnit = "%"
	GPUUtilizationDesc = "Percent of time over the past sample period " +
		"during which one or more kernels were executing on the GPU."
	MemoryUtilizationAttr  = "Memory utilization"
	MemoryUtilizationUnit  = "%"
	MemoryUtilizationDesc  = "Percentage of bandwidth used during the past sample period"
	EncoderUtilizationAttr = "Encoder utilization"
	EncoderUtilizationUnit = "%"
	EncoderUtilizationDesc = "Percent of time over the past sample period " +
		"during which GPU Encoder was used"
	DecoderUtilizationAttr = "Decoder utilization"
	DecoderUtilizationUnit = "%"
	DecoderUtilizationDesc = "Percent of time over the past sample period " +
		"during which GPU Decoder was used"
	TemperatureAttr      = "Temperature"
	TemperatureUnit      = "C" // Celsius degrees
	TemperatureDesc      = "Temperature of the Unit"
	MemoryStateAttr      = "Memory state"
	MemoryStateUnit      = "MiB" // Mebibytes
	MemoryStateDesc      = "UsedMemory / TotalMemory"
	BAR1StateAttr        = "BAR1 buffer state"
	BAR1StateUnit        = "MiB" // Mebibytes
	BAR1StateDesc        = "UsedBAR1 / TotalBAR1"
	ECCErrorsL1CacheAttr = "ECC L1 errors"
	ECCErrorsL1CacheUnit = "#" // number of errors
	ECCErrorsL1CacheDesc = "Requested L1Cache error counter for the device"
	ECCErrorsL2CacheAttr = "ECC L2 errors"
	ECCErrorsL2CacheUnit = "#" // number of errors
	ECCErrorsL2CacheDesc = "Requested L2Cache error counter for the device"
	ECCErrorsDeviceAttr  = "ECC memory errors"
	ECCErrorsDeviceUnit  = "#" // number of errors
	ECCErrorsDeviceDesc  = "Requested memory error counter for the device"
)

// stats is the long running goroutine that streams device statistics
func (d *NvidiaDevice) stats(ctx context.Context, stats chan<- *device.StatsResponse, interval time.Duration) {
	defer close(stats)

	if d.initErr != nil {
		if d.initErr.Error() != nvml.UnavailableLib.Error() {
			d.logger.Error("exiting stats due to problems with NVML loading", "error", d.initErr)
			stats <- device.NewStatsError(d.initErr)
		}

		return
	}

	// Create a timer that will fire immediately for the first detection
	ticker := time.NewTimer(0)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ticker.Reset(interval)
		}

		d.writeStatsToChannel(stats, time.Now())
	}
}

// filterStatsByID accepts list of StatsData and set of IDs
// this function would return entries from StatsData with IDs found in the set
func filterStatsByID(stats []*nvml.StatsData, IDs map[string]struct{}) []*nvml.StatsData {
	var filteredStats []*nvml.StatsData
	for _, statsItem := range stats {
		if _, ok := IDs[statsItem.UUID]; ok {
			filteredStats = append(filteredStats, statsItem)
		}
	}
	return filteredStats
}

// writeStatsToChannel collects StatsData from NVML backend, groups StatsData
// by DeviceName attribute, populates DeviceGroupStats structure for every group
// and sends data over provided channel
func (d *NvidiaDevice) writeStatsToChannel(stats chan<- *device.StatsResponse, timestamp time.Time) {
	statsData, err := d.nvmlClient.GetStatsData()
	if err != nil {
		d.logger.Error("failed to get nvidia stats", "error", err)
		stats <- &device.StatsResponse{
			Error: err,
		}
		return
	}

	// filter only stats from devices that are stored in NvidiaDevice struct
	d.deviceLock.RLock()
	statsData = filterStatsByID(statsData, d.devices)
	d.deviceLock.RUnlock()

	// group stats by DeviceName struct field
	statsListByDeviceName := make(map[string][]*nvml.StatsData)
	for _, statsItem := range statsData {
		deviceName := statsItem.DeviceName
		if deviceName == nil {
			// nvml driver was not able to detect device name. This kind
			// of devices are placed to single group with 'notAvailable' name
			notAvailableCopy := notAvailable
			deviceName = &notAvailableCopy
		}

		statsListByDeviceName[*deviceName] = append(statsListByDeviceName[*deviceName], statsItem)
	}

	// place data device.DeviceGroupStats struct for every group of stats
	deviceGroupsStats := make([]*device.DeviceGroupStats, 0, len(statsListByDeviceName))
	for groupName, groupStats := range statsListByDeviceName {
		deviceGroupsStats = append(deviceGroupsStats, statsForGroup(groupName, groupStats, timestamp))
	}

	stats <- &device.StatsResponse{
		Groups: deviceGroupsStats,
	}
}

func newNotAvailableDeviceStats(unit, desc string) *device.StatValue {
	return &device.StatValue{Unit: unit, Desc: desc, StringVal: notAvailable}
}

// statsForGroup is a helper function that populates device.DeviceGroupStats
// for given groupName with groupStats list
func statsForGroup(groupName string, groupStats []*nvml.StatsData, timestamp time.Time) *device.DeviceGroupStats {
	instanceStats := make(map[string]*device.DeviceStats)
	for _, statsItem := range groupStats {
		instanceStats[statsItem.UUID] = statsForItem(statsItem, timestamp)
	}

	return &device.DeviceGroupStats{
		Vendor:        vendor,
		Type:          deviceType,
		Name:          groupName,
		InstanceStats: instanceStats,
	}
}

// statsForItem is a helper function that populates device.DeviceStats for given
// nvml.StatsData
func statsForItem(statsItem *nvml.StatsData, timestamp time.Time) *device.DeviceStats {
	// nvml.StatsData holds pointers to values that can be nil
	// In case they are nil return stats with 'notAvailable' constant
	var (
		powerUsageStat         *device.StatValue
		GPUUtilizationStat     *device.StatValue
		memoryUtilizationStat  *device.StatValue
		encoderUtilizationStat *device.StatValue
		decoderUtilizationStat *device.StatValue
		temperatureStat        *device.StatValue
		memoryStateStat        *device.StatValue
		BAR1StateStat          *device.StatValue
		ECCErrorsL1CacheStat   *device.StatValue
		ECCErrorsL2CacheStat   *device.StatValue
		ECCErrorsDeviceStat    *device.StatValue
	)

	if statsItem.PowerUsageW == nil || statsItem.PowerW == nil {
		powerUsageStat = newNotAvailableDeviceStats(PowerUsageUnit, PowerUsageDesc)
	} else {
		powerUsageStat = &device.StatValue{
			Unit:              PowerUsageUnit,
			Desc:              PowerUsageDesc,
			IntNumeratorVal:   int64(*statsItem.PowerUsageW),
			IntDenominatorVal: int64(*statsItem.PowerW),
		}
	}

	if statsItem.GPUUtilization == nil {
		GPUUtilizationStat = newNotAvailableDeviceStats(GPUUtilizationUnit, GPUUtilizationDesc)
	} else {
		GPUUtilizationStat = &device.StatValue{
			Unit:            GPUUtilizationUnit,
			Desc:            GPUUtilizationDesc,
			IntNumeratorVal: int64(*statsItem.GPUUtilization),
		}
	}

	if statsItem.MemoryUtilization == nil {
		memoryUtilizationStat = newNotAvailableDeviceStats(MemoryUtilizationUnit, MemoryUtilizationDesc)
	} else {
		memoryUtilizationStat = &device.StatValue{
			Unit:            MemoryUtilizationUnit,
			Desc:            MemoryUtilizationDesc,
			IntNumeratorVal: int64(*statsItem.MemoryUtilization),
		}
	}

	if statsItem.EncoderUtilization == nil {
		encoderUtilizationStat = newNotAvailableDeviceStats(EncoderUtilizationUnit, EncoderUtilizationDesc)
	} else {
		encoderUtilizationStat = &device.StatValue{
			Unit:            EncoderUtilizationUnit,
			Desc:            EncoderUtilizationDesc,
			IntNumeratorVal: int64(*statsItem.EncoderUtilization),
		}
	}

	if statsItem.DecoderUtilization == nil {
		decoderUtilizationStat = newNotAvailableDeviceStats(DecoderUtilizationUnit, DecoderUtilizationDesc)
	} else {
		decoderUtilizationStat = &device.StatValue{
			Unit:            DecoderUtilizationUnit,
			Desc:            DecoderUtilizationDesc,
			IntNumeratorVal: int64(*statsItem.DecoderUtilization),
		}
	}

	if statsItem.TemperatureC == nil {
		temperatureStat = newNotAvailableDeviceStats(TemperatureUnit, TemperatureDesc)
	} else {
		temperatureStat = &device.StatValue{
			Unit:            TemperatureUnit,
			Desc:            TemperatureDesc,
			IntNumeratorVal: int64(*statsItem.TemperatureC),
		}
	}

	if statsItem.UsedMemoryMiB == nil || statsItem.MemoryMiB == nil {
		memoryStateStat = newNotAvailableDeviceStats(MemoryStateUnit, MemoryStateDesc)
	} else {
		memoryStateStat = &device.StatValue{
			Unit:              MemoryStateUnit,
			Desc:              MemoryStateDesc,
			IntNumeratorVal:   int64(*statsItem.UsedMemoryMiB),
			IntDenominatorVal: int64(*statsItem.MemoryMiB),
		}
	}

	if statsItem.BAR1UsedMiB == nil || statsItem.BAR1MiB == nil {
		BAR1StateStat = newNotAvailableDeviceStats(BAR1StateUnit, BAR1StateDesc)
	} else {
		BAR1StateStat = &device.StatValue{
			Unit:              BAR1StateUnit,
			Desc:              BAR1StateDesc,
			IntNumeratorVal:   int64(*statsItem.BAR1UsedMiB),
			IntDenominatorVal: int64(*statsItem.BAR1MiB),
		}
	}

	if statsItem.ECCErrorsL1Cache == nil {
		ECCErrorsL1CacheStat = newNotAvailableDeviceStats(ECCErrorsL1CacheUnit, ECCErrorsL1CacheDesc)
	} else {
		ECCErrorsL1CacheStat = &device.StatValue{
			Unit:            ECCErrorsL1CacheUnit,
			Desc:            ECCErrorsL1CacheDesc,
			IntNumeratorVal: int64(*statsItem.ECCErrorsL1Cache),
		}
	}

	if statsItem.ECCErrorsL2Cache == nil {
		ECCErrorsL2CacheStat = newNotAvailableDeviceStats(ECCErrorsL2CacheUnit, ECCErrorsL2CacheDesc)
	} else {
		ECCErrorsL2CacheStat = &device.StatValue{
			Unit:            ECCErrorsL2CacheUnit,
			Desc:            ECCErrorsL2CacheDesc,
			IntNumeratorVal: int64(*statsItem.ECCErrorsL2Cache),
		}
	}

	if statsItem.ECCErrorsDevice == nil {
		ECCErrorsDeviceStat = newNotAvailableDeviceStats(ECCErrorsDeviceUnit, ECCErrorsDeviceDesc)
	} else {
		ECCErrorsDeviceStat = &device.StatValue{
			Unit:            ECCErrorsDeviceUnit,
			Desc:            ECCErrorsDeviceDesc,
			IntNumeratorVal: int64(*statsItem.ECCErrorsDevice),
		}
	}
	return &device.DeviceStats{
		Summary: temperatureStat,
		Stats: &device.StatObject{
			Attributes: map[string]*device.StatValue{
				PowerUsageAttr:         powerUsageStat,
				GPUUtilizationAttr:     GPUUtilizationStat,
				MemoryUtilizationAttr:  memoryUtilizationStat,
				EncoderUtilizationAttr: encoderUtilizationStat,
				DecoderUtilizationAttr: decoderUtilizationStat,
				TemperatureAttr:        temperatureStat,
				MemoryStateAttr:        memoryStateStat,
				BAR1StateAttr:          BAR1StateStat,
				ECCErrorsL1CacheAttr:   ECCErrorsL1CacheStat,
				ECCErrorsL2CacheAttr:   ECCErrorsL2CacheStat,
				ECCErrorsDeviceAttr:    ECCErrorsDeviceStat,
			},
		},
		Timestamp: timestamp,
	}
}
