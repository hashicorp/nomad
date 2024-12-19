// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"fmt"
	"math"
	"slices"

	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/safemath"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// binPackingMaxFitScore is the maximum possible bin packing fitness score.
	// This is used to normalize bin packing score to a value between 0 and 1
	binPackingMaxFitScore = 18.0
)

// Rank is used to provide a score and various ranking metadata
// along with a node when iterating. This state can be modified as
// various rank methods are applied.
type RankedNode struct {
	Node           *structs.Node
	FinalScore     float64
	Scores         []float64
	TaskResources  map[string]*structs.AllocatedTaskResources
	TaskLifecycles map[string]*structs.TaskLifecycleConfig
	AllocResources *structs.AllocatedSharedResources

	// Proposed is used to cache the proposed allocations on the
	// node. This can be shared between iterators that require it.
	Proposed []*structs.Allocation

	// PreemptedAllocs is used by the BinpackIterator to identify allocs
	// that should be preempted in order to make the placement
	PreemptedAllocs []*structs.Allocation
}

func (r *RankedNode) GoString() string {
	return fmt.Sprintf("<Node: %s Score: %0.3f>", r.Node.ID, r.FinalScore)
}

func (r *RankedNode) ProposedAllocs(ctx Context) ([]*structs.Allocation, error) {
	if r.Proposed != nil {
		return r.Proposed, nil
	}

	p, err := ctx.ProposedAllocs(r.Node.ID)
	if err != nil {
		return nil, err
	}
	r.Proposed = p
	return p, nil
}

func (r *RankedNode) SetTaskResources(
	task *structs.Task,
	resource *structs.AllocatedTaskResources,
) {
	if r.TaskResources == nil {
		r.TaskResources = make(map[string]*structs.AllocatedTaskResources)
		r.TaskLifecycles = make(map[string]*structs.TaskLifecycleConfig)
	}
	r.TaskResources[task.Name] = resource
	r.TaskLifecycles[task.Name] = task.Lifecycle
}

// RankIterator is used to iteratively yield nodes along
// with ranking metadata. The iterators may manage some state for
// performance optimizations.
type RankIterator interface {
	// Next yields a ranked option or nil if exhausted
	Next() *RankedNode

	// Reset is invoked when an allocation has been placed
	// to reset any stale state.
	Reset()
}

// FeasibleRankIterator is used to consume from a FeasibleIterator
// and return an unranked node with base ranking.
type FeasibleRankIterator struct {
	ctx    Context
	source FeasibleIterator
}

// NewFeasibleRankIterator is used to return a new FeasibleRankIterator
// from a FeasibleIterator source.
func NewFeasibleRankIterator(ctx Context, source FeasibleIterator) *FeasibleRankIterator {
	iter := &FeasibleRankIterator{
		ctx:    ctx,
		source: source,
	}
	return iter
}

func (iter *FeasibleRankIterator) Next() *RankedNode {
	option := iter.source.Next()
	if option == nil {
		return nil
	}
	ranked := &RankedNode{
		Node: option,
	}
	return ranked
}

func (iter *FeasibleRankIterator) Reset() {
	iter.source.Reset()
}

// StaticRankIterator is a RankIterator that returns a static set of results.
// This is largely only useful for testing.
type StaticRankIterator struct {
	ctx    Context
	nodes  []*RankedNode
	offset int
	seen   int
}

// NewStaticRankIterator returns a new static rank iterator over the given nodes
func NewStaticRankIterator(ctx Context, nodes []*RankedNode) *StaticRankIterator {
	iter := &StaticRankIterator{
		ctx:   ctx,
		nodes: nodes,
	}
	return iter
}

func (iter *StaticRankIterator) Next() *RankedNode {
	// Check if exhausted
	n := len(iter.nodes)
	if iter.offset == n || iter.seen == n {
		if iter.seen != n {
			iter.offset = 0
		} else {
			return nil
		}
	}

	// Return the next offset
	offset := iter.offset
	iter.offset += 1
	iter.seen += 1
	return iter.nodes[offset]
}

func (iter *StaticRankIterator) Reset() {
	iter.seen = 0
}

// BinPackIterator is a RankIterator that scores potential options
// based on a bin-packing algorithm.
type BinPackIterator struct {
	ctx                    Context
	source                 RankIterator
	evict                  bool
	priority               int
	jobId                  structs.NamespacedID
	taskGroup              *structs.TaskGroup
	memoryOversubscription bool
	scoreFit               func(*structs.Node, *structs.ComparableResources) float64
}

// NewBinPackIterator returns a BinPackIterator which tries to fit tasks
// potentially evicting other tasks based on a given priority.
func NewBinPackIterator(ctx Context, source RankIterator, evict bool, priority int) *BinPackIterator {
	return &BinPackIterator{
		ctx:      ctx,
		source:   source,
		evict:    evict,
		priority: priority,

		// These are default values that may be overwritten by
		// SetSchedulerConfiguration.
		memoryOversubscription: false,
		scoreFit:               structs.ScoreFitBinPack,
	}
}

