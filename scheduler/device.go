package scheduler

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

// deviceAllocator is used to allocate devices to allocations. The allocator
// tracks availability as to not double allocate devices.
type deviceAllocator struct {
	*structs.DeviceAccounter

	ctx Context
}

// newDeviceAllocator returns a new device allocator. The node is used to
// populate the set of available devices based on what healthy device instances
// exist on the node.
func newDeviceAllocator(ctx Context, n *structs.Node) *deviceAllocator {
	return &deviceAllocator{
		ctx:             ctx,
		DeviceAccounter: structs.NewDeviceAccounter(n),
	}
}

// AssignDevice takes a device request and returns an assignment as well as a
// score for the assignment. If no assignment could be made, an error is
// returned explaining why.
func (d *deviceAllocator) AssignDevice(ask *structs.RequestedDevice) (out *structs.AllocatedDeviceResource, score float64, err error) {
	// Try to hot path
	if len(d.Devices) == 0 {
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
	for id, devInst := range d.Devices {
		// Check if we have enough unused instances to use this
		assignable := uint64(0)
		for _, v := range devInst.Instances {
			if v == 0 {
				assignable++
			}
		}

		// This device doesn't have enough instances
		if assignable < ask.Count {
			continue
		}

		// Check if the device works
		if !nodeDeviceMatches(d.ctx, devInst.Device, ask) {
			continue
		}

		// Score the choice
		var choiceScore float64
		if l := len(ask.Affinities); l != 0 {
			totalWeight := 0.0
			for _, a := range ask.Affinities {
				// Resolve the targets
				lVal, ok := resolveDeviceTarget(a.LTarget, devInst.Device)
				if !ok {
					continue
				}
				rVal, ok := resolveDeviceTarget(a.RTarget, devInst.Device)
				if !ok {
					continue
				}

				totalWeight += a.Weight

				// Check if satisfied
				if !checkAttributeAffinity(d.ctx, a.Operand, lVal, rVal) {
					continue
				}
				choiceScore += a.Weight
			}

			// normalize
			choiceScore /= totalWeight
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
		for id, v := range devInst.Instances {
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
