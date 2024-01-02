// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"math"
	"sort"

	"github.com/hashicorp/nomad/nomad/structs"
)

// maxParallelPenalty is a score penalty applied to allocations to mitigate against
// too many allocations of the same job being preempted. This penalty is applied after the
// number of allocations being preempted exceeds max_parallel value in the job's migrate block
const maxParallelPenalty = 50.0

type groupedAllocs struct {
	priority int
	allocs   []*structs.Allocation
}

type allocInfo struct {
	maxParallel int
	resources   *structs.ComparableResources
}

// PreemptionResource interface is implemented by different
// types of resources.
type PreemptionResource interface {
	// MeetsRequirements returns true if the available resources match needed resources
	MeetsRequirements() bool

	// Distance returns values in the range [0, MaxFloat], lower is better
	Distance() float64
}

// NetworkPreemptionResource implements PreemptionResource for network assignments
// It only looks at MBits needed
type NetworkPreemptionResource struct {
	availableResources *structs.NetworkResource
	resourceNeeded     *structs.NetworkResource
}

func (n *NetworkPreemptionResource) MeetsRequirements() bool {
	mbitsAvailable := n.availableResources.MBits
	mbitsNeeded := n.resourceNeeded.MBits
	if mbitsAvailable == 0 || mbitsNeeded == 0 {
		return false
	}
	return mbitsAvailable >= mbitsNeeded
}

func (n *NetworkPreemptionResource) Distance() float64 {
	return networkResourceDistance(n.availableResources, n.resourceNeeded)
}

// BasePreemptionResource implements PreemptionResource for CPU/Memory/Disk
type BasePreemptionResource struct {
	availableResources *structs.ComparableResources
	resourceNeeded     *structs.ComparableResources
}

func (b *BasePreemptionResource) MeetsRequirements() bool {
	super, _ := b.availableResources.Superset(b.resourceNeeded)
	return super
}

func (b *BasePreemptionResource) Distance() float64 {
	return basicResourceDistance(b.resourceNeeded, b.availableResources)
}

// PreemptionResourceFactory returns a new PreemptionResource
type PreemptionResourceFactory func(availableResources *structs.ComparableResources, resourceAsk *structs.ComparableResources) PreemptionResource

// GetNetworkPreemptionResourceFactory returns a preemption resource factory for network assignments
func GetNetworkPreemptionResourceFactory() PreemptionResourceFactory {
	return func(availableResources *structs.ComparableResources, resourceNeeded *structs.ComparableResources) PreemptionResource {
		available := availableResources.Flattened.Networks[0]
		return &NetworkPreemptionResource{
			availableResources: available,
			resourceNeeded:     resourceNeeded.Flattened.Networks[0],
		}
	}
}

// GetBasePreemptionResourceFactory returns a preemption resource factory for CPU/Memory/Disk
func GetBasePreemptionResourceFactory() PreemptionResourceFactory {
	return func(availableResources *structs.ComparableResources, resourceNeeded *structs.ComparableResources) PreemptionResource {
		return &BasePreemptionResource{
			availableResources: availableResources,
			resourceNeeded:     resourceNeeded,
		}
	}
}

// Preemptor is used to track existing allocations
// and find suitable allocations to preempt
type Preemptor struct {

	// currentPreemptions is a map computed when SetPreemptions is called
	// it tracks the number of preempted allocations per job/taskgroup
	currentPreemptions map[structs.NamespacedID]map[string]int

	// allocDetails is a map computed when SetCandidates is called
	// it stores some precomputed details about the allocation needed
	// when scoring it for preemption
	allocDetails map[string]*allocInfo

	// jobPriority is the priority of the job being preempted
	jobPriority int

	// jobID is the ID of the job being preempted
	jobID *structs.NamespacedID

	// nodeRemainingResources tracks available resources on the node after
	// accounting for running allocations
	nodeRemainingResources *structs.ComparableResources

	// currentAllocs is the candidate set used to find preemptible allocations
	currentAllocs []*structs.Allocation

	// ctx is the context from the scheduler stack
	ctx Context
}

func NewPreemptor(jobPriority int, ctx Context, jobID *structs.NamespacedID) *Preemptor {
	return &Preemptor{
		currentPreemptions: make(map[structs.NamespacedID]map[string]int),
		jobPriority:        jobPriority,
		jobID:              jobID,
		allocDetails:       make(map[string]*allocInfo),
		ctx:                ctx,
	}
}

