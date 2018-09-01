package nvml

import (
	"fmt"
)

// DeviceData represents common fields for Nvidia device
type DeviceData struct {
	UUID       string
	DeviceName *string
	MemoryMiB  *uint64
	PowerW     *uint
	BAR1MiB    *uint64
}

// FingerprintDeviceData is a superset of DeviceData
// it describes device specific fields returned from
// nvml queries during fingerprinting call
type FingerprintDeviceData struct {
	*DeviceData
	PCIBandwidthMBPerS *uint
	CoresClockMHz      *uint
	MemoryClockMHz     *uint
	DisplayState       string
	PersistenceMode    string
	PCIBusID           string
}

// FingerprintData represets attributes of driver/devices
type FingerprintData struct {
	Devices       []*FingerprintDeviceData
	DriverVersion string
}

// NvmlClient describes how users would use nvml library
type NvmlClient interface {
	GetFingerprintData() (*FingerprintData, error)
}

// nvmlClient implements NvmlClient
// Users of this lib are expected to use this struct via NewNvmlClient func
type nvmlClient struct {
	driver NvmlDriver
}

// NewNvmlClient function creates new nvmlClient with real
// NvmlDriver implementation. Also, this func initializes NvmlDriver
func NewNvmlClient() (*nvmlClient, error) {
	driver := &nvmlDriver{}
	err := driver.Initialize()
	if err != nil {
		return nil, err
	}
	return &nvmlClient{
		driver: driver,
	}, nil
}

// GetFingerprintData returns FingerprintData for available Nvidia devices
func (c *nvmlClient) GetFingerprintData() (*FingerprintData, error) {
	/*
		nvml fields to be fingerprinted # nvml_library_call
		1  - Driver Version             # nvmlSystemGetDriverVersion
		2  - Product Name               # nvmlDeviceGetName
		3  - GPU UUID                   # nvmlDeviceGetUUID
		4  - Total Memory               # nvmlDeviceGetMemoryInfo
		5  - Power                      # nvmlDeviceGetPowerManagementLimit
		6  - PCIBusID                   # nvmlDeviceGetPciInfo
		7  - BAR1 Memory                # nvmlDeviceGetBAR1MemoryInfo(
		8  - PCI Bandwidth
		9  - Memory, Cores Clock        # nvmlDeviceGetMaxClockInfo
		10 - Display Mode               # nvmlDeviceGetDisplayMode
		11 - Persistence Mode           # nvmlDeviceGetPersistenceMode
	*/

	// Assumed that this method is called with receiver retrieved from
	// NewNvmlClient
	// because this method handles initialization of NVML library

	driverVersion, err := c.driver.SystemDriverVersion()
	if err != nil {
		return nil, fmt.Errorf("nvidia nvml SystemDriverVersion() error: %v\n", err)
	}

	numDevices, err := c.driver.DeviceCount()
	if err != nil {
		return nil, fmt.Errorf("nvidia nvml DeviceCount() error: %v\n", err)
	}

	allNvidiaGPUResources := make([]*FingerprintDeviceData, numDevices)

	for i := 0; i < int(numDevices); i++ {
		deviceInfo, err := c.driver.DeviceInfoByIndex(uint(i))
		if err != nil {
			return nil, fmt.Errorf("nvidia nvml DeviceInfoByIndex() error: %v\n", err)
		}

		allNvidiaGPUResources[i] = &FingerprintDeviceData{
			DeviceData: &DeviceData{
				DeviceName: deviceInfo.Name,
				UUID:       deviceInfo.UUID,
				MemoryMiB:  deviceInfo.MemoryMiB,
				PowerW:     deviceInfo.PowerW,
				BAR1MiB:    deviceInfo.BAR1MiB,
			},
			PCIBandwidthMBPerS: deviceInfo.PCIBandwidthMBPerS,
			CoresClockMHz:      deviceInfo.CoresClockMHz,
			MemoryClockMHz:     deviceInfo.MemoryClockMHz,
			DisplayState:       deviceInfo.DisplayState,
			PersistenceMode:    deviceInfo.PersistenceMode,
			PCIBusID:           deviceInfo.PCIBusID,
		}
	}
	return &FingerprintData{
		Devices:       allNvidiaGPUResources,
		DriverVersion: driverVersion,
	}, nil
}