func (iter *BinPackIterator) SetJob(job *structs.Job) {
	iter.priority = job.Priority
	iter.jobId = job.NamespacedID()
}

func (iter *BinPackIterator) SetTaskGroup(taskGroup *structs.TaskGroup) {
	iter.taskGroup = taskGroup
}

func (iter *BinPackIterator) SetSchedulerConfiguration(schedConfig *structs.SchedulerConfiguration) {
	// Set scoring function.
	algorithm := schedConfig.EffectiveSchedulerAlgorithm()
	scoreFn := structs.ScoreFitBinPack
	if algorithm == structs.SchedulerAlgorithmSpread {
		scoreFn = structs.ScoreFitSpread
	}
	iter.scoreFit = scoreFn

	// Set memory oversubscription.
	iter.memoryOversubscription = schedConfig != nil && schedConfig.MemoryOversubscriptionEnabled
}

func (iter *BinPackIterator) Next() *RankedNode {

NEXTNODE:
	for {
		// Get the next potential option
		option := iter.source.Next()
		if option == nil {
			return nil
		}

		// Get the allocations that already exist on the node + those allocs
		// that have been placed as part of this same evaluation
		proposed, err := option.ProposedAllocs(iter.ctx)
		if err != nil {
			iter.ctx.Logger().Named("binpack").Error("failed retrieving proposed allocations", "error", err)
			continue
		}

		// Index the existing network usage.
		// This should never collide, since it represents the current state of
		// the node. If it does collide though, it means we found a bug! So
		// collect as much information as possible.
		netIdx := structs.NewNetworkIndex()
		if err := netIdx.SetNode(option.Node); err != nil {
			iter.ctx.SendEvent(&PortCollisionEvent{
				Reason:   err.Error(),
				NetIndex: netIdx.Copy(),
				Node:     option.Node,
			})
			iter.ctx.Metrics().ExhaustedNode(option.Node, "network: invalid node")
			continue
		}
		if collide, reason := netIdx.AddAllocs(proposed); collide {
			event := &PortCollisionEvent{
				Reason:      reason,
				NetIndex:    netIdx.Copy(),
				Node:        option.Node,
				Allocations: make([]*structs.Allocation, len(proposed)),
			}
			for i, alloc := range proposed {
				event.Allocations[i] = alloc.Copy()
			}
			iter.ctx.SendEvent(event)
			iter.ctx.Metrics().ExhaustedNode(option.Node, "network: port collision")
			continue
		}

		// Create a device allocator
		devAllocator := newDeviceAllocator(iter.ctx, option.Node)
		devAllocator.AddAllocs(proposed)

		// Track the affinities of the devices
		totalDeviceAffinityWeight := 0.0
		sumMatchingAffinities := 0.0

		// Assign the resources for each task
		total := &structs.AllocatedResources{
			Tasks: make(map[string]*structs.AllocatedTaskResources,
				len(iter.taskGroup.Tasks)),
			TaskLifecycles: make(map[string]*structs.TaskLifecycleConfig,
				len(iter.taskGroup.Tasks)),
			Shared: structs.AllocatedSharedResources{
				DiskMB: int64(iter.taskGroup.EphemeralDisk.SizeMB),
			},
		}

		var allocsToPreempt []*structs.Allocation

		// Initialize preemptor with node
		preemptor := NewPreemptor(iter.priority, iter.ctx, &iter.jobId)
		preemptor.SetNode(option.Node)

		// Count the number of existing preemptions
		allPreemptions := iter.ctx.Plan().NodePreemptions
		var currentPreemptions []*structs.Allocation
		for _, allocs := range allPreemptions {
			currentPreemptions = append(currentPreemptions, allocs...)
		}
		preemptor.SetPreemptions(currentPreemptions)

		// Check if we need task group network resource
		if len(iter.taskGroup.Networks) > 0 {
			ask := iter.taskGroup.Networks[0].Copy()
			for i, port := range ask.DynamicPorts {
				if port.HostNetwork != "" {
					if hostNetworkValue, hostNetworkOk := resolveTarget(port.HostNetwork, option.Node); hostNetworkOk {
						ask.DynamicPorts[i].HostNetwork = hostNetworkValue
					} else {
						iter.ctx.Logger().Named("binpack").Error(fmt.Sprintf("Invalid template for %s host network in port %s", port.HostNetwork, port.Label))
						netIdx.Release()
						continue NEXTNODE
					}
				}
			}
			for i, port := range ask.ReservedPorts {
				if port.HostNetwork != "" {
					if hostNetworkValue, hostNetworkOk := resolveTarget(port.HostNetwork, option.Node); hostNetworkOk {
						ask.ReservedPorts[i].HostNetwork = hostNetworkValue
					} else {
						iter.ctx.Logger().Named("binpack").Error(fmt.Sprintf("Invalid template for %s host network in port %s", port.HostNetwork, port.Label))
						netIdx.Release()
						continue NEXTNODE
					}
				}
			}
			offer, err := netIdx.AssignPorts(ask)
			if err != nil {
				// If eviction is not enabled, mark this node as exhausted and continue
				if !iter.evict {
					iter.ctx.Metrics().ExhaustedNode(option.Node,
						fmt.Sprintf("network: %s", err))
					netIdx.Release()
					continue NEXTNODE
				}

				// Look for preemptible allocations to satisfy the network resource for this task
				preemptor.SetCandidates(proposed)

				netPreemptions := preemptor.PreemptForNetwork(ask, netIdx)
				if netPreemptions == nil {
					iter.ctx.Logger().Named("binpack").Debug("preemption not possible ", "network_resource", ask)
					iter.ctx.Metrics().ExhaustedNode(option.Node,
						fmt.Sprintf("network: %s", err))
					netIdx.Release()
					continue NEXTNODE
				}
				allocsToPreempt = append(allocsToPreempt, netPreemptions...)

				// First subtract out preempted allocations
				proposed = structs.RemoveAllocs(proposed, netPreemptions)

				// Reset the network index and try the offer again
				netIdx.Release()
				netIdx = structs.NewNetworkIndex()
				netIdx.SetNode(option.Node)
				netIdx.AddAllocs(proposed)

				offer, err = netIdx.AssignPorts(ask)
				if err != nil {
					iter.ctx.Logger().Named("binpack").Debug("unexpected error, unable to create network offer after considering preemption", "error", err)
					iter.ctx.Metrics().ExhaustedNode(option.Node,
						fmt.Sprintf("network: %s", err))
					netIdx.Release()
					continue NEXTNODE
				}
			}

			// Reserve this to prevent another task from colliding
			netIdx.AddReservedPorts(offer)

			// Update the network ask to the offer
			nwRes := structs.AllocatedPortsToNetworkResouce(ask, offer, option.Node.NodeResources)
			total.Shared.Networks = []*structs.NetworkResource{nwRes}
			total.Shared.Ports = offer
			option.AllocResources = &structs.AllocatedSharedResources{
				Networks: []*structs.NetworkResource{nwRes},
				DiskMB:   int64(iter.taskGroup.EphemeralDisk.SizeMB),
				Ports:    offer,
			}

		}

		for _, task := range iter.taskGroup.Tasks {
			// Allocate the resources
			taskResources := &structs.AllocatedTaskResources{
				Cpu: structs.AllocatedCpuResources{
					CpuShares: int64(task.Resources.CPU),
				},
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: safemath.Add(
						int64(task.Resources.MemoryMB), int64(task.Resources.SecretsMB)),
				},
			}
			if iter.memoryOversubscription {
				taskResources.Memory.MemoryMaxMB = safemath.Add(
					int64(task.Resources.MemoryMaxMB), int64(task.Resources.SecretsMB))
			}

			// Check if we need a network resource
			if len(task.Resources.Networks) > 0 {
				ask := task.Resources.Networks[0].Copy()
				offer, err := netIdx.AssignTaskNetwork(ask)
				if offer == nil {
					// If eviction is not enabled, mark this node as exhausted and continue
					if !iter.evict {
						iter.ctx.Metrics().ExhaustedNode(option.Node,
							fmt.Sprintf("network: %s", err))
						netIdx.Release()
						continue NEXTNODE
					}

					// Look for preemptible allocations to satisfy the network resource for this task
					preemptor.SetCandidates(proposed)

					netPreemptions := preemptor.PreemptForNetwork(ask, netIdx)
					if netPreemptions == nil {
						iter.ctx.Logger().Named("binpack").Debug("preemption not possible ", "network_resource", ask)
						iter.ctx.Metrics().ExhaustedNode(option.Node,
							fmt.Sprintf("network: %s", err))
						netIdx.Release()
						continue NEXTNODE
					}
					allocsToPreempt = append(allocsToPreempt, netPreemptions...)

					// First subtract out preempted allocations
					proposed = structs.RemoveAllocs(proposed, netPreemptions)

					// Reset the network index and try the offer again
					netIdx.Release()
					netIdx = structs.NewNetworkIndex()
					netIdx.SetNode(option.Node)
					netIdx.AddAllocs(proposed)

					offer, err = netIdx.AssignTaskNetwork(ask)
					if offer == nil {
						iter.ctx.Logger().Named("binpack").Debug("unexpected error, unable to create network offer after considering preemption", "error", err)
						iter.ctx.Metrics().ExhaustedNode(option.Node,
							fmt.Sprintf("network: %s", err))
						netIdx.Release()
						continue NEXTNODE
					}
				}
				// Reserve this to prevent another task from colliding
				netIdx.AddReserved(offer)

				// Update the network ask to the offer
				taskResources.Networks = []*structs.NetworkResource{offer}
			}

			// Acquire devices

			// deviceMemoryNode will record which NUMA memory node our devices
			// connected to, or -1 to indicate we did not care
			deviceMemoryNode := -1

			// if there are no devices, skip over device assignments
			if len(task.Resources.Devices) == 0 {
				goto SELECTCORES
			}

			{
				// Attempt device assignments without pre-emption.
				//
				// This block will attempt to assign devices using the available
				// CPU cores and devices WITHOUT leveraging preemption to make
				// things fit. If this fails we do this logic again below but
				// with pre-emption logic.
				//
				// We do this so as to give priority to device allocation
				// options that do not involve killing other tasks, while still
				// ensuring we get the NUMA associativity the task is asking for.

				// set of already consumed cores on this node
				consumedCores := idset.Empty[hw.CoreID]()
				for _, alloc := range proposed {
					allocCores := alloc.AllocatedResources.Comparable().Flattened.Cpu.ReservedCores
					idset.InsertSlice(consumedCores, allocCores...)
				}

				// add cores reserved for other tasks
				for _, tr := range total.Tasks {
					taskCores := tr.Cpu.ReservedCores
					idset.InsertSlice(consumedCores, taskCores...)
				}

				nodeCores := option.Node.NodeResources.Processors.Topology.UsableCores()

				// usable cores not yet consumed for this node
				availableCores := nodeCores.Difference(consumedCores)

				// the memory nodes with sufficient cores for the task
				// resources, calculated by subtracting off all cores currently
				// in use because we are not allowing preemption
				candidateMemoryNodes := (&coreSelector{
					topology:       option.Node.NodeResources.Processors.Topology,
					availableCores: availableCores,
				}).candidateMemoryNodes(task.Resources)

				// snapshot the current state of device allocation, which we
				// will revert to each time we run into a problem while selecting
				// devices with memory node limitations
				devAllocatorSnapshot := devAllocator.Copy()
				taskResourcesSnapshot := slices.Clone(taskResources.Devices)
				sumMatchingAffinitiesSnapshot := sumMatchingAffinities
				totalDeviceAffinityWeightSnapshot := totalDeviceAffinityWeight

			SELECT_BY_NUMA_WITHOUT_EVICT:
				for _, candidateMemoryNode := range candidateMemoryNodes {
					deviceMemoryNode = candidateMemoryNode

					// attempt to assign devices using the given target memory
					// node
					count := 0
					for _, device := range task.Resources.Devices {
						memory := &memoryNodeMatcher{
							memoryNode: candidateMemoryNode,
							topology:   option.Node.NodeResources.Processors.Topology,
							devices:    set.From(task.Resources.NUMA.GetDevices()),
						}

						var offer *structs.AllocatedDeviceResource
						var sumAffinities float64
						offer, sumAffinities, err = devAllocator.createOffer(memory, device)
						if offer == nil || err != nil {
							devAllocator = devAllocatorSnapshot
							taskResources.Devices = taskResourcesSnapshot
							sumMatchingAffinities = sumMatchingAffinitiesSnapshot
							totalDeviceAffinityWeight = totalDeviceAffinityWeightSnapshot
							continue SELECT_BY_NUMA_WITHOUT_EVICT
						}

						// assign the offer for this device to our allocator
						devAllocator.AddReserved(offer)
						taskResources.Devices = append(taskResources.Devices, offer)

						// Add the scores
						if len(device.Affinities) != 0 {
							for _, a := range device.Affinities {
								totalDeviceAffinityWeight += math.Abs(float64(a.Weight))
							}
							sumMatchingAffinities += sumAffinities
						}
						count++
					}

					if count == len(task.Resources.Devices) {
						// We were able to allocate every device, no need to
						// try again using preemption. Skip on down to the
						// allocation of cpu cores.
						goto SELECTCORES
					}

					// reset allocation attempt to snapshot before trying with
					// next memory node option
					devAllocator = devAllocatorSnapshot
					taskResources.Devices = taskResourcesSnapshot
					sumMatchingAffinities = sumMatchingAffinitiesSnapshot
					totalDeviceAffinityWeight = totalDeviceAffinityWeightSnapshot
				}
			}

			{
				// Attempt device assignments with pre-emption.
				//
				// This block will attempt to assign devices using any CPU cores
				// and devices WITH leveraging preemption. We will have already
				// made attempts without preemption.

				// If preemption is not enabled, then this node is exhausted.
				if !iter.evict {
					// surface err from createOffer()
					iter.ctx.Metrics().ExhaustedNode(option.Node, fmt.Sprintf("devices: %s", err))
					continue NEXTNODE
				}

				// get a list of available memory nodes, including cores currently
				// in-use, which we can acquire by evicting tasks
				candidateMemoryNodes := (&coreSelector{
					topology:       option.Node.NodeResources.Processors.Topology,
					availableCores: option.Node.NodeResources.Processors.Topology.UsableCores(),
				}).candidateMemoryNodes(task.Resources)

				// snapshot the current state of device allocation, which we
				// will revert to each time we run into a problem while selecting
				// devices with memory node limitations
				devAllocatorSnapshot := devAllocator.Copy()
				taskResourcesSnapshot := slices.Clone(taskResources.Devices)
				sumMatchingAffinitiesSnapshot := sumMatchingAffinities
				totalDeviceAffinityWeightSnapshot := totalDeviceAffinityWeight
				preemptorSnapshot := preemptor.Copy()
				allocsToPreemptSnapshot := helper.CopySlice(allocsToPreempt)
				proposedSnapshot := helper.CopySlice(proposed)

				var offerErr error = nil

			SELECT_BY_NUMA_WITH_EVICT:
				for _, candidateMemoryNode := range candidateMemoryNodes {
					deviceMemoryNode = candidateMemoryNode

					// attempt to assign devices using the given target memory
					// node
					count := 0
					for _, device := range task.Resources.Devices {
						memory := &memoryNodeMatcher{
							memoryNode: candidateMemoryNode,
							topology:   option.Node.NodeResources.Processors.Topology,
							devices:    set.From(task.Resources.NUMA.GetDevices()),
						}

						offer, sumAffinities, err := devAllocator.createOffer(memory, device)
						if offer == nil {
							offerErr = err

							// get the potential preemptions
							preemptor.SetCandidates(proposed) // allocations
							devicePreemptions := preemptor.PreemptForDevice(device, devAllocator)

							restoreSnapshots := func() {
								devAllocator = devAllocatorSnapshot
								taskResources.Devices = taskResourcesSnapshot
								sumMatchingAffinities = sumMatchingAffinitiesSnapshot
								totalDeviceAffinityWeight = totalDeviceAffinityWeightSnapshot
								preemptor = preemptorSnapshot
								allocsToPreempt = allocsToPreemptSnapshot
								proposed = proposedSnapshot
							}

							// not able to assign device even with preemption,
							// reset to snapshots and try next memory node
							if devicePreemptions == nil {
								restoreSnapshots()
								continue SELECT_BY_NUMA_WITH_EVICT
							}

							allocsToPreempt = append(allocsToPreempt, devicePreemptions...)

							// subtract out preempted allocations
							proposed = structs.RemoveAllocs(proposed, allocsToPreempt)

							// use a device allocator with new set of proposed allocs
							devAllocatorEvict := newDeviceAllocator(iter.ctx, option.Node)
							devAllocatorEvict.AddAllocs(proposed)

							// attempt the offer again
							offerEvict, sumAffinitiesEvict, err := devAllocatorEvict.createOffer(memory, device)
							if offerEvict == nil || err != nil {
								// we cannot acquire this device even with preemption
								iter.ctx.Logger().Named("binpack").Debug("unexpected error, unable to create device offer after considering preemption", "error", err)
								iter.ctx.Metrics().ExhaustedNode(option.Node, fmt.Sprintf("devices: %s", err))
								continue NEXTNODE
							}

							offer = offerEvict
							sumAffinities = sumAffinitiesEvict
						}

						// assign the offer for this device to our allocator
						devAllocator.AddReserved(offer)
						taskResources.Devices = append(taskResources.Devices, offer)

						// Add the scores
						if len(device.Affinities) != 0 {
							for _, a := range device.Affinities {
								totalDeviceAffinityWeight += math.Abs(float64(a.Weight))
							}
							sumMatchingAffinities += sumAffinities
						}
						count++
					}

					if count == len(task.Resources.Devices) {
						// We were able to allocate every device.
						goto SELECTCORES
					}
				}

				// We were not able to allocate every device, implying
				// this node could not support the device ask.
				iter.ctx.Logger().Named("binpack").Debug("preemption not possible")
				iter.ctx.Metrics().ExhaustedNode(option.Node, fmt.Sprintf("devices: %s", offerErr))
				netIdx.Release()
				continue NEXTNODE

			} // preempt attempt

		SELECTCORES:

			// Handle CPU core reservations
			if wantedCores := task.Resources.Cores; wantedCores > 0 {
				// set of cores on this node allowable for use by nomad
				nodeCores := option.Node.NodeResources.Processors.Topology.UsableCores()

				// set of consumed cores on this node
				consumedCores := idset.Empty[hw.CoreID]()
				for _, alloc := range proposed { // proposed is existing + proposal
					allocCores := alloc.AllocatedResources.Comparable().Flattened.Cpu.ReservedCores
					idset.InsertSlice(consumedCores, allocCores...)
				}

				// add cores reserved for other tasks
				for _, tr := range total.Tasks {
					taskCores := tr.Cpu.ReservedCores
					idset.InsertSlice(consumedCores, taskCores...)
				}

				// usable cores not yet reserved on this node
				availableCores := nodeCores.Difference(consumedCores)

				// mark the node as exhausted if not enough cores available
				if availableCores.Size() < wantedCores {
					iter.ctx.Metrics().ExhaustedNode(option.Node, "cores")
					continue NEXTNODE
				}

				// set the task's reserved cores
				cores, bandwidth := (&coreSelector{
					topology:         option.Node.NodeResources.Processors.Topology,
					availableCores:   availableCores,
					shuffle:          randomizeCores,
					deviceMemoryNode: deviceMemoryNode,
				}).Select(task.Resources)

				// mark the node as exhausted if not enough cores available given
				// the NUMA preference
				if cores == nil {
					iter.ctx.Metrics().ExhaustedNode(option.Node, "numa-cores")
					continue NEXTNODE
				}

				// set the cores and bandwidth consumed by the task
				taskResources.Cpu.ReservedCores = cores
				taskResources.Cpu.CpuShares = int64(bandwidth)
			}

			// Store the task resource
			option.SetTaskResources(task, taskResources)

			// Accumulate the total resource requirement
			total.Tasks[task.Name] = taskResources
			total.TaskLifecycles[task.Name] = task.Lifecycle
		}

		// Store current set of running allocs before adding resources for the task group
		current := proposed

		// Add the resources we are trying to fit
		proposed = append(proposed, &structs.Allocation{AllocatedResources: total})

		// Check if these allocations fit, if they do not, simply skip this node
		fit, dim, util, _ := structs.AllocsFit(option.Node, proposed, netIdx, false)
		netIdx.Release()
		if !fit {
			// Skip the node if evictions are not enabled
			if !iter.evict {
				iter.ctx.Metrics().ExhaustedNode(option.Node, dim)
				continue
			}

			// If eviction is enabled and the node doesn't fit the alloc, check if
			// any allocs can be preempted

			// Initialize preemptor with candidate set
			preemptor.SetCandidates(current)

			preemptedAllocs := preemptor.PreemptForTaskGroup(total)
			allocsToPreempt = append(allocsToPreempt, preemptedAllocs...)

			// If we were unable to find preempted allocs to meet these requirements
			// mark as exhausted and continue
			if len(preemptedAllocs) == 0 {
				iter.ctx.Metrics().ExhaustedNode(option.Node, dim)
				continue
			}
		}
		if len(allocsToPreempt) > 0 {
			option.PreemptedAllocs = allocsToPreempt
		}

		// Score the fit normally otherwise
		fitness := iter.scoreFit(option.Node, util)
		normalizedFit := fitness / binPackingMaxFitScore
		option.Scores = append(option.Scores, normalizedFit)
		iter.ctx.Metrics().ScoreNode(option.Node, "binpack", normalizedFit)

		// Score the device affinity
		if totalDeviceAffinityWeight != 0 {
			sumMatchingAffinities /= totalDeviceAffinityWeight
			option.Scores = append(option.Scores, sumMatchingAffinities)
			iter.ctx.Metrics().ScoreNode(option.Node, "devices", sumMatchingAffinities)
		}

		return option
	}
}