// SetNode sets the node
func (p *Preemptor) SetNode(node *structs.Node) {
	nodeRemainingResources := node.ComparableResources()

	// Subtract the reserved resources of the node
	if c := node.ComparableReservedResources(); c != nil {
		nodeRemainingResources.Subtract(c)
	}
	p.nodeRemainingResources = nodeRemainingResources
}

// SetCandidates initializes the candidate set from which preemptions are chosen
func (p *Preemptor) SetCandidates(allocs []*structs.Allocation) {
	// Reset candidate set
	p.currentAllocs = []*structs.Allocation{}
	for _, alloc := range allocs {
		// Ignore any allocations of the job being placed
		// This filters out any previous allocs of the job, and any new allocs in the plan
		if alloc.JobID == p.jobID.ID && alloc.Namespace == p.jobID.Namespace {
			continue
		}

		maxParallel := 0
		tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
		if tg != nil && tg.Migrate != nil {
			maxParallel = tg.Migrate.MaxParallel
		}
		p.allocDetails[alloc.ID] = &allocInfo{maxParallel: maxParallel, resources: alloc.ComparableResources()}
		p.currentAllocs = append(p.currentAllocs, alloc)
	}
}

// SetPreemptions initializes a map tracking existing counts of preempted allocations
// per job/task group. This is used while scoring preemption options
func (p *Preemptor) SetPreemptions(allocs []*structs.Allocation) {

	// Clear out existing values since this can be called more than once
	p.currentPreemptions = make(map[structs.NamespacedID]map[string]int)

	// Initialize counts
	for _, alloc := range allocs {
		id := structs.NewNamespacedID(alloc.JobID, alloc.Namespace)
		countMap, ok := p.currentPreemptions[id]
		if !ok {
			countMap = make(map[string]int)
			p.currentPreemptions[id] = countMap
		}
		countMap[alloc.TaskGroup]++
	}
}

// getNumPreemptions counts the number of other allocations being preempted that match the job and task group of
// the alloc under consideration. This is used as a scoring factor to minimize too many allocs of the same job being preempted at once
func (p *Preemptor) getNumPreemptions(alloc *structs.Allocation) int {
	c, ok := p.currentPreemptions[structs.NewNamespacedID(alloc.JobID, alloc.Namespace)][alloc.TaskGroup]
	if !ok {
		return 0
	}
	return c
}

// PreemptForTaskGroup computes a list of allocations to preempt to accommodate
// the resources asked for. Only allocs with a job priority < 10 of jobPriority are considered
// This method is meant only for finding preemptible allocations based on CPU/Memory/Disk
func (p *Preemptor) PreemptForTaskGroup(resourceAsk *structs.AllocatedResources) []*structs.Allocation {
	resourcesNeeded := resourceAsk.Comparable()

	// Subtract current allocations
	for _, alloc := range p.currentAllocs {
		allocResources := p.allocDetails[alloc.ID].resources
		p.nodeRemainingResources.Subtract(allocResources)
	}

	// Group candidates by priority, filter out ineligible allocs
	allocsByPriority := filterAndGroupPreemptibleAllocs(p.jobPriority, p.currentAllocs)

	var bestAllocs []*structs.Allocation
	allRequirementsMet := false

	// Initialize variable to track resources as they become available from preemption
	availableResources := p.nodeRemainingResources.Copy()

	resourcesAsked := resourceAsk.Comparable()
	// Iterate over allocations grouped by priority to find preemptible allocations
	for _, allocGrp := range allocsByPriority {
		for len(allocGrp.allocs) > 0 && !allRequirementsMet {
			closestAllocIndex := -1
			bestDistance := math.MaxFloat64
			// Find the alloc with the closest distance
			for index, alloc := range allocGrp.allocs {
				currentPreemptionCount := p.getNumPreemptions(alloc)
				allocDetails := p.allocDetails[alloc.ID]
				maxParallel := allocDetails.maxParallel
				distance := scoreForTaskGroup(resourcesNeeded, allocDetails.resources, maxParallel, currentPreemptionCount)
				if distance < bestDistance {
					bestDistance = distance
					closestAllocIndex = index
				}
			}
			closestAlloc := allocGrp.allocs[closestAllocIndex]
			closestResources := p.allocDetails[closestAlloc.ID].resources
			availableResources.Add(closestResources)

			// This step needs the original resources asked for as the second arg, can't use the running total
			allRequirementsMet, _ = availableResources.Superset(resourcesAsked)

			bestAllocs = append(bestAllocs, closestAlloc)

			allocGrp.allocs[closestAllocIndex] = allocGrp.allocs[len(allocGrp.allocs)-1]
			allocGrp.allocs = allocGrp.allocs[:len(allocGrp.allocs)-1]

			// This is the remaining total of resources needed
			resourcesNeeded.Subtract(closestResources)
		}
		if allRequirementsMet {
			break
		}
	}

	// Early return if all allocs examined and requirements were not met
	if !allRequirementsMet {
		return nil
	}

	// We do another pass to eliminate unnecessary preemptions
	// This filters out allocs whose resources are already covered by another alloc
	basePreemptionResource := GetBasePreemptionResourceFactory()
	resourcesNeeded = resourceAsk.Comparable()
	filteredBestAllocs := p.filterSuperset(bestAllocs, p.nodeRemainingResources, resourcesNeeded, basePreemptionResource)
	return filteredBestAllocs

}

