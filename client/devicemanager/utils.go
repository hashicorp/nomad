// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package devicemanager

import (
	"errors"
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/device"
	psstructs "github.com/hashicorp/nomad/plugins/shared/structs"
)

// UnknownDeviceError is returned when an operation is attempted on an unknown
// device.
type UnknownDeviceError struct {
	Err    error
	Name   string
	Vendor string
	Type   string
	IDs    []string
}

// NewUnknownDeviceError returns a new UnknownDeviceError for the given device.
func NewUnknownDeviceError(err error, name, vendor, devType string, ids []string) *UnknownDeviceError {
	return &UnknownDeviceError{
		Err:    err,
		Name:   name,
		Vendor: vendor, Type: devType,
		IDs: ids,
	}
}

// Error returns an error formatting that reveals which unknown devices were
// requested
func (u *UnknownDeviceError) Error() string {
	return fmt.Sprintf("operation on unknown device(s) \"%s/%s/%s\" (%v): %v",
		u.Vendor, u.Type, u.Name, u.IDs, u.Err)
}

// UnknownDeviceErrFromAllocated is a helper that returns an UnknownDeviceError
// populating it via the AllocatedDeviceResource struct.
func UnknownDeviceErrFromAllocated(err string, d *structs.AllocatedDeviceResource) *UnknownDeviceError {
	return NewUnknownDeviceError(errors.New(err), d.Name, d.Vendor, d.Type, d.DeviceIDs)
}

// convertDeviceGroup converts a device group to a structs NodeDeviceResource
func convertDeviceGroup(d *device.DeviceGroup) *structs.NodeDeviceResource {
	if d == nil {
		return nil
	}

	return &structs.NodeDeviceResource{
		Vendor:     d.Vendor,
		Type:       d.Type,
		Name:       d.Name,
		Instances:  convertDevices(d.Devices),
		Attributes: psstructs.CopyMapStringAttribute(d.Attributes),
	}
}

func convertDevices(devs []*device.Device) []*structs.NodeDevice {
	if devs == nil {
		return nil
	}

	out := make([]*structs.NodeDevice, len(devs))
	for i, dev := range devs {
		out[i] = convertDevice(dev)
	}
	return out
}

func convertDevice(dev *device.Device) *structs.NodeDevice {
	if dev == nil {
		return nil
	}

	return &structs.NodeDevice{
		ID:                dev.ID,
		Healthy:           dev.Healthy,
		HealthDescription: dev.HealthDesc,
		Locality:          convertHwLocality(dev.HwLocality),
		Shared:            convertDeviceSharing(dev.Shared),
	}
}

func convertHwLocality(l *device.DeviceLocality) *structs.NodeDeviceLocality {
	if l == nil {
		return nil
	}

	return &structs.NodeDeviceLocality{
		PciBusID: l.PciBusID,
	}
}

func convertDeviceSharing(s *device.DeviceSharing) *structs.DeviceSharing {
	if s == nil {
		return &structs.DeviceSharing{Shared: structs.DeviceSharingInactive}
	}

	switch s.Shared {
	case device.SharingIneligible:
		return &structs.DeviceSharing{Shared: structs.DeviceSharingIneligible}
	case device.SharingActive:
		return &structs.DeviceSharing{Shared: structs.DeviceSharingActive}
	case device.SharingInactive:
		return &structs.DeviceSharing{Shared: structs.DeviceSharingInactive}
	default:
		return &structs.DeviceSharing{Shared: structs.DeviceSharingUnset}
	}
}
