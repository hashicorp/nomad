package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDriverInfoEquals(t *testing.T) {
	require := require.New(t)
	var driverInfoTest = []struct {
		input    []*DriverInfo
		expected bool
		errorMsg string
	}{
		{
			[]*DriverInfo{
				&DriverInfo{
					Healthy: true,
				},
				&DriverInfo{
					Healthy: false,
				},
			},
			false,
			"Different healthy values should not be equal.",
		},
		{
			[]*DriverInfo{
				&DriverInfo{
					HealthDescription: "not running",
				},
				&DriverInfo{
					HealthDescription: "running",
				},
			},
			false,
			"Different health description values should not be equal.",
		},
		{
			[]*DriverInfo{
				&DriverInfo{
					Healthy:           true,
					HealthDescription: "running",
				},
				&DriverInfo{
					Healthy:           true,
					HealthDescription: "running",
				},
			},
			true,
			"Different health description values should not be equal.",
		},
	}
	for _, testCase := range driverInfoTest {
		first := testCase.input[0]
		second := testCase.input[1]
		require.Equal(testCase.expected, first.HealthCheckEquals(second), testCase.errorMsg)
	}
}
