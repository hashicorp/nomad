package scheduler

import (
	"math"
	"sort"

	"github.com/hashicorp/nomad/nomad/structs"
)

// maxParallelPenalty is a score penalty applied to allocations to mitigate against
// too many allocations of the same job being preempted. This penalty is applied after the
// number of allocations being preempted exceeds max_parallel value in the job's migrate stanza
const maxParallelPenalty = 50.0

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

// getPreemptionScoreForTaskGroupResources is used to calculate a score (lower is better) based on the distance between
// the needed resource and requirements. A penalty is added when the choice already has some existing
// allocations in the plan that are being preempted.
func getPreemptionScoreForTaskGroupResources(resourceAsk *structs.ComparableResources, resourceUsed *structs.ComparableResources, maxParallel int, numPreemptedAllocs int) float64 {
	maxParallelScorePenalty := 0.0
	if maxParallel > 0 && numPreemptedAllocs >= maxParallel {
		maxParallelScorePenalty = float64((numPreemptedAllocs+1)-maxParallel) * maxParallelPenalty
	}
	return basicResourceDistance(resourceAsk, resourceUsed) + maxParallelScorePenalty
}

// getPreemptionScoreForNetwork is similar to getPreemptionScoreForTaskGroupResources but only uses network mbits to calculate a preemption score
func getPreemptionScoreForNetwork(resourceUsed *structs.NetworkResource, resourceNeeded *structs.NetworkResource, maxParallel int, numPreemptedAllocs int) float64 {
	if resourceUsed == nil || resourceNeeded == nil {
		return math.MaxFloat64
	}
	maxParallelScorePenalty := 0.0
	if maxParallel > 0 && numPreemptedAllocs >= maxParallel {
		maxParallelScorePenalty = float64((numPreemptedAllocs+1)-maxParallel) * maxParallelPenalty
	}
	return networkResourceDistance(resourceUsed, resourceNeeded) + maxParallelScorePenalty
}

// networkResourceDistance returns distance based on network megabits
func networkResourceDistance(resourceUsed *structs.NetworkResource, resourceNeeded *structs.NetworkResource) float64 {
	networkCoord := math.MaxFloat64
	if resourceUsed != nil && resourceNeeded != nil {
		networkCoord = float64(resourceNeeded.MBits-resourceUsed.MBits) / float64(resourceNeeded.MBits)
	}

	originDist := math.Sqrt(
		math.Pow(networkCoord, 2))
	return originDist
}

