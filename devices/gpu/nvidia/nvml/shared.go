package nvml

import "errors"

var (
	// UnavailableLib is returned when the nvml library could not be loaded.
	UnavailableLib = errors.New("could not load NVML library")
)

// nvmlDriver implements NvmlDriver
// Users are required to call Initialize method before using any other methods
type nvmlDriver struct{}

// NvmlDriver represents set of methods to query nvml library
type NvmlDriver interface {
	Initialize() error
	Shutdown() error
	SystemDriverVersion() (string, error)
	DeviceCount() (uint, error)
	DeviceInfoByIndex(uint) (*DeviceInfo, error)
	DeviceInfoAndStatusByIndex(uint) (*DeviceInfo, *DeviceStatus, error)
}

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
