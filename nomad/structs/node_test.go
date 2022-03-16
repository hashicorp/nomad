package structs

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

func TestDriverInfoEquals(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	var driverInfoTest = []struct {
		input    []*DriverInfo
		expected bool
		errorMsg string
	}{
		{
			[]*DriverInfo{
				{
					Healthy: true,
				},
				{
					Healthy: false,
				},
			},
			false,
			"Different healthy values should not be equal.",
		},
		{
			[]*DriverInfo{
				{
					HealthDescription: "not running",
				},
				{
					HealthDescription: "running",
				},
			},
			false,
			"Different health description values should not be equal.",
		},
		{
			[]*DriverInfo{
				{
					Detected:          false,
					Healthy:           true,
					HealthDescription: "This driver is ok",
				},
				{
					Detected:          true,
					Healthy:           true,
					HealthDescription: "This driver is ok",
				},
			},
			true,
			"Same health check should be equal",
		},
	}
	for _, testCase := range driverInfoTest {
		first := testCase.input[0]
		second := testCase.input[1]
		require.Equal(testCase.expected, first.HealthCheckEquals(second), testCase.errorMsg)
	}
}
