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
	// Attribute names and units for reporting Fingerprint output
	MemoryAttr          = "memory"
	PowerAttr           = "power"
	BAR1Attr            = "bar1"
	DriverVersionAttr   = "driver_version"
	CoresClockAttr      = "cores_clock"
	MemoryClockAttr     = "memory_clock"
	PCIBandwidthAttr    = "pci_bandwidth"
	DisplayStateAttr    = "display_state"
	PersistenceModeAttr = "persistence_mode"
)

// fingerprint is the long running goroutine that detects hardware
func (d *NvidiaDevice) fingerprint(ctx context.Context, devices chan<- *device.FingerprintResponse) {
	defer close(devices)

	if d.initErr != nil {
		if d.initErr.Error() != nvml.UnavailableLib.Error() {
			d.logger.Error("exiting fingerprinting due to problems with NVML loading", "error", d.initErr)
			devices <- device.NewFingerprintError(d.initErr)
		}

		// Just close the channel to let server know that there are no working
		// Nvidia GPU units
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

	commonAttributes := map[string]*structs.Attribute{
		DriverVersionAttr: {
			String: helper.StringToPtr(fingerprintData.DriverVersion),
		},
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
func deviceGroupFromFingerprintData(groupName string, deviceList []*nvml.FingerprintDeviceData, commonAttributes map[string]*structs.Attribute) *device.DeviceGroup {
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
func attributesFromFingerprintDeviceData(d *nvml.FingerprintDeviceData) map[string]*structs.Attribute {
	attrs := map[string]*structs.Attribute{
		DisplayStateAttr: {
			String: helper.StringToPtr(d.DisplayState),
		},
		PersistenceModeAttr: {
			String: helper.StringToPtr(d.PersistenceMode),
		},
	}

	if d.MemoryMiB != nil {
		attrs[MemoryAttr] = &structs.Attribute{
			Int:  helper.Int64ToPtr(int64(*d.MemoryMiB)),
			Unit: structs.UnitMiB,
		}
	}
	if d.PowerW != nil {
		attrs[PowerAttr] = &structs.Attribute{
			Int:  helper.Int64ToPtr(int64(*d.PowerW)),
			Unit: structs.UnitW,
		}
	}
	if d.BAR1MiB != nil {
		attrs[BAR1Attr] = &structs.Attribute{
			Int:  helper.Int64ToPtr(int64(*d.BAR1MiB)),
			Unit: structs.UnitMiB,
		}
	}
	if d.CoresClockMHz != nil {
		attrs[CoresClockAttr] = &structs.Attribute{
			Int:  helper.Int64ToPtr(int64(*d.CoresClockMHz)),
			Unit: structs.UnitMHz,
		}
	}
	if d.MemoryClockMHz != nil {
		attrs[MemoryClockAttr] = &structs.Attribute{
			Int:  helper.Int64ToPtr(int64(*d.MemoryClockMHz)),
			Unit: structs.UnitMHz,
		}
	}
	if d.PCIBandwidthMBPerS != nil {
		attrs[PCIBandwidthAttr] = &structs.Attribute{
			Int:  helper.Int64ToPtr(int64(*d.PCIBandwidthMBPerS)),
			Unit: structs.UnitMBPerS,
		}
	}

	return attrs
}
