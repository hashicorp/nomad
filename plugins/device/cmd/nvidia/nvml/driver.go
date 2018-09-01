package nvml

import (
	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
)

// DeviceInfo represents nvml device data
// this struct is returned by NvmlDriver DeviceInfoByIndex method
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

// NvmlDriver represents set of methods to query nvml library
type NvmlDriver interface {
	Initialize() error
	Shutdown() error
	SystemDriverVersion() (string, error)
	DeviceCount() (uint, error)
	DeviceInfoByIndex(uint) (*DeviceInfo, error)
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
