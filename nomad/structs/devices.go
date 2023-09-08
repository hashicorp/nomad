// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

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

// AddAllocs takes a set of allocations and internally marks which devices are
// used. If a device is used more than once by the set of passed allocations,
// the collision will be returned as true.
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
			for _, device := range tr.Devices {
				devID := device.ID()

				// Go through each assigned device
				for _, instanceID := range device.DeviceIDs {

					// Mark that we are using the device. It may not be in the
					// map if the device is no longer being fingerprinted, is
					// unhealthy, etc.
					if devInst, ok := d.Devices[*devID]; ok {
						if i, ok := devInst.Instances[instanceID]; ok {
							// Mark that the device is in use
							devInst.Instances[instanceID]++

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

// AddReserved marks the device instances in the passed device reservation as
// used and returns if there is a collision.
func (d *DeviceAccounter) AddReserved(res *AllocatedDeviceResource) (collision bool) {
	// Lookup the device.
	devInst, ok := d.Devices[*res.ID()]
	if !ok {
		return false
	}

	// For each reserved instance, mark it as used
	for _, id := range res.DeviceIDs {
		cur, ok := devInst.Instances[id]
		if !ok {
			continue
		}

		// It has already been used, so mark that there is a collision
		if cur != 0 {
			collision = true
		}

		devInst.Instances[id]++
	}

	return
}

// FreeCount returns the number of free device instances
func (i *DeviceAccounterInstance) FreeCount() int {
	count := 0
	for _, c := range i.Instances {
		if c == 0 {
			count++
		}
	}
	return count
}
