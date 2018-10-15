package scheduler

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

type deviceAllocator struct {
	ctx     Context
	devices map[structs.DeviceIdTuple]*deviceAllocatorInstance
}

type deviceAllocatorInstance struct {
	d         *structs.NodeDeviceResource
	instances map[string]int
}

// Free returns if the device is free to use.
func (d *deviceAllocatorInstance) Free(id string) bool {
	uses, ok := d.instances[id]
	return ok && uses == 0
}

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
// used.
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

func (d *deviceAllocator) AddReserved(res *structs.AllocatedDeviceResource) (collision bool) {
	devInst, ok := d.devices[*res.ID()]
	if !ok {
		return false
	}

	for _, id := range res.DeviceIDs {
		cur, ok := devInst.instances[id]
		if !ok {
			continue
		}

		if cur != 0 {
			collision = true
		}

		devInst.instances[id]++
	}

	return
}

func (d *deviceAllocator) AssignDevice(ask *structs.RequestedDevice) (out *structs.AllocatedDeviceResource, err error) {
	// Try to hot path
	if len(d.devices) == 0 {
		return nil, fmt.Errorf("no devices available")
	}
	if ask.Count == 0 {
		return nil, fmt.Errorf("invalid request of zero devices")
	}

	// Hold the current best offer
	var offer *structs.AllocatedDeviceResource
	var score float64

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
		if len(ask.Affinities) != 0 {
			// TODO
		}

		if offer != nil && choiceScore < score {
			continue
		}

		// Set the new highest score
		score = choiceScore

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

	if offer == nil {
		return nil, fmt.Errorf("no devices match request")
	}

	return offer, nil
}