// findPreemptibleAllocationsForTaskGroup computes a list of allocations to preempt to accommodate
// the resources asked for. Only allocs with a job priority < 10 of jobPriority are considered
// This method is used after network resource needs have already been met.
func findPreemptibleAllocationsForTaskGroup(jobPriority int, current []*structs.Allocation, resourceAsk *structs.AllocatedResources, node *structs.Node, currentPreemptions []*structs.Allocation) []*structs.Allocation {
	resourcesNeeded := resourceAsk.Comparable()
	allocsByPriority := filterAndGroupPreemptibleAllocs(jobPriority, current)
	var bestAllocs []*structs.Allocation
	allRequirementsMet := false
	var preemptedResources *structs.ComparableResources

	//TODO(preetha): should add some debug logging

	nodeRemainingResources := node.ComparableResources()

	// Initialize nodeRemainingResources with the remaining resources
	// after accounting for reserved resources and all allocations

	// Subtract the reserved resources of the node
	if node.ComparableReservedResources() != nil {
		nodeRemainingResources.Subtract(node.ComparableReservedResources())
	}

	// Subtract current allocations
	for _, alloc := range current {
		nodeRemainingResources.Subtract(alloc.ComparableResources())
	}

	// Iterate over allocations grouped by priority to find preemptible allocations
	for _, allocGrp := range allocsByPriority {
		for len(allocGrp.allocs) > 0 && !allRequirementsMet {
			closestAllocIndex := -1
			bestDistance := math.MaxFloat64
			// find the alloc with the closest distance
			for index, alloc := range allocGrp.allocs {
				currentPreemptionCount := computeCurrentPreemptions(alloc, currentPreemptions)
				maxParallel := 0
				tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
				if tg != nil && tg.Migrate != nil {
					maxParallel = tg.Migrate.MaxParallel
				}
				distance := getPreemptionScoreForTaskGroupResources(resourcesNeeded, alloc.ComparableResources(), maxParallel, currentPreemptionCount)
				if distance < bestDistance {
					bestDistance = distance
					closestAllocIndex = index
				}
			}
			closestAlloc := allocGrp.allocs[closestAllocIndex]

			if preemptedResources == nil {
				preemptedResources = closestAlloc.ComparableResources()
			} else {
				preemptedResources.Add(closestAlloc.ComparableResources())
			}
			availableResources := preemptedResources.Copy()
			availableResources.Add(nodeRemainingResources)

			// This step needs the original resources asked for as the second arg, can't use the running total
			allRequirementsMet, _ = availableResources.Superset(resourceAsk.Comparable())

			bestAllocs = append(bestAllocs, closestAlloc)

			allocGrp.allocs[closestAllocIndex] = allocGrp.allocs[len(allocGrp.allocs)-1]
			allocGrp.allocs = allocGrp.allocs[:len(allocGrp.allocs)-1]

			// this is the remaining total of resources needed
			resourcesNeeded.Subtract(closestAlloc.ComparableResources())
		}
		if allRequirementsMet {
			break
		}
	}

	// Early return if all allocs examined and requirements were not met
	if !allRequirementsMet {
		return nil
	}

	resourcesNeeded = resourceAsk.Comparable()
	// We do another pass to eliminate unnecessary preemptions
	// This filters out allocs whose resources are already covered by another alloc

	// Sort bestAllocs by distance descending (without penalty)
	sort.Slice(bestAllocs, func(i, j int) bool {
		distance1 := basicResourceDistance(resourcesNeeded, bestAllocs[i].ComparableResources())
		distance2 := basicResourceDistance(resourcesNeeded, bestAllocs[j].ComparableResources())
		return distance1 > distance2
	})

	filteredBestAllocs := eliminateSuperSetAllocationsForTaskGroup(bestAllocs, nodeRemainingResources, resourcesNeeded)
	return filteredBestAllocs

}

// computeCurrentPreemptions counts the number of other allocations being preempted that match the job and task group of
// the alloc under consideration. This is used as a scoring factor to minimize too many allocs of the same job being preempted at once
func computeCurrentPreemptions(currentAlloc *structs.Allocation, currentPreemptions []*structs.Allocation) int {
	numCurrentPreemptionsForJob := 0
	for _, alloc := range currentPreemptions {
		if alloc.JobID == currentAlloc.JobID && alloc.Namespace == currentAlloc.Namespace && alloc.TaskGroup == currentAlloc.TaskGroup {
			numCurrentPreemptionsForJob++
		}
	}
	return numCurrentPreemptionsForJob
}

// meetsNetworkRequirements checks if the first resource meets or exceeds the second resource's network MBits requirements
func meetsNetworkRequirements(firstMbits int, secondMbits int) bool {
	if firstMbits == 0 || secondMbits == 0 {
		return false
	}
	return firstMbits >= secondMbits
}

type groupedAllocs struct {
	priority int
	allocs   []*structs.Allocation
}