// PreemptForNetwork tries to find allocations to preempt to meet network resources.
// This is called once per task when assigning a network to the task. While finding allocations
// to preempt, this only considers allocations that share the same network device
func (p *Preemptor) PreemptForNetwork(networkResourceAsk *structs.NetworkResource, netIdx *structs.NetworkIndex) []*structs.Allocation {

	// Early return if there are no current allocs
	if len(p.currentAllocs) == 0 {
		return nil
	}

	deviceToAllocs := make(map[string][]*structs.Allocation)
	MbitsNeeded := networkResourceAsk.MBits
	reservedPortsNeeded := networkResourceAsk.ReservedPorts

	// Build map of reserved ports needed for fast access
	reservedPorts := make(map[int]struct{})
	for _, port := range reservedPortsNeeded {
		reservedPorts[port.Value] = struct{}{}
	}

	// filteredReservedPorts tracks reserved ports that are
	// currently used by higher priority allocations that can't
	// be preempted
	filteredReservedPorts := make(map[string]map[int]struct{})

	// Create a map from each device to allocs
	// We can only preempt within allocations that
	// are using the same device
	for _, alloc := range p.currentAllocs {
		if alloc.Job == nil {
			continue
		}

		allocResources := p.allocDetails[alloc.ID].resources
		networks := allocResources.Flattened.Networks
		if len(networks) == 0 {
			continue
		}

		// We only check first network - TODO: why?!?!
		net := networks[0]

		// Filter out alloc that's ineligible due to priority
		if p.jobPriority-alloc.Job.Priority < 10 {
			// Populate any reserved ports used by
			// this allocation that cannot be preempted
			for _, port := range net.ReservedPorts {
				portMap, ok := filteredReservedPorts[net.Device]
				if !ok {
					portMap = make(map[int]struct{})
					filteredReservedPorts[net.Device] = portMap
				}
				portMap[port.Value] = struct{}{}
			}
			continue
		}

		// Only include if the alloc has a network device
		device := networks[0].Device
		allocsForDevice := deviceToAllocs[device]
		allocsForDevice = append(allocsForDevice, alloc)
		deviceToAllocs[device] = allocsForDevice
	}

	// If no existing allocations use network resources, return early
	if len(deviceToAllocs) == 0 {
		return nil
	}

	var allocsToPreempt []*structs.Allocation
	met := false
	freeBandwidth := 0
	preemptedDevice := ""

OUTER:
	for device, currentAllocs := range deviceToAllocs {
		preemptedDevice = device
		totalBandwidth := netIdx.AvailBandwidth[device]

		// If the device doesn't have enough total available bandwidth, skip
		if totalBandwidth < MbitsNeeded {
			continue
		}

		// Track how much existing free bandwidth we have before preemption
		freeBandwidth = totalBandwidth - netIdx.UsedBandwidth[device]

		preemptedBandwidth := 0

		// Reset allocsToPreempt since we don't want to preempt across devices for the same task
		allocsToPreempt = nil

		// usedPortToAlloc tracks used ports by allocs in this device
		usedPortToAlloc := make(map[int]*structs.Allocation)

		// First try to satisfy needed reserved ports
		if len(reservedPortsNeeded) > 0 {

			// Populate usedPort map
			for _, alloc := range currentAllocs {
				allocResources := p.allocDetails[alloc.ID].resources
				for _, n := range allocResources.Flattened.Networks {
					reservedPorts := n.ReservedPorts
					for _, p := range reservedPorts {
						usedPortToAlloc[p.Value] = alloc
					}
				}
			}
			// Look for allocs that are using reserved ports needed
			for _, port := range reservedPortsNeeded {
				alloc, ok := usedPortToAlloc[port.Value]
				if ok {
					allocResources := p.allocDetails[alloc.ID].resources
					preemptedBandwidth += allocResources.Flattened.Networks[0].MBits
					allocsToPreempt = append(allocsToPreempt, alloc)
				} else {
					// Check if a higher priority allocation is using this port
					// It cant be preempted so we skip to the next device
					_, ok := filteredReservedPorts[device][port.Value]
					if ok {
						continue OUTER
					}
				}
			}

			// Remove allocs that were preempted to satisfy reserved ports
			currentAllocs = structs.RemoveAllocs(currentAllocs, allocsToPreempt)
		}

		// If bandwidth requirements have been met, stop
		if preemptedBandwidth+freeBandwidth >= MbitsNeeded {
			met = true
			break OUTER
		}

		// Split by priority
		allocsByPriority := filterAndGroupPreemptibleAllocs(p.jobPriority, currentAllocs)

		for _, allocsGrp := range allocsByPriority {
			allocs := allocsGrp.allocs

			// Sort by distance function
			sort.Slice(allocs, func(i, j int) bool {
				return p.distanceComparatorForNetwork(allocs, networkResourceAsk, i, j)
			})

			// Iterate over allocs until end of if requirements have been met
			for _, alloc := range allocs {
				allocResources := p.allocDetails[alloc.ID].resources
				preemptedBandwidth += allocResources.Flattened.Networks[0].MBits
				allocsToPreempt = append(allocsToPreempt, alloc)
				if preemptedBandwidth+freeBandwidth >= MbitsNeeded {
					met = true
					break OUTER
				}
			}

		}

	}

	// Early return if we could not meet resource needs after examining allocs
	if !met {
		return nil
	}

	// Build a resource object with just the network Mbits filled in
	nodeRemainingResources := &structs.ComparableResources{
		Flattened: structs.AllocatedTaskResources{
			Networks: []*structs.NetworkResource{
				{
					Device: preemptedDevice,
					MBits:  freeBandwidth,
				},
			},
		},
	}

	// Do a final pass to eliminate any superset allocations
	preemptionResourceFactory := GetNetworkPreemptionResourceFactory()
	resourcesNeeded := &structs.ComparableResources{
		Flattened: structs.AllocatedTaskResources{
			Networks: []*structs.NetworkResource{networkResourceAsk},
		},
	}
	filteredBestAllocs := p.filterSuperset(allocsToPreempt, nodeRemainingResources, resourcesNeeded, preemptionResourceFactory)
	return filteredBestAllocs
}