func (iter *BinPackIterator) Reset() {
	iter.source.Reset()
}

// JobAntiAffinityIterator is used to apply an anti-affinity to allocating
// along side other allocations from this job. This is used to help distribute
// load across the cluster.
type JobAntiAffinityIterator struct {
	ctx          Context
	source       RankIterator
	jobID        string
	taskGroup    string
	desiredCount int
}

// NewJobAntiAffinityIterator is used to create a JobAntiAffinityIterator that
// applies the given penalty for co-placement with allocs from this job.
func NewJobAntiAffinityIterator(ctx Context, source RankIterator, jobID string) *JobAntiAffinityIterator {
	iter := &JobAntiAffinityIterator{
		ctx:    ctx,
		source: source,
		jobID:  jobID,
	}
	return iter
}

func (iter *JobAntiAffinityIterator) SetJob(job *structs.Job) {
	iter.jobID = job.ID
}

func (iter *JobAntiAffinityIterator) SetTaskGroup(tg *structs.TaskGroup) {
	iter.taskGroup = tg.Name
	iter.desiredCount = tg.Count
}

func (iter *JobAntiAffinityIterator) Next() *RankedNode {
	for {
		option := iter.source.Next()
		if option == nil {
			return nil
		}

		// Get the proposed allocations
		proposed, err := option.ProposedAllocs(iter.ctx)
		if err != nil {
			iter.ctx.Logger().Named("job_anti_affinity").Error("failed retrieving proposed allocations", "error", err)
			continue
		}

		// Determine the number of collisions
		collisions := 0
		for _, alloc := range proposed {
			if alloc.JobID == iter.jobID && alloc.TaskGroup == iter.taskGroup {
				collisions += 1
			}
		}

		// Calculate the penalty based on number of collisions
		// TODO(preetha): Figure out if batch jobs need a different scoring penalty where collisions matter less
		if collisions > 0 {
			scorePenalty := -1 * float64(collisions+1) / float64(iter.desiredCount)
			option.Scores = append(option.Scores, scorePenalty)
			iter.ctx.Metrics().ScoreNode(option.Node, "job-anti-affinity", scorePenalty)
		} else {
			iter.ctx.Metrics().ScoreNode(option.Node, "job-anti-affinity", 0)
		}
		return option
	}
}