func filterAndGroupPreemptibleAllocs(jobPriority int, current []*structs.Allocation) []*groupedAllocs {
	allocsByPriority := make(map[int][]*structs.Allocation)
	for _, alloc := range current {
		// Why is alloc.Job even nil though?
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

func eliminateSuperSetAllocationsForTaskGroup(bestAllocs []*structs.Allocation,
	nodeRemainingResources *structs.ComparableResources,
	resourceAsk *structs.ComparableResources) []*structs.Allocation {

	var preemptedResources *structs.ComparableResources
	var filteredBestAllocs []*structs.Allocation

	// Do another pass to eliminate allocations that are a superset of other allocations
	// in the preemption set
	for _, alloc := range bestAllocs {
		if preemptedResources == nil {
			preemptedResources = alloc.ComparableResources().Copy()
		} else {
			preemptedResources.Add(alloc.ComparableResources().Copy())
		}
		filteredBestAllocs = append(filteredBestAllocs, alloc)
		availableResources := preemptedResources.Copy()
		availableResources.Add(nodeRemainingResources)

		requirementsMet, _ := availableResources.Superset(resourceAsk)
		if requirementsMet {
			break
		}
	}
	return filteredBestAllocs
}

func eliminateSuperSetAllocationsForNetwork(bestAllocs []*structs.Allocation, networkResourcesAsk *structs.NetworkResource,
	nodeRemainingResources *structs.ComparableResources) []*structs.Allocation {

	var preemptedResources *structs.ComparableResources
	var filteredBestAllocs []*structs.Allocation

	// Do another pass to eliminate allocations that are a superset of other allocations
	// in the preemption set
	for _, alloc := range bestAllocs {
		if preemptedResources == nil {
			preemptedResources = alloc.ComparableResources().Copy()
		} else {
			preemptedResources.Add(alloc.ComparableResources().Copy())
		}
		filteredBestAllocs = append(filteredBestAllocs, alloc)
		availableResources := preemptedResources.Copy()
		availableResources.Add(nodeRemainingResources)

		requirementsMet := meetsNetworkRequirements(availableResources.Flattened.Networks[0].MBits, networkResourcesAsk.MBits)
		if requirementsMet {
			break
		}
	}
	return filteredBestAllocs
}

// preemptForNetworkResourceAsk tries to find allocations to preempt to meet network resources.
// this needs to consider network resources at the task level and for the same task it should
// only preempt allocations that share the same network device
func preemptForNetworkResourceAsk(jobPriority int, currentAllocs []*structs.Allocation, networkResourceAsk *structs.NetworkResource,
	netIdx *structs.NetworkIndex, currentPreemptions []*structs.Allocation) []*structs.Allocation {

	// Early return if there are no current allocs
	if len(currentAllocs) == 0 {
		return nil
	}

	deviceToAllocs := make(map[string][]*structs.Allocation)
	MbitsNeeded := networkResourceAsk.MBits
	reservedPortsNeeded := networkResourceAsk.ReservedPorts

	// Create a map from each device to allocs
	// We do this because to place a task we have to be able to
	// preempt allocations that are using the same device.
	//
	// This step also filters out high priority allocations and allocations
	// that are not using any network resources
	for _, alloc := range currentAllocs {
		if alloc.Job == nil {
			continue
		}

		if jobPriority-alloc.Job.Priority < 10 {
			continue
		}
		networks := alloc.CompatibleNetworkResources()
		if len(networks) > 0 {
			device := networks[0].Device
			allocsForDevice := deviceToAllocs[device]
			allocsForDevice = append(allocsForDevice, alloc)
			deviceToAllocs[device] = allocsForDevice
		}
	}

	// If no existing allocations use network resources, return early
	if len(deviceToAllocs) == 0 {
		return nil
	}

	var allocsToPreempt []*structs.Allocation

	met := false
	freeBandwidth := 0

	for device, currentAllocs := range deviceToAllocs {
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

		// Build map from used reserved ports to allocation
		usedPortToAlloc := make(map[int]*structs.Allocation)

		// First try to satisfy needed reserved ports
		if len(reservedPortsNeeded) > 0 {
			for _, alloc := range currentAllocs {
				for _, tr := range alloc.TaskResources {
					reservedPorts := tr.Networks[0].ReservedPorts
					for _, p := range reservedPorts {
						usedPortToAlloc[p.Value] = alloc
					}
				}
			}

			// Look for allocs that are using reserved ports needed
			for _, port := range reservedPortsNeeded {
				alloc, ok := usedPortToAlloc[port.Value]
				if ok {
					preemptedBandwidth += alloc.Resources.Networks[0].MBits
					allocsToPreempt = append(allocsToPreempt, alloc)
				}
			}

			// Remove allocs that were preempted to satisfy reserved ports
			currentAllocs = structs.RemoveAllocs(currentAllocs, allocsToPreempt)
		}

		// If bandwidth requirements have been met, stop
		if preemptedBandwidth+freeBandwidth >= MbitsNeeded {
			break
		}

		// Split by priority
		allocsByPriority := filterAndGroupPreemptibleAllocs(jobPriority, currentAllocs)

		for _, allocsGrp := range allocsByPriority {
			allocs := allocsGrp.allocs

			// Sort by distance function that takes into account needed MBits
			// as well as penalty for preempting an allocation
			// whose task group already has existing preemptions
			sort.Slice(allocs, func(i, j int) bool {
				return distanceComparatorForNetwork(allocs, currentPreemptions, networkResourceAsk, i, j)
			})

			for _, alloc := range allocs {
				preemptedBandwidth += alloc.Resources.Networks[0].MBits
				allocsToPreempt = append(allocsToPreempt, alloc)
				if preemptedBandwidth+freeBandwidth >= MbitsNeeded {
					met = true
					break
				}
			}
			// If we met bandwidth needs we can break out of loop that's iterating by priority within a device
			if met {
				break
			}
		}
		// If we met bandwidth needs we can break out of loop that's iterating by allocs sharing the same network device
		if met {
			break
		}
	}
	if len(allocsToPreempt) == 0 {
		return nil
	}

	// Build a resource object with just the network Mbits filled in
	// Its safe to use the first preempted allocation's network resource
	// here because all allocations preempted will be from the same device
	nodeRemainingResources := &structs.ComparableResources{
		Flattened: structs.AllocatedTaskResources{
			Networks: []*structs.NetworkResource{
				{
					Device: allocsToPreempt[0].Resources.Networks[0].Device,
					MBits:  freeBandwidth,
				},
			},
		},
	}

	// Sort by distance reversed to surface any superset allocs first
	// This sort only looks at mbits because we should still not prefer
	// allocs that have a maxParallel penalty
	sort.Slice(allocsToPreempt, func(i, j int) bool {
		firstAlloc := allocsToPreempt[i]
		secondAlloc := allocsToPreempt[j]

		firstAllocNetworks := firstAlloc.ComparableResources().Flattened.Networks
		var firstAllocNetResourceUsed *structs.NetworkResource
		if len(firstAllocNetworks) > 0 {
			firstAllocNetResourceUsed = firstAllocNetworks[0]
		}

		secondAllocNetworks := secondAlloc.ComparableResources().Flattened.Networks
		var secondAllocNetResourceUsed *structs.NetworkResource
		if len(secondAllocNetworks) > 0 {
			secondAllocNetResourceUsed = secondAllocNetworks[0]
		}

		distance1 := networkResourceDistance(firstAllocNetResourceUsed, networkResourceAsk)
		distance2 := networkResourceDistance(secondAllocNetResourceUsed, networkResourceAsk)
		return distance1 > distance2
	})

	// Do a final pass to eliminate any superset allocations
	filteredBestAllocs := eliminateSuperSetAllocationsForNetwork(allocsToPreempt, networkResourceAsk, nodeRemainingResources)
	return filteredBestAllocs
}

func distanceComparatorForNetwork(allocs []*structs.Allocation, currentPreemptions []*structs.Allocation, networkResourceAsk *structs.NetworkResource, i int, j int) bool {
	firstAlloc := allocs[i]
	currentPreemptionCount1 := computeCurrentPreemptions(firstAlloc, currentPreemptions)
	// Look up configured maxParallel value for these allocation's task groups
	var maxParallel1, maxParallel2 int
	tg1 := allocs[i].Job.LookupTaskGroup(firstAlloc.TaskGroup)
	if tg1 != nil && tg1.Migrate != nil {
		maxParallel1 = tg1.Migrate.MaxParallel
	}
	// Dereference network usage on first alloc if its there
	firstAllocNetworks := firstAlloc.CompatibleNetworkResources()
	var firstAllocNetResourceUsed *structs.NetworkResource
	if len(firstAllocNetworks) > 0 {
		firstAllocNetResourceUsed = firstAllocNetworks[0]
	}

	distance1 := getPreemptionScoreForNetwork(firstAllocNetResourceUsed, networkResourceAsk, maxParallel1, currentPreemptionCount1)

	secondAlloc := allocs[j]
	currentPreemptionCount2 := computeCurrentPreemptions(secondAlloc, currentPreemptions)
	tg2 := secondAlloc.Job.LookupTaskGroup(secondAlloc.TaskGroup)
	if tg2 != nil && tg2.Migrate != nil {
		maxParallel2 = tg2.Migrate.MaxParallel
	}
	// Dereference network usage on first alloc if its there
	secondAllocNetworks := secondAlloc.CompatibleNetworkResources()
	var secondAllocNetResourceUsed *structs.NetworkResource
	if len(secondAllocNetworks) > 0 {
		secondAllocNetResourceUsed = secondAllocNetworks[0]
	}

	distance2 := getPreemptionScoreForNetwork(secondAllocNetResourceUsed, networkResourceAsk, maxParallel2, currentPreemptionCount2)
	return distance1 < distance2
}
