package nvml

import (
	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
)

// DeviceInfo represents nvml device data
// this struct is returned by NvmlDriver DeviceInfoByIndex and
// DeviceInfoAndStatusByIndex methods
type DeviceInfo struct {
	// The following fields are guaranteed to be retrieved from nvml
	UUID            string
	PCIBusID        string
	DisplayState    string
	PersistenceMode string

	// The following fields can be nil after call to nvml, because nvml was
	// not able to retrieve this fields for specific nvidia card
	Name               *string
	MemoryMiB          *uint64
	PowerW             *uint
	BAR1MiB            *uint64
	PCIBandwidthMBPerS *uint
	CoresClockMHz      *uint
	MemoryClockMHz     *uint
}

// DeviceStatus represents nvml device status
// this struct is returned by NvmlDriver DeviceInfoAndStatusByIndex method
type DeviceStatus struct {
	// The following fields can be nil after call to nvml, because nvml was
	// not able to retrieve this fields for specific nvidia card
	PowerUsageW        *uint
	TemperatureC       *uint
	GPUUtilization     *uint // %
	MemoryUtilization  *uint // %
	EncoderUtilization *uint // %
	DecoderUtilization *uint // %
	BAR1UsedMiB        *uint64
	UsedMemoryMiB      *uint64
	ECCErrorsL1Cache   *uint64
	ECCErrorsL2Cache   *uint64
	ECCErrorsDevice    *uint64
}

// NvmlDriver represents set of methods to query nvml library
type NvmlDriver interface {
	Initialize() error
	Shutdown() error
	SystemDriverVersion() (string, error)
	DeviceCount() (uint, error)
	DeviceInfoByIndex(uint) (*DeviceInfo, error)
	DeviceInfoAndStatusByIndex(uint) (*DeviceInfo, *DeviceStatus, error)
}

// nvmlDriver implements NvmlDriver
// Users are required to call Initialize method before using any other methods
type nvmlDriver struct{}

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
