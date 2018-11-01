package device

import (
	"context"
	"time"

	"github.com/hashicorp/nomad/plugins/base"
)

type FingerprintFn func(context.Context) (<-chan *FingerprintResponse, error)
type ReserveFn func([]string) (*ContainerReservation, error)
type StatsFn func(context.Context, time.Duration) (<-chan *StatsResponse, error)

// MockDevicePlugin is used for testing.
// Each function can be set as a closure to make assertions about how data
// is passed through the base plugin layer.
type MockDevicePlugin struct {
	*base.MockPlugin
	FingerprintF FingerprintFn
	ReserveF     ReserveFn
	StatsF       StatsFn
}

func (p *MockDevicePlugin) Fingerprint(ctx context.Context) (<-chan *FingerprintResponse, error) {
	return p.FingerprintF(ctx)
}

func (p *MockDevicePlugin) Reserve(devices []string) (*ContainerReservation, error) {
	return p.ReserveF(devices)
}

func (p *MockDevicePlugin) Stats(ctx context.Context, interval time.Duration) (<-chan *StatsResponse, error) {
	return p.StatsF(ctx, interval)
}

// Below are static implementations of the device functions

// StaticFingerprinter fingerprints the passed devices just once
func StaticFingerprinter(devices []*DeviceGroup) FingerprintFn {
	return func(_ context.Context) (<-chan *FingerprintResponse, error) {
		outCh := make(chan *FingerprintResponse, 1)
		outCh <- &FingerprintResponse{
			Devices: devices,
		}
		return outCh, nil
	}
}

// ErrorChFingerprinter returns an error fingerprinting over the channel
func ErrorChFingerprinter(err error) FingerprintFn {
	return func(_ context.Context) (<-chan *FingerprintResponse, error) {
		outCh := make(chan *FingerprintResponse, 1)
		outCh <- &FingerprintResponse{
			Error: err,
		}
		return outCh, nil
	}
}

// StaticReserve returns the passed container reservation
func StaticReserve(out *ContainerReservation) ReserveFn {
	return func(_ []string) (*ContainerReservation, error) {
		return out, nil
	}
}

// ErrorReserve returns the passed error
func ErrorReserve(err error) ReserveFn {
	return func(_ []string) (*ContainerReservation, error) {
		return nil, err
	}
}

// StaticStats returns the passed statistics only updating the timestamp
func StaticStats(out []*DeviceGroupStats) StatsFn {
	return func(ctx context.Context, intv time.Duration) (<-chan *StatsResponse, error) {
		outCh := make(chan *StatsResponse, 1)

		go func() {
			ticker := time.NewTimer(0)
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					ticker.Reset(intv)
				}

				now := time.Now()
				for _, g := range out {
					for _, i := range g.InstanceStats {
						i.Timestamp = now
					}
				}

				outCh <- &StatsResponse{
					Groups: out,
				}
			}
		}()

		return outCh, nil
	}
}

// ErrorChStats returns an error collecting stats over the channel
func ErrorChStats(err error) StatsFn {
	return func(_ context.Context, _ time.Duration) (<-chan *StatsResponse, error) {
		outCh := make(chan *StatsResponse, 1)
		outCh <- &StatsResponse{
			Error: err,
		}
		return outCh, nil
	}
}
