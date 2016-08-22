// +build nomad_test

package driver

// Add the mock driver
func init() {
	BuiltinDrivers["mock_driver"] = NewMockDriver
}
