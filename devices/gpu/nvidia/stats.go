package nvidia

import (
	"context"
	"time"

	"github.com/hashicorp/nomad/devices/gpu/nvidia/nvml"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/shared/structs"
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

func newNotAvailableDeviceStats(unit, desc string) *structs.StatValue {
	return &structs.StatValue{Unit: unit, Desc: desc, StringVal: helper.StringToPtr(notAvailable)}
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
		powerUsageStat         *structs.StatValue
		GPUUtilizationStat     *structs.StatValue
		memoryUtilizationStat  *structs.StatValue
		encoderUtilizationStat *structs.StatValue
		decoderUtilizationStat *structs.StatValue
		temperatureStat        *structs.StatValue
		memoryStateStat        *structs.StatValue
		BAR1StateStat          *structs.StatValue
		ECCErrorsL1CacheStat   *structs.StatValue
		ECCErrorsL2CacheStat   *structs.StatValue
		ECCErrorsDeviceStat    *structs.StatValue
	)

	if statsItem.PowerUsageW == nil || statsItem.PowerW == nil {
		powerUsageStat = newNotAvailableDeviceStats(PowerUsageUnit, PowerUsageDesc)
	} else {
		powerUsageStat = &structs.StatValue{
			Unit:              PowerUsageUnit,
			Desc:              PowerUsageDesc,
			IntNumeratorVal:   helper.Int64ToPtr(int64(*statsItem.PowerUsageW)),
			IntDenominatorVal: uintToInt64Ptr(statsItem.PowerW),
		}
	}

	if statsItem.GPUUtilization == nil {
		GPUUtilizationStat = newNotAvailableDeviceStats(GPUUtilizationUnit, GPUUtilizationDesc)
	} else {
		GPUUtilizationStat = &structs.StatValue{
			Unit:            GPUUtilizationUnit,
			Desc:            GPUUtilizationDesc,
			IntNumeratorVal: uintToInt64Ptr(statsItem.GPUUtilization),
		}
	}

	if statsItem.MemoryUtilization == nil {
		memoryUtilizationStat = newNotAvailableDeviceStats(MemoryUtilizationUnit, MemoryUtilizationDesc)
	} else {
		memoryUtilizationStat = &structs.StatValue{
			Unit:            MemoryUtilizationUnit,
			Desc:            MemoryUtilizationDesc,
			IntNumeratorVal: uintToInt64Ptr(statsItem.MemoryUtilization),
		}
	}

	if statsItem.EncoderUtilization == nil {
		encoderUtilizationStat = newNotAvailableDeviceStats(EncoderUtilizationUnit, EncoderUtilizationDesc)
	} else {
		encoderUtilizationStat = &structs.StatValue{
			Unit:            EncoderUtilizationUnit,
			Desc:            EncoderUtilizationDesc,
			IntNumeratorVal: uintToInt64Ptr(statsItem.EncoderUtilization),
		}
	}

	if statsItem.DecoderUtilization == nil {
		decoderUtilizationStat = newNotAvailableDeviceStats(DecoderUtilizationUnit, DecoderUtilizationDesc)
	} else {
		decoderUtilizationStat = &structs.StatValue{
			Unit:            DecoderUtilizationUnit,
			Desc:            DecoderUtilizationDesc,
			IntNumeratorVal: uintToInt64Ptr(statsItem.DecoderUtilization),
		}
	}

	if statsItem.TemperatureC == nil {
		temperatureStat = newNotAvailableDeviceStats(TemperatureUnit, TemperatureDesc)
	} else {
		temperatureStat = &structs.StatValue{
			Unit:            TemperatureUnit,
			Desc:            TemperatureDesc,
			IntNumeratorVal: uintToInt64Ptr(statsItem.TemperatureC),
		}
	}

	if statsItem.UsedMemoryMiB == nil || statsItem.MemoryMiB == nil {
		memoryStateStat = newNotAvailableDeviceStats(MemoryStateUnit, MemoryStateDesc)
	} else {
		memoryStateStat = &structs.StatValue{
			Unit:              MemoryStateUnit,
			Desc:              MemoryStateDesc,
			IntNumeratorVal:   uint64ToInt64Ptr(statsItem.UsedMemoryMiB),
			IntDenominatorVal: uint64ToInt64Ptr(statsItem.MemoryMiB),
		}
	}

	if statsItem.BAR1UsedMiB == nil || statsItem.BAR1MiB == nil {
		BAR1StateStat = newNotAvailableDeviceStats(BAR1StateUnit, BAR1StateDesc)
	} else {
		BAR1StateStat = &structs.StatValue{
			Unit:              BAR1StateUnit,
			Desc:              BAR1StateDesc,
			IntNumeratorVal:   uint64ToInt64Ptr(statsItem.BAR1UsedMiB),
			IntDenominatorVal: uint64ToInt64Ptr(statsItem.BAR1MiB),
		}
	}

	if statsItem.ECCErrorsL1Cache == nil {
		ECCErrorsL1CacheStat = newNotAvailableDeviceStats(ECCErrorsL1CacheUnit, ECCErrorsL1CacheDesc)
	} else {
		ECCErrorsL1CacheStat = &structs.StatValue{
			Unit:            ECCErrorsL1CacheUnit,
			Desc:            ECCErrorsL1CacheDesc,
			IntNumeratorVal: uint64ToInt64Ptr(statsItem.ECCErrorsL1Cache),
		}
	}

	if statsItem.ECCErrorsL2Cache == nil {
		ECCErrorsL2CacheStat = newNotAvailableDeviceStats(ECCErrorsL2CacheUnit, ECCErrorsL2CacheDesc)
	} else {
		ECCErrorsL2CacheStat = &structs.StatValue{
			Unit:            ECCErrorsL2CacheUnit,
			Desc:            ECCErrorsL2CacheDesc,
			IntNumeratorVal: uint64ToInt64Ptr(statsItem.ECCErrorsL2Cache),
		}
	}

	if statsItem.ECCErrorsDevice == nil {
		ECCErrorsDeviceStat = newNotAvailableDeviceStats(ECCErrorsDeviceUnit, ECCErrorsDeviceDesc)
	} else {
		ECCErrorsDeviceStat = &structs.StatValue{
			Unit:            ECCErrorsDeviceUnit,
			Desc:            ECCErrorsDeviceDesc,
			IntNumeratorVal: uint64ToInt64Ptr(statsItem.ECCErrorsDevice),
		}
	}
	return &device.DeviceStats{
		Summary: memoryStateStat,
		Stats: &structs.StatObject{
			Attributes: map[string]*structs.StatValue{
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

func uintToInt64Ptr(u *uint) *int64 {
	if u == nil {
		return nil
	}

	v := int64(*u)
	return &v
}

func uint64ToInt64Ptr(u *uint64) *int64 {
	if u == nil {
		return nil
	}

	v := int64(*u)
	return &v
}
