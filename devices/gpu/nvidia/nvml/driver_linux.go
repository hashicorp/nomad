package nvml

import (
	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
)

// Initialize nvml library by locating nvml shared object file and calling ldopen
func (n *nvmlDriver) Initialize() error {
	return nvml.Init()
}

// Shutdown stops any further interaction with nvml
func (n *nvmlDriver) Shutdown() error {
	return nvml.Shutdown()
}

// SystemDriverVersion returns installed driver version
func (n *nvmlDriver) SystemDriverVersion() (string, error) {
	return nvml.GetDriverVersion()
}

// DeviceCount reports number of available GPU devices
func (n *nvmlDriver) DeviceCount() (uint, error) {
	return nvml.GetDeviceCount()
}

// DeviceInfoByIndex returns DeviceInfo for index GPU in system device list
func (n *nvmlDriver) DeviceInfoByIndex(index uint) (*DeviceInfo, error) {
	device, err := nvml.NewDevice(index)
	if err != nil {
		return nil, err
	}
	deviceMode, err := device.GetDeviceMode()
	if err != nil {
		return nil, err
	}
	return &DeviceInfo{
		UUID:               device.UUID,
		Name:               device.Model,
		MemoryMiB:          device.Memory,
		PowerW:             device.Power,
		BAR1MiB:            device.PCI.BAR1,
		PCIBandwidthMBPerS: device.PCI.Bandwidth,
		PCIBusID:           device.PCI.BusID,
		CoresClockMHz:      device.Clocks.Cores,
		MemoryClockMHz:     device.Clocks.Memory,
		DisplayState:       deviceMode.DisplayInfo.Mode.String(),
		PersistenceMode:    deviceMode.Persistence.String(),
	}, nil
}

// DeviceInfoByIndex returns DeviceInfo and DeviceStatus for index GPU in system device list
func (n *nvmlDriver) DeviceInfoAndStatusByIndex(index uint) (*DeviceInfo, *DeviceStatus, error) {
	device, err := nvml.NewDevice(index)
	if err != nil {
		return nil, nil, err
	}
	status, err := device.Status()
	if err != nil {
		return nil, nil, err
	}
	return &DeviceInfo{
			UUID:               device.UUID,
			Name:               device.Model,
			MemoryMiB:          device.Memory,
			PowerW:             device.Power,
			BAR1MiB:            device.PCI.BAR1,
			PCIBandwidthMBPerS: device.PCI.Bandwidth,
			PCIBusID:           device.PCI.BusID,
			CoresClockMHz:      device.Clocks.Cores,
			MemoryClockMHz:     device.Clocks.Memory,
		}, &DeviceStatus{
			TemperatureC:       status.Temperature,
			GPUUtilization:     status.Utilization.GPU,
			MemoryUtilization:  status.Utilization.Memory,
			EncoderUtilization: status.Utilization.Encoder,
			DecoderUtilization: status.Utilization.Decoder,
			UsedMemoryMiB:      status.Memory.Global.Used,
			ECCErrorsL1Cache:   status.Memory.ECCErrors.L1Cache,
			ECCErrorsL2Cache:   status.Memory.ECCErrors.L2Cache,
			ECCErrorsDevice:    status.Memory.ECCErrors.Device,
			PowerUsageW:        status.Power,
			BAR1UsedMiB:        status.PCI.BAR1Used,
		}, nil
}
