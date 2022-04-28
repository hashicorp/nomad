// +build !linux

package nvml

// Initialize nvml library by locating nvml shared object file and calling ldopen
func (n *nvmlDriver) Initialize() error {
	return UnavailableLib
}

// Shutdown stops any further interaction with nvml
func (n *nvmlDriver) Shutdown() error {
	return UnavailableLib
}

// SystemDriverVersion returns installed driver version
func (n *nvmlDriver) SystemDriverVersion() (string, error) {
	return "", UnavailableLib
}

// DeviceCount reports number of available GPU devices
func (n *nvmlDriver) DeviceCount() (uint, error) {
	return 0, UnavailableLib
}

// DeviceInfoByIndex returns DeviceInfo for index GPU in system device list
func (n *nvmlDriver) DeviceInfoByIndex(index uint) (*DeviceInfo, error) {
	return nil, UnavailableLib
}

// DeviceInfoByIndex returns DeviceInfo and DeviceStatus for index GPU in system device list
func (n *nvmlDriver) DeviceInfoAndStatusByIndex(index uint) (*DeviceInfo, *DeviceStatus, error) {
	return nil, nil, UnavailableLib
}
