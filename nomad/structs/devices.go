// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"maps"
)

// DeviceAccounter is used to account for device usage on a node. It can detect
// when a node is oversubscribed and can be used for deciding what devices are
// free
type DeviceAccounter struct {
	// Devices maps a device group to its device accounter instance
	Devices map[DeviceIdTuple]*DeviceAccounterInstance
}

// DeviceAccounterInstance wraps a device and adds tracking to the instances of
// the device to determine if they are free or not.
type DeviceAccounterInstance struct {
	// Device is the device being wrapped
	Device *NodeDeviceResource

	// Instances is a mapping of the device IDs to their usage.
	// Only a value of 0 indicates that the instance is unused.
	Instances map[string]int

	// WillShare is a mapping of the device IDs to whether the
	// tasks allocated to them indicated a willingness to share
	WillShare map[string]bool
}

// GetLocality returns the NodeDeviceLocality of the instance of the specific
// deviceID.
//
// If no instance matching the deviceID is found, nil is returned.
func (dai *DeviceAccounterInstance) GetLocality(instanceID string) *NodeDeviceLocality {
	for _, instance := range dai.Device.Instances {
		if instance.ID == instanceID {
			return instance.Locality.Copy()
		}
	}
	return nil
}

func (dai *DeviceAccounterInstance) Copy() *DeviceAccounterInstance {
	return &DeviceAccounterInstance{
		Device:    dai.Device.Copy(),
		Instances: maps.Clone(dai.Instances),
		WillShare: maps.Clone(dai.WillShare),
	}
}

// NewDeviceAccounter returns a new device accounter. The node is used to
// populate the set of available devices based on what healthy device instances
// exist on the node.
func NewDeviceAccounter(n *Node) *DeviceAccounter {
	numDevices := 0
	var devices []*NodeDeviceResource

	// COMPAT(0.11): Remove in 0.11
	if n.NodeResources != nil {
		numDevices = len(n.NodeResources.Devices)
		devices = n.NodeResources.Devices
	}

	d := &DeviceAccounter{
		Devices: make(map[DeviceIdTuple]*DeviceAccounterInstance, numDevices),
	}

	for _, dev := range devices {
		id := *dev.ID()
		d.Devices[id] = &DeviceAccounterInstance{
			Device:    dev,
			Instances: make(map[string]int, len(dev.Instances)),
			WillShare: make(map[string]bool, len(dev.Instances)),
		}
		for _, instance := range dev.Instances {
			// Skip unhealthy devices as they aren't allocatable
			if !instance.Healthy {
				continue
			}

			d.Devices[id].Instances[instance.ID] = 0
		}
	}

	return d
}

func (d *DeviceAccounter) Copy() *DeviceAccounter {
	devices := make(map[DeviceIdTuple]*DeviceAccounterInstance, len(d.Devices))
	for k, v := range d.Devices {
		devices[k] = v.Copy()
	}
	return &DeviceAccounter{Devices: devices}
}

// AddAllocs takes a set of allocations and internally marks which devices are
// used. If a device is used more than once by the set of passed allocations,
// the collision will be returned as true unless it has been placed on a
// device that explicitly allows sharing.
func (d *DeviceAccounter) AddAllocs(allocs []*Allocation) (collision bool) {
	for _, a := range allocs {
		// Filter any terminal allocation
		if a.ClientTerminalStatus() {
			continue
		}

		// COMPAT(0.11): Remove in 0.11
		// If the alloc doesn't have the new style resources, it can't have
		// devices
		if a.AllocatedResources == nil {
			continue
		}

		// Go through each task  resource
		for _, tr := range a.AllocatedResources.Tasks {

			// Go through each assigned device group
			for _, allocatedDeviceGroup := range tr.Devices {

				devID := allocatedDeviceGroup.ID()
				// Go through each assigned device
				for _, instanceID := range allocatedDeviceGroup.DeviceIDs {

					// Mark that we are using the device. It may not be in the
					// map if the device is no longer being fingerprinted, is
					// unhealthy, etc.
					if devAccounter, ok := d.Devices[*devID]; ok {
						if i, ok := devAccounter.Instances[instanceID]; ok {
							// Mark that the device is in use
							devAccounter.Instances[instanceID]++
							if shared := isShared(instanceID, devAccounter); shared {
								continue
							}
							if i != 0 {
								collision = true
							}
						}
					}
				}
			}
		}
	}

	return
}

// isShared loops through the []*NodeDevices in DeviceAccounterInstance.Device
// and returns a bool to indicate whether the device matching the supplied
// instanceID is shared
func isShared(instanceID string, accounterInst *DeviceAccounterInstance) bool {
	for _, device := range accounterInst.Device.Instances {
		if device.ID == instanceID {
			if device.Shared == DeviceSharingActive {
				return true
			}
		}
	}
	return false
}

// willShare is called in the loop that marks each reserved instance as used
// in the accounter. It takes a deviceID string and uses it to look up
// return the task requesting the device is willing to share
func willShare(res *AllocatedDeviceResource, deviceID string) bool {
	// d.WillShare is nil => return false as default and do reservation as usual
	if res.WillShare == nil {
		return false
	}
	// does exist, is true = > this is the shared device, it will share => return true
	if exists, willing := res.WillShare[deviceID]; exists && willing {

		return true
	}
	// In all remaining cases we return false
	// does not exist, val is true => this is not the shared device's ID => return false
	// does not exist, va, is false => this is not the shared device's ID => return false
	// does exist is false = > this is the shared device, it will not share => return false
	return false
}

// AddReserved marks the device instances in the passed device reservation as
// used, checks the res.WillShare map to see if the createOffer expected the device
// to share. If the device will share we do not report a collision even if it
// has already been used
func (d *DeviceAccounter) AddReserved(res *AllocatedDeviceResource) (collision bool) {
	// Lookup the deviceAccounter
	devAccounter, ok := d.Devices[*res.ID()]
	if !ok {
		return false
	}

	// For each reserved instance, mark it as used
	for _, id := range res.DeviceIDs {
		cur, ok := devAccounter.Instances[id]
		if !ok {
			continue
		}

		// if offer expects device will share, mark device as used
		// and continue without marking collision
		if willShare(res, id) {
			devAccounter.Instances[id]++
			continue
		}

		// mark collision if device will not share and has already been used
		if cur != 0 {
			collision = true
		}
		devAccounter.Instances[id]++

	}
	return
}

// FreeCount returns the number of free device instances
func (dai *DeviceAccounterInstance) FreeCount() int {
	count := 0
	for _, c := range dai.Instances {
		if c == 0 {
			count++
		}
	}
	return count
}
