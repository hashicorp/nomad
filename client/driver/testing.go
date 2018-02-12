package driver

import (
	"testing"
)

// CheckForMockDriver is a test helper that ensures the mock driver is enabled.
// If not, it skips the current test.
func CheckForMockDriver(t *testing.T) {
	if _, ok := BuiltinDrivers["mock_driver"]; !ok {
		t.Skip(`test requires mock_driver; run with "-tags nomad_test"`)
	}
}
