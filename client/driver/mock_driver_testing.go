// +build !release

package driver

// Add the mock driver to the list of builtin drivers
func init() {
	BuiltinDrivers["mock_driver"] = NewMockDriver
}
