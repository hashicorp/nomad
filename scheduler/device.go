// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"fmt"

	"math"

	"github.com/hashicorp/nomad/nomad/structs"
	psstructs "github.com/hashicorp/nomad/plugins/shared/structs"
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
	var matchedWeights float64

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

		// Track the sum of matched affinity weights in a separate variable
		// We return this if this device had the best score compared to other devices considered
		var sumMatchedWeights float64
		if l := len(ask.Affinities); l != 0 {
			totalWeight := 0.0
			for _, a := range ask.Affinities {
				// Resolve the targets
				lVal, lOk := resolveDeviceTarget(a.LTarget, devInst.Device)
				rVal, rOk := resolveDeviceTarget(a.RTarget, devInst.Device)

				totalWeight += math.Abs(float64(a.Weight))

				// Check if satisfied
				if !checkAttributeAffinity(d.ctx, a.Operand, lVal, rVal, lOk, rOk) {
					continue
				}
				choiceScore += float64(a.Weight)
				sumMatchedWeights += float64(a.Weight)
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

		// Set the new sum of matching affinity weights
		matchedWeights = sumMatchedWeights

		// Build the choice
		offer = &structs.AllocatedDeviceResource{
			Vendor:    id.Vendor,
			Type:      id.Type,
			Name:      id.Name,
			DeviceIDs: make([]string, 0, ask.Count),
		}

		assigned := uint64(0)
		for id, v := range devInst.Instances {
			if v == 0 && assigned < ask.Count &&
				d.deviceIDMatchesConstraint(id, ask.Constraints, devInst.Device) {
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

	return offer, matchedWeights, nil
}

// deviceIDMatchesConstraint checks a device instance ID against the constraints
// to ensure we're only assigning instance IDs that match. This is a narrower
// check than nodeDeviceMatches because we've already asserted that the device
// matches and now need to filter by instance ID.
func (d *deviceAllocator) deviceIDMatchesConstraint(id string, constraints structs.Constraints, device *structs.NodeDeviceResource) bool {

	// There are no constraints to consider
	if len(constraints) == 0 {
		return true
	}

	deviceID := psstructs.NewStringAttribute(id)

	for _, c := range constraints {
		var target *psstructs.Attribute
		if c.LTarget == "${device.ids}" {
			target, _ = resolveDeviceTarget(c.RTarget, device)
		} else if c.RTarget == "${device.ids}" {
			target, _ = resolveDeviceTarget(c.LTarget, device)
		} else {
			continue
		}

		if !checkAttributeConstraint(d.ctx, c.Operand, target, deviceID, true, true) {
			return false
		}
	}

	return true
}
