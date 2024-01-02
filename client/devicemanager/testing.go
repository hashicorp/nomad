// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package devicemanager

import (
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/device"
)

type ReserveFn func(d *structs.AllocatedDeviceResource) (*device.ContainerReservation, error)
type AllStatsFn func() []*device.DeviceGroupStats
type DeviceStatsFn func(d *structs.AllocatedDeviceResource) (*device.DeviceGroupStats, error)

func NoopReserve(*structs.AllocatedDeviceResource) (*device.ContainerReservation, error) {
	return nil, nil
}

func NoopAllStats() []*device.DeviceGroupStats {
	return nil
}

func NoopDeviceStats(*structs.AllocatedDeviceResource) (*device.DeviceGroupStats, error) {
	return nil, nil
}

func NoopMockManager() *MockManager {
	return &MockManager{
		ReserveF:     NoopReserve,
		AllStatsF:    NoopAllStats,
		DeviceStatsF: NoopDeviceStats,
	}
}

type MockManager struct {
	ReserveF     ReserveFn
	AllStatsF    AllStatsFn
	DeviceStatsF DeviceStatsFn
}

func (m *MockManager) Run()                                 {}
func (m *MockManager) Shutdown()                            {}
func (m *MockManager) PluginType() string                   { return base.PluginTypeDevice }
func (m *MockManager) AllStats() []*device.DeviceGroupStats { return m.AllStatsF() }

func (m *MockManager) Reserve(d *structs.AllocatedDeviceResource) (*device.ContainerReservation, error) {
	return m.ReserveF(d)
}

func (m *MockManager) DeviceStats(d *structs.AllocatedDeviceResource) (*device.DeviceGroupStats, error) {
	return m.DeviceStatsF(d)
}