func (iter *JobAntiAffinityIterator) Reset() {
	iter.source.Reset()
}

// NodeReschedulingPenaltyIterator is used to apply a penalty to
// a node that had a previous failed allocation for the same job.
// This is used when attempting to reschedule a failed alloc
type NodeReschedulingPenaltyIterator struct {
	ctx          Context
	source       RankIterator
	penaltyNodes map[string]struct{}
}

// NewNodeReschedulingPenaltyIterator is used to create a NodeReschedulingPenaltyIterator that
// applies the given scoring penalty for placement onto nodes in penaltyNodes
func NewNodeReschedulingPenaltyIterator(ctx Context, source RankIterator) *NodeReschedulingPenaltyIterator {
	iter := &NodeReschedulingPenaltyIterator{
		ctx:    ctx,
		source: source,
	}
	return iter
}

func (iter *NodeReschedulingPenaltyIterator) SetPenaltyNodes(penaltyNodes map[string]struct{}) {
	iter.penaltyNodes = penaltyNodes
}

func (iter *NodeReschedulingPenaltyIterator) Next() *RankedNode {
	option := iter.source.Next()
	if option == nil {
		return nil
	}

	_, ok := iter.penaltyNodes[option.Node.ID]
	if ok {
		option.Scores = append(option.Scores, -1)
		iter.ctx.Metrics().ScoreNode(option.Node, "node-reschedule-penalty", -1)
	} else {
		iter.ctx.Metrics().ScoreNode(option.Node, "node-reschedule-penalty", 0)
	}

	return option
}

