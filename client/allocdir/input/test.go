// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package test

import (
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/device"
)

type Client interface {
	AllocStateHandler
}

// AllocStateHandler exposes a handler to be called when an allocation's state changes
type AllocStateHandler interface {
	// AllocStateUpdated is used to emit an updated allocation. This allocation
	// is stripped to only include client settable fields.
	AllocStateUpdated(alloc *structs.Allocation)
}

// DeviceStatsReporter gives access to the latest resource usage
// for devices
type DeviceStatsReporter interface {
	LatestDeviceResourceStats([]*structs.AllocatedDeviceResource) []*device.DeviceGroupStats
}
