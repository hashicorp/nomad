package nvidia

import (
	"github.com/hashicorp/nomad/plugins/device/cmd/nvidia/nvml"
)

type MockNvmlClient struct {
	FingerprintError            error
	FingerprintResponseReturned *nvml.FingerprintData
}

func (c *MockNvmlClient) GetFingerprintData() (*nvml.FingerprintData, error) {
	return c.FingerprintResponseReturned, c.FingerprintError
}
