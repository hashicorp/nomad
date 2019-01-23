package nvidia

import (
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/devices/gpu/nvidia/nvml"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/stretchr/testify/require"
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

func TestReserve(t *testing.T) {
	for _, testCase := range []struct {
		Name                string
		ExpectedReservation *device.ContainerReservation
		ExpectedError       error
		Device              *NvidiaDevice
		RequestedIDs        []string
	}{
		{
			Name:                "All RequestedIDs are not managed by Device",
			ExpectedReservation: nil,
			ExpectedError: &reservationError{[]string{
				"UUID1",
				"UUID2",
				"UUID3",
			}},
			RequestedIDs: []string{
				"UUID1",
				"UUID2",
				"UUID3",
			},
			Device: &NvidiaDevice{
				logger: hclog.NewNullLogger(),
			},
		},
		{
			Name:                "Some RequestedIDs are not managed by Device",
			ExpectedReservation: nil,
			ExpectedError: &reservationError{[]string{
				"UUID1",
				"UUID2",
			}},
			RequestedIDs: []string{
				"UUID1",
				"UUID2",
				"UUID3",
			},
			Device: &NvidiaDevice{
				devices: map[string]struct{}{
					"UUID3": {},
				},
				logger: hclog.NewNullLogger(),
			},
		},
		{
			Name: "All RequestedIDs are managed by Device",
			ExpectedReservation: &device.ContainerReservation{
				Envs: map[string]string{
					NvidiaVisibleDevices: "UUID1,UUID2,UUID3",
				},
			},
			ExpectedError: nil,
			RequestedIDs: []string{
				"UUID1",
				"UUID2",
				"UUID3",
			},
			Device: &NvidiaDevice{
				devices: map[string]struct{}{
					"UUID1": {},
					"UUID2": {},
					"UUID3": {},
				},
				logger: hclog.NewNullLogger(),
			},
		},
		{
			Name:                "No IDs requested",
			ExpectedReservation: &device.ContainerReservation{},
			ExpectedError:       nil,
			RequestedIDs:        nil,
			Device: &NvidiaDevice{
				devices: map[string]struct{}{
					"UUID1": {},
					"UUID2": {},
					"UUID3": {},
				},
				logger: hclog.NewNullLogger(),
			},
		},
	} {
		actualReservation, actualError := testCase.Device.Reserve(testCase.RequestedIDs)
		req := require.New(t)
		req.Equal(testCase.ExpectedReservation, actualReservation)
		req.Equal(testCase.ExpectedError, actualError)
	}
}