// deviceGroupAllocs represents a group of allocs that share a device
type deviceGroupAllocs struct {
	allocs []*structs.Allocation

	// deviceInstances tracks the number of instances used per alloc
	deviceInstances map[string]int
}

func newAllocDeviceGroup() *deviceGroupAllocs {
	return &deviceGroupAllocs{
		deviceInstances: make(map[string]int),
	}
}

// PreemptForDevice tries to find allocations to preempt to meet devices needed
// This is called once per device request when assigning devices to the task
func (p *Preemptor) PreemptForDevice(ask *structs.RequestedDevice, devAlloc *deviceAllocator) []*structs.Allocation {

	// Group allocations by device, tracking the number of
	// instances used in each device by alloc id
	deviceToAllocs := make(map[structs.DeviceIdTuple]*deviceGroupAllocs)
	for _, alloc := range p.currentAllocs {
		for _, tr := range alloc.AllocatedResources.Tasks {
			// Ignore allocs that don't use devices
			if len(tr.Devices) == 0 {
				continue
			}

			// Go through each assigned device group
			for _, device := range tr.Devices {
				// Look up the device instance from the device allocator
				deviceIdTuple := *device.ID()
				devInst := devAlloc.Devices[deviceIdTuple]

				// devInst can be nil if the device is no longer healthy
				if devInst == nil {
					continue
				}

				// Ignore if the device doesn't match the ask
				if !nodeDeviceMatches(p.ctx, devInst.Device, ask) {
					continue
				}

				// Store both the alloc and the number of instances used
				// in our tracking map
				allocDeviceGrp := deviceToAllocs[deviceIdTuple]
				if allocDeviceGrp == nil {
					allocDeviceGrp = newAllocDeviceGroup()
					deviceToAllocs[deviceIdTuple] = allocDeviceGrp
				}
				allocDeviceGrp.allocs = append(allocDeviceGrp.allocs, alloc)
				allocDeviceGrp.deviceInstances[alloc.ID] += len(device.DeviceIDs)
			}
		}
	}

	neededCount := ask.Count

	var preemptionOptions []*deviceGroupAllocs
	// Examine matching allocs by device
OUTER:
	for deviceIDTuple, allocsGrp := range deviceToAllocs {
		// First group and sort allocations using this device by priority
		allocsByPriority := filterAndGroupPreemptibleAllocs(p.jobPriority, allocsGrp.allocs)

		// Reset preempted count for this device
		preemptedCount := 0

		// Initialize slice of preempted allocations
		var preemptedAllocs []*structs.Allocation

		for _, grpAllocs := range allocsByPriority {
			for _, alloc := range grpAllocs.allocs {
				// Look up the device instance from the device allocator
				devInst := devAlloc.Devices[deviceIDTuple]

				// Add to preemption list because this device matches
				preemptedCount += allocsGrp.deviceInstances[alloc.ID]
				preemptedAllocs = append(preemptedAllocs, alloc)

				// Check if we met needed count
				if preemptedCount+devInst.FreeCount() >= int(neededCount) {
					preemptionOptions = append(preemptionOptions, &deviceGroupAllocs{
						allocs:          preemptedAllocs,
						deviceInstances: allocsGrp.deviceInstances,
					})
					continue OUTER
				}
			}
		}
	}

	// Find the combination of allocs with lowest net priority
	if len(preemptionOptions) > 0 {
		return selectBestAllocs(preemptionOptions, int(neededCount))
	}

	return nil
}