func (iter *NodeReschedulingPenaltyIterator) Reset() {
	iter.penaltyNodes = make(map[string]struct{})
	iter.source.Reset()
}

// NodeAffinityIterator is used to resolve any affinity rules in the job or task group,
// and apply a weighted score to nodes if they match.
type NodeAffinityIterator struct {
	ctx           Context
	source        RankIterator
	jobAffinities []*structs.Affinity
	affinities    []*structs.Affinity
}

// NewNodeAffinityIterator is used to create a NodeAffinityIterator that
// applies a weighted score according to whether nodes match any
// affinities in the job or task group.
func NewNodeAffinityIterator(ctx Context, source RankIterator) *NodeAffinityIterator {
	return &NodeAffinityIterator{
		ctx:    ctx,
		source: source,
	}
}

func (iter *NodeAffinityIterator) SetJob(job *structs.Job) {
	iter.jobAffinities = job.Affinities
}

func (iter *NodeAffinityIterator) SetTaskGroup(tg *structs.TaskGroup) {
	// Merge job affinities
	if iter.jobAffinities != nil {
		iter.affinities = append(iter.affinities, iter.jobAffinities...)
	}

	// Merge task group affinities and task affinities
	if tg.Affinities != nil {
		iter.affinities = append(iter.affinities, tg.Affinities...)
	}
	for _, task := range tg.Tasks {
		if task.Affinities != nil {
			iter.affinities = append(iter.affinities, task.Affinities...)
		}
	}
}

