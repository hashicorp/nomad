package scheduler

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

// deviceAllocator is used to allocate devices to allocations. The allocator
// tracks availability as to not double allocate devices.
type deviceAllocator struct {
	ctx     Context
	devices map[structs.DeviceIdTuple]*deviceAllocatorInstance
}

// deviceAllocatorInstance wraps a device and adds tracking to the instances of
// the device to determine if they are free or not.
type deviceAllocatorInstance struct {
	// d is the device being wrapped
	d *structs.NodeDeviceResource

	// instances is a mapping of the device IDs of the instances to their usage.
	// Only a value of 0 indicates that the instance is unused.
	instances map[string]int
}

// newDeviceAllocator returns a new device allocator. The node is used to
// populate the set of available devices based on what healthy device instances
// exist on the node.
func newDeviceAllocator(ctx Context, n *structs.Node) *deviceAllocator {
	numDevices := 0
	var devices []*structs.NodeDeviceResource

	// COMPAT(0.11): Remove in 0.11
	if n.NodeResources != nil {
		numDevices = len(n.NodeResources.Devices)
		devices = n.NodeResources.Devices
	}

	d := &deviceAllocator{
		ctx:     ctx,
		devices: make(map[structs.DeviceIdTuple]*deviceAllocatorInstance, numDevices),
	}

	for _, dev := range devices {
		id := *dev.ID()
		d.devices[id] = &deviceAllocatorInstance{
			d:         dev,
			instances: make(map[string]int, len(dev.Instances)),
		}
		for _, instance := range dev.Instances {
			// Skip unhealthy devices as they aren't allocatable
			if !instance.Healthy {
				continue
			}

			d.devices[id].instances[instance.ID] = 0
		}
	}

	return d
}

// AddAllocs takes a set of allocations and internally marks which devices are
// used. If a device is used more than once by the set of passed allocations,
// the collision will be returned as true.
func (d *deviceAllocator) AddAllocs(allocs []*structs.Allocation) (collision bool) {
	for _, a := range allocs {
		// Filter any terminal allocation
		if a.TerminalStatus() {
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
					if devInst, ok := d.devices[*devID]; ok {
						if i, ok := devInst.instances[instanceID]; ok {
							// Mark that the device is in use
							devInst.instances[instanceID]++

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
func (d *deviceAllocator) AddReserved(res *structs.AllocatedDeviceResource) (collision bool) {
	// Lookup the device.
	devInst, ok := d.devices[*res.ID()]
	if !ok {
		return false
	}

	// For each reserved instance, mark it as used
	for _, id := range res.DeviceIDs {
		cur, ok := devInst.instances[id]
		if !ok {
			continue
		}

		// It has already been used, so mark that there is a collision
		if cur != 0 {
			collision = true
		}

		devInst.instances[id]++
	}

	return
}

// AssignDevice takes a device request and returns an assignment as well as a
// score for the assignment. If no assignment could be made, an error is
// returned explaining why.
func (d *deviceAllocator) AssignDevice(ask *structs.RequestedDevice) (out *structs.AllocatedDeviceResource, score float64, err error) {
	// Try to hot path
	if len(d.devices) == 0 {
		return nil, 0.0, fmt.Errorf("no devices available")
	}
	if ask.Count == 0 {
		return nil, 0.0, fmt.Errorf("invalid request of zero devices")
	}

	// Hold the current best offer
	var offer *structs.AllocatedDeviceResource
	var offerScore float64

	// Determine the devices that are feasible based on availability and
	// constraints
	for id, devInst := range d.devices {
		// Check if we have enough unused instances to use this
		assignable := uint64(0)
		for _, v := range devInst.instances {
			if v == 0 {
				assignable++
			}
		}

		// This device doesn't have enough instances
		if assignable < ask.Count {
			continue
		}

		// Check if the device works
		if !nodeDeviceMatches(d.ctx, devInst.d, ask) {
			continue
		}

		// Score the choice
		var choiceScore float64
		if l := len(ask.Affinities); l != 0 {
			for _, a := range ask.Affinities {
				// Resolve the targets
				lVal, ok := resolveDeviceTarget(a.LTarget, devInst.d)
				if !ok {
					continue
				}
				rVal, ok := resolveDeviceTarget(a.RTarget, devInst.d)
				if !ok {
					continue
				}

				// Check if satisfied
				if !checkAttributeAffinity(d.ctx, a.Operand, lVal, rVal) {
					continue
				}
				choiceScore += a.Weight
			}

			// normalize
			choiceScore /= float64(l)
		}

		// Only use the device if it is a higher score than we have already seen
		if offer != nil && choiceScore < offerScore {
			continue
		}

		// Set the new highest score
		offerScore = choiceScore

		// Build the choice
		offer = &structs.AllocatedDeviceResource{
			Vendor:    id.Vendor,
			Type:      id.Type,
			Name:      id.Name,
			DeviceIDs: make([]string, 0, ask.Count),
		}

		assigned := uint64(0)
		for id, v := range devInst.instances {
			if v == 0 && assigned < ask.Count {
				assigned++
				offer.DeviceIDs = append(offer.DeviceIDs, id)
				if assigned == ask.Count {
					break
				}
			}
		}
	}

	// Failed to find a match
	if offer == nil {
		return nil, 0.0, fmt.Errorf("no devices match request")
	}

	return offer, offerScore, nil
}