// selectBestAllocs finds the best allocations based on minimal net priority amongst
// all options. The net priority is the sum of unique priorities in each option
func selectBestAllocs(preemptionOptions []*deviceGroupAllocs, neededCount int) []*structs.Allocation {
	bestPriority := math.MaxInt32
	var bestAllocs []*structs.Allocation

	// We iterate over allocations in priority order, so its possible
	// that we have more allocations than needed to meet the needed count.
	// e.g we need 4 instances, and we get 3 from a priority 10 alloc, and 4 from
	// a priority 20 alloc. We should filter out the priority 10 alloc in that case.
	// This loop does a filter and chooses the set with the smallest net priority
	for _, allocGrp := range preemptionOptions {
		// Find unique priorities and add them to calculate net priority
		priorities := map[int]struct{}{}
		netPriority := 0

		devInst := allocGrp.deviceInstances
		var filteredAllocs []*structs.Allocation

		// Sort by number of device instances used, descending
		sort.Slice(allocGrp.allocs, func(i, j int) bool {
			instanceCount1 := devInst[allocGrp.allocs[i].ID]
			instanceCount2 := devInst[allocGrp.allocs[j].ID]
			return instanceCount1 > instanceCount2
		})

		// Filter and calculate net priority
		preemptedInstanceCount := 0
		for _, alloc := range allocGrp.allocs {
			if preemptedInstanceCount >= neededCount {
				break
			}
			instanceCount := devInst[alloc.ID]
			preemptedInstanceCount += instanceCount
			filteredAllocs = append(filteredAllocs, alloc)
			_, ok := priorities[alloc.Job.Priority]
			if !ok {
				priorities[alloc.Job.Priority] = struct{}{}
				netPriority += alloc.Job.Priority
			}
		}
		if netPriority < bestPriority {
			bestPriority = netPriority
			bestAllocs = filteredAllocs
		}
	}
	return bestAllocs
}