func (iter *NodeAffinityIterator) Reset() {
	iter.source.Reset()
	// This method is called between each task group, so only reset the merged list
	iter.affinities = nil
}

func (iter *NodeAffinityIterator) hasAffinities() bool {
	return len(iter.affinities) > 0
}

func (iter *NodeAffinityIterator) Next() *RankedNode {
	option := iter.source.Next()
	if option == nil {
		return nil
	}
	if !iter.hasAffinities() {
		iter.ctx.Metrics().ScoreNode(option.Node, "node-affinity", 0)
		return option
	}
	// TODO(preetha): we should calculate normalized weights once and reuse it here
	sumWeight := 0.0
	for _, affinity := range iter.affinities {
		sumWeight += math.Abs(float64(affinity.Weight))
	}

	totalAffinityScore := 0.0
	for _, affinity := range iter.affinities {
		if matchesAffinity(iter.ctx, affinity, option.Node) {
			totalAffinityScore += float64(affinity.Weight)
		}
	}
	normScore := totalAffinityScore / sumWeight
	if totalAffinityScore != 0.0 {
		option.Scores = append(option.Scores, normScore)
		iter.ctx.Metrics().ScoreNode(option.Node, "node-affinity", normScore)
	}
	return option
}

func matchesAffinity(ctx Context, affinity *structs.Affinity, option *structs.Node) bool {
	//TODO(preetha): Add a step here that filters based on computed node class for potential speedup
	// Resolve the targets
	lVal, lOk := resolveTarget(affinity.LTarget, option)
	rVal, rOk := resolveTarget(affinity.RTarget, option)

	// Check if satisfied
	return checkAffinity(ctx, affinity.Operand, lVal, rVal, lOk, rOk)
}

