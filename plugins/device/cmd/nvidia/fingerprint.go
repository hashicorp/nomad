package nvidia

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/device/cmd/nvidia/nvml"
)

const (
	// Attribute names for reporting Fingerprint output
	MemoryMiBAttr          = "memory_mib"
	PowerWAttr             = "power_w"
	BAR1MiBAttr            = "bar1_mib"
	DriverVersionAttr      = "driver_version"
	CoresClockMHzAttr      = "cores_clock_mhz"
	MemoryClockMHzAttr     = "memory_clock_mhz"
	PCIBandwidthMBPerSAttr = "pci_bandwidth_mb/s"
	DisplayStateAttr       = "display_state"
	PersistenceModeAttr    = "persistence_mode"
)

// fingerprint is the long running goroutine that detects hardware
func (d *NvidiaDevice) fingerprint(ctx context.Context, devices chan<- *device.FingerprintResponse) {
	defer close(devices)

	if d.nvmlClientInitializationError != nil {
		d.logger.Error("exiting fingerprinting due to problems with NVML loading", "error", d.nvmlClientInitializationError)
		// write empty fingerprint response to let server know that there are
		// no working Nvidia GPU units
		devices <- device.NewFingerprint()
		return
	}

	// Create a timer that will fire immediately for the first detection
	ticker := time.NewTimer(0)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ticker.Reset(d.fingerprintPeriod)
		}
		d.writeFingerprintToChannel(devices)
	}
}

// writeFingerprintToChannel makes nvml call and writes response to channel
func (d *NvidiaDevice) writeFingerprintToChannel(devices chan<- *device.FingerprintResponse) {
	fingerprintData, err := d.nvmlClient.GetFingerprintData()

	if err != nil {
		d.logger.Error("failed to get fingerprint nvidia devices", "error", err)
		devices <- device.NewFingerprintError(err)
		return
	}

	// ignore devices from fingerprint output
	fingerprintDevices := ignoreFingerprintedDevices(fingerprintData.Devices, d.ignoredGPUIDs)
	// check if any device health was updated or any device was added to host
	if !d.fingerprintChanged(fingerprintDevices) {
		return
	}

	commonAttributes := map[string]string{
		DriverVersionAttr: fingerprintData.DriverVersion,
	}

	// Group all FingerprintDevices by DeviceName attribute
	deviceListByDeviceName := make(map[string][]*nvml.FingerprintDeviceData)
	for _, device := range fingerprintDevices {
		deviceName := device.DeviceName
		if deviceName == nil {
			// nvml driver was not able to detect device name. This kind
			// of devices are placed to single group with 'notAvailable' name
			notAvailableCopy := notAvailable
			deviceName = &notAvailableCopy
		}

		deviceListByDeviceName[*deviceName] = append(deviceListByDeviceName[*deviceName], device)
	}

	// Build Fingerprint response with computed groups and send it over the channel
	deviceGroups := make([]*device.DeviceGroup, 0, len(deviceListByDeviceName))
	for groupName, devices := range deviceListByDeviceName {
		deviceGroups = append(deviceGroups, deviceGroupFromFingerprintData(groupName, devices, commonAttributes))
	}
	devices <- device.NewFingerprint(deviceGroups...)
}

// ignoreFingerprintedDevices excludes ignored devices from fingerprint output
func ignoreFingerprintedDevices(deviceData []*nvml.FingerprintDeviceData, ignoredGPUIDs map[string]struct{}) []*nvml.FingerprintDeviceData {
	var result []*nvml.FingerprintDeviceData
	for _, fingerprintDevice := range deviceData {
		if _, ignored := ignoredGPUIDs[fingerprintDevice.UUID]; !ignored {
			result = append(result, fingerprintDevice)
		}
	}
	return result
}

// fingerprintChanged checks if there are any previously unseen nvidia devices located
// or any of fingerprinted nvidia devices disappeared since the last fingerprint run.
// Also, this func updates device map on NvidiaDevice with the latest data
func (d *NvidiaDevice) fingerprintChanged(allDevices []*nvml.FingerprintDeviceData) bool {
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()

	changeDetected := false
	// check if every device in allDevices is in d.devices
	for _, device := range allDevices {
		if _, ok := d.devices[device.UUID]; !ok {
			changeDetected = true
		}
	}

	// check if every device in d.devices is in allDevices
	fingerprintDeviceMap := make(map[string]struct{})
	for _, device := range allDevices {
		fingerprintDeviceMap[device.UUID] = struct{}{}
	}
	for id := range d.devices {
		if _, ok := fingerprintDeviceMap[id]; !ok {
			changeDetected = true
		}
	}

	d.devices = fingerprintDeviceMap
	return changeDetected
}