// basicResourceDistance computes a distance using a coordinate system. It compares resource fields like CPU/Memory and Disk.
// Values emitted are in the range [0, maxFloat]
func basicResourceDistance(resourceAsk *structs.ComparableResources, resourceUsed *structs.ComparableResources) float64 {
	memoryCoord, cpuCoord, diskMBCoord := 0.0, 0.0, 0.0
	if resourceAsk.Flattened.Memory.MemoryMB > 0 {
		memoryCoord = (float64(resourceAsk.Flattened.Memory.MemoryMB) - float64(resourceUsed.Flattened.Memory.MemoryMB)) / float64(resourceAsk.Flattened.Memory.MemoryMB)
	}
	if resourceAsk.Flattened.Cpu.CpuShares > 0 {
		cpuCoord = (float64(resourceAsk.Flattened.Cpu.CpuShares) - float64(resourceUsed.Flattened.Cpu.CpuShares)) / float64(resourceAsk.Flattened.Cpu.CpuShares)
	}
	if resourceAsk.Shared.DiskMB > 0 {
		diskMBCoord = (float64(resourceAsk.Shared.DiskMB) - float64(resourceUsed.Shared.DiskMB)) / float64(resourceAsk.Shared.DiskMB)
	}
	originDist := math.Sqrt(
		math.Pow(memoryCoord, 2) +
			math.Pow(cpuCoord, 2) +
			math.Pow(diskMBCoord, 2))
	return originDist
}

// networkResourceDistance returns a distance based only on network megabits
func networkResourceDistance(resourceUsed *structs.NetworkResource, resourceNeeded *structs.NetworkResource) float64 {
	networkCoord := math.MaxFloat64
	if resourceUsed != nil && resourceNeeded != nil {
		networkCoord = float64(resourceNeeded.MBits-resourceUsed.MBits) / float64(resourceNeeded.MBits)
	}

	originDist := math.Abs(networkCoord)
	return originDist
}

// scoreForTaskGroup is used to calculate a score (lower is better) based on the distance between
// the needed resource and requirements. A penalty is added when the choice already has some existing
// allocations in the plan that are being preempted.
func scoreForTaskGroup(resourceAsk *structs.ComparableResources, resourceUsed *structs.ComparableResources, maxParallel int, numPreemptedAllocs int) float64 {
	maxParallelScorePenalty := 0.0
	if maxParallel > 0 && numPreemptedAllocs >= maxParallel {
		maxParallelScorePenalty = float64((numPreemptedAllocs+1)-maxParallel) * maxParallelPenalty
	}
	return basicResourceDistance(resourceAsk, resourceUsed) + maxParallelScorePenalty
}

// scoreForNetwork is similar to scoreForTaskGroup
// but only uses network Mbits to calculate a preemption score
func scoreForNetwork(resourceUsed *structs.NetworkResource, resourceNeeded *structs.NetworkResource, maxParallel int, numPreemptedAllocs int) float64 {
	if resourceUsed == nil || resourceNeeded == nil {
		return math.MaxFloat64
	}
	maxParallelScorePenalty := 0.0
	if maxParallel > 0 && numPreemptedAllocs >= maxParallel {
		maxParallelScorePenalty = float64((numPreemptedAllocs+1)-maxParallel) * maxParallelPenalty
	}
	return networkResourceDistance(resourceUsed, resourceNeeded) + maxParallelScorePenalty
}

// filterAndGroupPreemptibleAllocs groups allocations by priority after filtering allocs
// that are not preemptible based on the jobPriority arg
func filterAndGroupPreemptibleAllocs(jobPriority int, current []*structs.Allocation) []*groupedAllocs {
	allocsByPriority := make(map[int][]*structs.Allocation)
	for _, alloc := range current {
		if alloc.Job == nil {
			continue
		}

		// Skip allocs whose priority is within a delta of 10
		// This also skips any allocs of the current job
		// for which we are attempting preemption
		if jobPriority-alloc.Job.Priority < 10 {
			continue
		}
		grpAllocs, ok := allocsByPriority[alloc.Job.Priority]
		if !ok {
			grpAllocs = make([]*structs.Allocation, 0)
		}
		grpAllocs = append(grpAllocs, alloc)
		allocsByPriority[alloc.Job.Priority] = grpAllocs
	}

	var groupedSortedAllocs []*groupedAllocs
	for priority, allocs := range allocsByPriority {
		groupedSortedAllocs = append(groupedSortedAllocs, &groupedAllocs{
			priority: priority,
			allocs:   allocs})
	}

	// Sort by priority
	sort.Slice(groupedSortedAllocs, func(i, j int) bool {
		return groupedSortedAllocs[i].priority < groupedSortedAllocs[j].priority
	})

	return groupedSortedAllocs
}

