package device

import (
	"context"

	"github.com/hashicorp/nomad/plugins/base"
)

// MockDevicePlugin is used for testing.
// Each function can be set as a closure to make assertions about how data
// is passed through the base plugin layer.
type MockDevicePlugin struct {
	*base.MockPlugin
	FingerprintF func(context.Context) (<-chan *FingerprintResponse, error)
	ReserveF     func([]string) (*ContainerReservation, error)
	StatsF       func(context.Context) (<-chan *StatsResponse, error)
}

func (p *MockDevicePlugin) Fingerprint(ctx context.Context) (<-chan *FingerprintResponse, error) {
	return p.FingerprintF(ctx)
}

func (p *MockDevicePlugin) Reserve(devices []string) (*ContainerReservation, error) {
	return p.ReserveF(devices)
}

func (p *MockDevicePlugin) Stats(ctx context.Context) (<-chan *StatsResponse, error) {
	return p.StatsF(ctx)
}