// ScoreNormalizationIterator is used to combine scores from various prior
// iterators and combine them into one final score. The current implementation
// averages the scores together.
type ScoreNormalizationIterator struct {
	ctx    Context
	source RankIterator
}

// NewScoreNormalizationIterator is used to create a ScoreNormalizationIterator that
// averages scores from various iterators into a final score.
func NewScoreNormalizationIterator(ctx Context, source RankIterator) *ScoreNormalizationIterator {
	return &ScoreNormalizationIterator{
		ctx:    ctx,
		source: source}
}

func (iter *ScoreNormalizationIterator) Reset() {
	iter.source.Reset()
}

func (iter *ScoreNormalizationIterator) Next() *RankedNode {
	option := iter.source.Next()
	if option == nil || len(option.Scores) == 0 {
		return option
	}
	numScorers := len(option.Scores)
	sum := 0.0
	for _, score := range option.Scores {
		sum += score
	}
	option.FinalScore = sum / float64(numScorers)
	//TODO(preetha): Turn map in allocmetrics into a heap of topK scores
	iter.ctx.Metrics().ScoreNode(option.Node, "normalized-score", option.FinalScore)
	return option
}

// PreemptionScoringIterator is used to score nodes according to the
// combination of preemptible allocations in them
type PreemptionScoringIterator struct {
	ctx    Context
	source RankIterator
}

