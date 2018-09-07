package nvidia

import (
	"github.com/hashicorp/nomad/plugins/device/cmd/nvidia/nvml"
)

type MockNvmlClient struct {
	FingerprintError            error
	FingerprintResponseReturned *nvml.FingerprintData

	StatsError            error
	StatsResponseReturned []*nvml.StatsData
}

func (c *MockNvmlClient) GetFingerprintData() (*nvml.FingerprintData, error) {
	return c.FingerprintResponseReturned, c.FingerprintError
}

func (c *MockNvmlClient) GetStatsData() ([]*nvml.StatsData, error) {
	return c.StatsResponseReturned, c.StatsError
}