// filterSuperset is used as a final step to remove
// any allocations that meet a superset of requirements from
// the set of allocations to preempt
func (p *Preemptor) filterSuperset(bestAllocs []*structs.Allocation,
	nodeRemainingResources *structs.ComparableResources,
	resourceAsk *structs.ComparableResources,
	preemptionResourceFactory PreemptionResourceFactory) []*structs.Allocation {

	// Sort bestAllocs by distance descending (without penalty)
	sort.Slice(bestAllocs, func(i, j int) bool {
		a1Resources := p.allocDetails[bestAllocs[i].ID].resources
		a2Resources := p.allocDetails[bestAllocs[j].ID].resources
		distance1 := preemptionResourceFactory(a1Resources, resourceAsk).Distance()
		distance2 := preemptionResourceFactory(a2Resources, resourceAsk).Distance()
		return distance1 > distance2
	})

	availableResources := nodeRemainingResources.Copy()
	var filteredBestAllocs []*structs.Allocation

	// Do another pass to eliminate allocations that are a superset of other allocations
	// in the preemption set
	for _, alloc := range bestAllocs {
		filteredBestAllocs = append(filteredBestAllocs, alloc)
		allocResources := p.allocDetails[alloc.ID].resources
		availableResources.Add(allocResources)

		premptionResource := preemptionResourceFactory(availableResources, resourceAsk)
		requirementsMet := premptionResource.MeetsRequirements()
		if requirementsMet {
			break
		}
	}
	return filteredBestAllocs
}

// distanceComparatorForNetwork is used as the sorting function when finding allocations to preempt. It uses
// both a coordinate distance function based on Mbits needed, and a penalty if the allocation under consideration
// belongs to a job that already has more preempted allocations
func (p *Preemptor) distanceComparatorForNetwork(allocs []*structs.Allocation, networkResourceAsk *structs.NetworkResource, i int, j int) bool {
	firstAlloc := allocs[i]
	currentPreemptionCount1 := p.getNumPreemptions(firstAlloc)

	// Look up configured maxParallel value for these allocation's task groups
	var maxParallel1, maxParallel2 int
	tg1 := allocs[i].Job.LookupTaskGroup(firstAlloc.TaskGroup)
	if tg1 != nil && tg1.Migrate != nil {
		maxParallel1 = tg1.Migrate.MaxParallel
	}

	// Dereference network usage on first alloc if its there
	firstAllocResources := p.allocDetails[firstAlloc.ID].resources
	firstAllocNetworks := firstAllocResources.Flattened.Networks
	var firstAllocNetResourceUsed *structs.NetworkResource
	if len(firstAllocNetworks) > 0 {
		firstAllocNetResourceUsed = firstAllocNetworks[0]
	}

	distance1 := scoreForNetwork(firstAllocNetResourceUsed, networkResourceAsk, maxParallel1, currentPreemptionCount1)

	secondAlloc := allocs[j]
	currentPreemptionCount2 := p.getNumPreemptions(secondAlloc)
	tg2 := secondAlloc.Job.LookupTaskGroup(secondAlloc.TaskGroup)
	if tg2 != nil && tg2.Migrate != nil {
		maxParallel2 = tg2.Migrate.MaxParallel
	}

	// Dereference network usage on second alloc if its there
	secondAllocResources := p.allocDetails[secondAlloc.ID].resources
	secondAllocNetworks := secondAllocResources.Flattened.Networks
	var secondAllocNetResourceUsed *structs.NetworkResource
	if len(secondAllocNetworks) > 0 {
		secondAllocNetResourceUsed = secondAllocNetworks[0]
	}

	distance2 := scoreForNetwork(secondAllocNetResourceUsed, networkResourceAsk, maxParallel2, currentPreemptionCount2)
	return distance1 < distance2
}