// NewPreemptionScoringIterator is used to create a score based on net
// aggregate priority of preempted allocations.
func NewPreemptionScoringIterator(ctx Context, source RankIterator) RankIterator {
	return &PreemptionScoringIterator{
		ctx:    ctx,
		source: source,
	}
}

func (iter *PreemptionScoringIterator) Reset() {
	iter.source.Reset()
}

func (iter *PreemptionScoringIterator) Next() *RankedNode {
	option := iter.source.Next()
	if option == nil || option.PreemptedAllocs == nil {
		return option
	}

	netPriority := netPriority(option.PreemptedAllocs)
	// preemption score is inversely proportional to netPriority
	preemptionScore := preemptionScore(netPriority)
	option.Scores = append(option.Scores, preemptionScore)
	iter.ctx.Metrics().ScoreNode(option.Node, "preemption", preemptionScore)

	return option
}

// netPriority is a scoring heuristic that represents a combination of two factors.
// First factor is the max priority in the set of allocations, with
// an additional factor that takes into account the individual priorities of allocations
func netPriority(allocs []*structs.Allocation) float64 {
	sumPriority := 0
	max := 0.0
	for _, alloc := range allocs {
		if float64(alloc.Job.Priority) > max {
			max = float64(alloc.Job.Priority)
		}
		sumPriority += alloc.Job.Priority
	}
	// We use the maximum priority across all allocations
	// with an additional penalty that increases proportional to the
	// ratio of the sum by max
	// This ensures that we penalize nodes that have a low max but a high
	// number of preemptible allocations
	ret := max + (float64(sumPriority) / max)
	return ret
}

// preemptionScore is calculated using a logistic function
// see https://www.desmos.com/calculator/alaeiuaiey for a visual representation of the curve.
// Lower values of netPriority get a score closer to 1 and the inflection point is around 2048
// The score is modelled to be between 0 and 1 because its combined with other
// scoring factors like bin packing
func preemptionScore(netPriority float64) float64 {
	// These values were chosen such that a net priority of 2048 would get a preemption score of 0.5
	// rate is the decay parameter of the logistic function used in scoring preemption options
	const rate = 0.0048

	// origin controls the inflection point of the logistic function used in scoring preemption options
	const origin = 2048.0

	// This function manifests as an s curve that asympotically moves towards zero for large values of netPriority
	return 1.0 / (1 + math.Exp(rate*(netPriority-origin)))
}
