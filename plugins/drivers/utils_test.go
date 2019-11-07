package drivers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResourceUsageRoundTrip(t *testing.T) {
	input := &ResourceUsage{
		CpuStats: &CpuStats{
			SystemMode:       0,
			UserMode:         0.9963907032120152,
			TotalTicks:       21.920595295932515,
			ThrottledPeriods: 2321,
			ThrottledTime:    123,
			Percent:          0.9963906952696598,
			Measured:         []string{"System Mode", "User Mode", "Percent"},
		},
		MemoryStats: &MemoryStats{
			RSS:            25681920,
			Swap:           15681920,
			Usage:          12,
			MaxUsage:       23,
			KernelUsage:    34,
			KernelMaxUsage: 45,
			Measured:       []string{"RSS", "Swap"},
		},
	}

	parsed := resourceUsageFromProto(resourceUsageToProto(input))

	require.EqualValues(t, parsed, input)
}