// deviceGroupFromFingerprintData composes deviceGroup from FingerprintDeviceData slice
func deviceGroupFromFingerprintData(groupName string, deviceList []*nvml.FingerprintDeviceData, commonAttributes map[string]string) *device.DeviceGroup {
	// deviceGroup without devices makes no sense -> return nil when no devices are provided
	if len(deviceList) == 0 {
		return nil
	}

	devices := make([]*device.Device, len(deviceList))
	for index, dev := range deviceList {
		devices[index] = &device.Device{
			ID: dev.UUID,
			// all fingerprinted devices are "healthy" for now
			// to get real health data -> dcgm bindings should be used
			Healthy: true,
			HwLocality: &device.DeviceLocality{
				PciBusID: dev.PCIBusID,
			},
		}
	}

	deviceGroup := &device.DeviceGroup{
		Vendor:  vendor,
		Type:    deviceType,
		Name:    groupName,
		Devices: devices,
		// Assumption made that devices with the same DeviceName have the same
		// attributes like amount of memory, power, bar1memory etc
		Attributes: attributesFromFingerprintDeviceData(deviceList[0]),
	}

	// Extend attribute map with common attributes
	for attributeKey, attributeValue := range commonAttributes {
		deviceGroup.Attributes[attributeKey] = attributeValue
	}

	return deviceGroup
}

// attributesFromFingerprintDeviceData converts nvml.FingerprintDeviceData
// struct to device.DeviceGroup.Attributes format (map[string]string)
// this function performs all nil checks for FingerprintDeviceData pointers
func attributesFromFingerprintDeviceData(fingerprintDeviceData *nvml.FingerprintDeviceData) map[string]string {
	// The following fields in FingerprintDeviceData are pointers, so they can be nil
	// In case they are nil -> return 'notAvailable' constant instead
	var (
		MemoryMiB          string
		PowerW             string
		BAR1MiB            string
		CoresClockMHz      string
		MemoryClockMHz     string
		PCIBandwidthMBPerS string
	)

	if fingerprintDeviceData.MemoryMiB == nil {
		MemoryMiB = notAvailable
	} else {
		MemoryMiB = fmt.Sprint(*fingerprintDeviceData.MemoryMiB)
	}

	if fingerprintDeviceData.PowerW == nil {
		PowerW = notAvailable
	} else {
		PowerW = fmt.Sprint(*fingerprintDeviceData.PowerW)
	}

	if fingerprintDeviceData.BAR1MiB == nil {
		BAR1MiB = notAvailable
	} else {
		BAR1MiB = fmt.Sprint(*fingerprintDeviceData.BAR1MiB)
	}

	if fingerprintDeviceData.CoresClockMHz == nil {
		CoresClockMHz = notAvailable
	} else {
		CoresClockMHz = fmt.Sprint(*fingerprintDeviceData.CoresClockMHz)
	}

	if fingerprintDeviceData.MemoryClockMHz == nil {
		MemoryClockMHz = notAvailable
	} else {
		MemoryClockMHz = fmt.Sprint(*fingerprintDeviceData.MemoryClockMHz)
	}

	if fingerprintDeviceData.PCIBandwidthMBPerS == nil {
		PCIBandwidthMBPerS = notAvailable
	} else {
		PCIBandwidthMBPerS = fmt.Sprint(*fingerprintDeviceData.PCIBandwidthMBPerS)
	}

	return map[string]string{
		DisplayStateAttr:       fingerprintDeviceData.DisplayState,
		PersistenceModeAttr:    fingerprintDeviceData.PersistenceMode,
		MemoryMiBAttr:          MemoryMiB,
		PowerWAttr:             PowerW,
		BAR1MiBAttr:            BAR1MiB,
		CoresClockMHzAttr:      CoresClockMHz,
		MemoryClockMHzAttr:     MemoryClockMHz,
		PCIBandwidthMBPerSAttr: PCIBandwidthMBPerS,
	}

}
