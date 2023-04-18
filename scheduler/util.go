package scheduler

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"reflect"

	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
	"golang.org/x/exp/slices"
)

// allocTuple is a tuple of the allocation name and potential alloc ID
type allocTuple struct {
	Name      string
	TaskGroup *structs.TaskGroup
	Alloc     *structs.Allocation
}

// diffResult is used to return the sets that result from the diff
type diffResult struct {
	place, update, migrate, stop, ignore, lost, disconnecting, reconnecting []allocTuple
}

func (d *diffResult) GoString() string {
	return fmt.Sprintf("allocs: (place %d) (update %d) (migrate %d) (stop %d) (ignore %d) (lost %d) (disconnecting %d) (reconnecting %d)",
		len(d.place), len(d.update), len(d.migrate), len(d.stop), len(d.ignore), len(d.lost), len(d.disconnecting), len(d.reconnecting))
}

func (d *diffResult) Append(other *diffResult) {
	d.place = append(d.place, other.place...)
	d.update = append(d.update, other.update...)
	d.migrate = append(d.migrate, other.migrate...)
	d.stop = append(d.stop, other.stop...)
	d.ignore = append(d.ignore, other.ignore...)
	d.lost = append(d.lost, other.lost...)
	d.disconnecting = append(d.disconnecting, other.disconnecting...)
	d.reconnecting = append(d.reconnecting, other.reconnecting...)
}

// readyNodesInDCs returns all the ready nodes in the given datacenters and a
// mapping of each data center to the count of ready nodes.
func readyNodesInDCs(state State, dcs []string) ([]*structs.Node, map[string]struct{}, map[string]int, error) {
	// Index the DCs
	dcMap := make(map[string]int, len(dcs))
	for _, dc := range dcs {
		dcMap[dc] = 0
	}

	// Scan the nodes
	ws := memdb.NewWatchSet()
	var out []*structs.Node
	notReady := map[string]struct{}{}
	iter, err := state.Nodes(ws)
	if err != nil {
		return nil, nil, nil, err
	}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		// Filter on datacenter and status
		node := raw.(*structs.Node)
		if !node.Ready() {
			notReady[node.ID] = struct{}{}
			continue
		}
		if _, ok := dcMap[node.Datacenter]; !ok {
			continue
		}
		out = append(out, node)
		dcMap[node.Datacenter]++
	}
	return out, notReady, dcMap, nil
}

// retryMax is used to retry a callback until it returns success or
// a maximum number of attempts is reached. An optional reset function may be
// passed which is called after each failed iteration. If the reset function is
// set and returns true, the number of attempts is reset back to max.
func retryMax(max int, cb func() (bool, error), reset func() bool) error {
	attempts := 0
	for attempts < max {
		done, err := cb()
		if err != nil {
			return err
		}
		if done {
			return nil
		}

		// Check if we should reset the number attempts
		if reset != nil && reset() {
			attempts = 0
		} else {
			attempts++
		}
	}
	return &SetStatusError{
		Err:        fmt.Errorf("maximum attempts reached (%d)", max),
		EvalStatus: structs.EvalStatusFailed,
	}
}

// progressMade checks to see if the plan result made allocations or updates.
// If the result is nil, false is returned.
func progressMade(result *structs.PlanResult) bool {
	return result != nil && (len(result.NodeUpdate) != 0 ||
		len(result.NodeAllocation) != 0 || result.Deployment != nil ||
		len(result.DeploymentUpdates) != 0)
}

// taintedNodes is used to scan the allocations and then check if the
// underlying nodes are tainted, and should force a migration of the allocation,
// or if the underlying nodes are disconnected, and should be used to calculate
// the reconnect timeout of its allocations. All the nodes returned in the map are tainted.
func taintedNodes(state State, allocs []*structs.Allocation) (map[string]*structs.Node, error) {
	out := make(map[string]*structs.Node)
	for _, alloc := range allocs {
		if _, ok := out[alloc.NodeID]; ok {
			continue
		}

		ws := memdb.NewWatchSet()
		node, err := state.NodeByID(ws, alloc.NodeID)
		if err != nil {
			return nil, err
		}

		// If the node does not exist, we should migrate
		if node == nil {
			out[alloc.NodeID] = nil
			continue
		}
		if structs.ShouldDrainNode(node.Status) || node.DrainStrategy != nil {
			out[alloc.NodeID] = node
		}

		// Disconnected nodes are included in the tainted set so that their
		// MaxClientDisconnect configuration can be included in the
		// timeout calculation.
		if node.Status == structs.NodeStatusDisconnected {
			out[alloc.NodeID] = node
		}
	}

	return out, nil
}

// shuffleNodes randomizes the slice order with the Fisher-Yates
// algorithm. We seed the random source with the eval ID (which is
// random) to aid in postmortem debugging of specific evaluations and
// state snapshots.
func shuffleNodes(plan *structs.Plan, index uint64, nodes []*structs.Node) {

	// use the last 4 bytes because those are the random bits
	// if we have sortable IDs
	buf := []byte(plan.EvalID)
	seed := binary.BigEndian.Uint64(buf[len(buf)-8:])

	// for retried plans the index is the plan result's RefreshIndex
	// so that we don't retry with the exact same shuffle
	seed ^= index
	r := rand.New(rand.NewSource(int64(seed >> 2)))

	n := len(nodes)
	for i := n - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}
}

// tasksUpdated does a diff between task groups to see if the
// tasks, their drivers, environment variables or config have updated. The
// inputs are the task group name to diff and two jobs to diff.
// taskUpdated and functions called within assume that the given
// taskGroup has already been checked to not be nil
func tasksUpdated(jobA, jobB *structs.Job, taskGroup string) bool {
	a := jobA.LookupTaskGroup(taskGroup)
	b := jobB.LookupTaskGroup(taskGroup)

	// If the number of tasks do not match, clearly there is an update
	if len(a.Tasks) != len(b.Tasks) {
		return true
	}

	// Check ephemeral disk
	if !reflect.DeepEqual(a.EphemeralDisk, b.EphemeralDisk) {
		return true
	}

	// Check that the network resources haven't changed
	if networkUpdated(a.Networks, b.Networks) {
		return true
	}

	// Check Affinities
	if affinitiesUpdated(jobA, jobB, taskGroup) {
		return true
	}

	// Check Spreads
	if spreadsUpdated(jobA, jobB, taskGroup) {
		return true
	}

	// Check consul namespace updated
	if consulNamespaceUpdated(a, b) {
		return true
	}

	// Check connect service(s) updated
	if connectServiceUpdated(a.Services, b.Services) {
		return true
	}

	// Check if volumes are updated (no task driver can support
	// altering mounts in-place)
	if !reflect.DeepEqual(a.Volumes, b.Volumes) {
		return true
	}

	// Check each task
	for _, at := range a.Tasks {
		bt := b.LookupTask(at.Name)
		if bt == nil {
			return true
		}
		if at.Driver != bt.Driver {
			return true
		}
		if at.User != bt.User {
			return true
		}
		if !reflect.DeepEqual(at.Config, bt.Config) {
			return true
		}
		if !reflect.DeepEqual(at.Env, bt.Env) {
			return true
		}
		if !reflect.DeepEqual(at.Artifacts, bt.Artifacts) {
			return true
		}
		if !reflect.DeepEqual(at.Vault, bt.Vault) {
			return true
		}
		if !reflect.DeepEqual(at.Templates, bt.Templates) {
			return true
		}
		if !reflect.DeepEqual(at.CSIPluginConfig, bt.CSIPluginConfig) {
			return true
		}
		if !reflect.DeepEqual(at.VolumeMounts, bt.VolumeMounts) {
			return true
		}

		// Check the metadata
		if !reflect.DeepEqual(
			jobA.CombinedTaskMeta(taskGroup, at.Name),
			jobB.CombinedTaskMeta(taskGroup, bt.Name)) {
			return true
		}

		// Inspect the network to see if the dynamic ports are different
		if networkUpdated(at.Resources.Networks, bt.Resources.Networks) {
			return true
		}

		// Inspect the non-network resources
		if ar, br := at.Resources, bt.Resources; ar.CPU != br.CPU {
			return true
		} else if ar.Cores != br.Cores {
			return true
		} else if ar.MemoryMB != br.MemoryMB {
			return true
		} else if ar.MemoryMaxMB != br.MemoryMaxMB {
			return true
		} else if !ar.Devices.Equals(&br.Devices) {
			return true
		}
	}
	return false
}

// consulNamespaceUpdated returns true if the Consul namespace in the task group
// has been changed.
//
// This is treated as a destructive update unlike ordinary Consul service configuration
// because Namespaces directly impact networking validity among Consul intentions.
// Forcing the task through a reschedule is a sure way of breaking no-longer valid
// network connections.
func consulNamespaceUpdated(tgA, tgB *structs.TaskGroup) bool {
	// job.ConsulNamespace is pushed down to the TGs, just check those
	return tgA.Consul.GetNamespace() != tgB.Consul.GetNamespace()
}

// connectServiceUpdated returns true if any services with a connect block have
// been changed in such a way that requires a destructive update.
//
// Ordinary services can be updated in-place by updating the service definition
// in Consul. Connect service changes mostly require destroying the task.
func connectServiceUpdated(servicesA, servicesB []*structs.Service) bool {
	for _, serviceA := range servicesA {
		if serviceA.Connect != nil {
			for _, serviceB := range servicesB {
				if serviceA.Name == serviceB.Name {
					if connectUpdated(serviceA.Connect, serviceB.Connect) {
						return true
					}
					// Part of the Connect plumbing is derived from port label,
					// if that changes we need to destroy the task.
					if serviceA.PortLabel != serviceB.PortLabel {
						return true
					}
					break
				}
			}
		}
	}
	return false
}

// connectUpdated returns true if the connect block has been updated in a manner
// that will require a destructive update.
//
// Fields that can be updated through consul-sync do not need a destructive
// update.
func connectUpdated(connectA, connectB *structs.ConsulConnect) bool {
	if connectA == nil || connectB == nil {
		return connectA != connectB
	}

	if connectA.Native != connectB.Native {
		return true
	}

	if !connectA.Gateway.Equals(connectB.Gateway) {
		return true
	}

	if !connectA.SidecarTask.Equals(connectB.SidecarTask) {
		return true
	}

	// not everything in sidecar_service needs task destruction
	if connectSidecarServiceUpdated(connectA.SidecarService, connectB.SidecarService) {
		return true
	}

	return false
}

func connectSidecarServiceUpdated(ssA, ssB *structs.ConsulSidecarService) bool {
	if ssA == nil || ssB == nil {
		return ssA != ssB
	}

	if ssA.Port != ssB.Port {
		return true
	}

	// sidecar_service.tags handled in-place (registration)

	// sidecar_service.proxy handled in-place (registration + xDS)

	return false
}

func networkUpdated(netA, netB []*structs.NetworkResource) bool {
	if len(netA) != len(netB) {
		return true
	}
	for idx := range netA {
		an := netA[idx]
		bn := netB[idx]

		if an.Mode != bn.Mode {
			return true
		}

		if an.MBits != bn.MBits {
			return true
		}

		if an.Hostname != bn.Hostname {
			return true
		}

		if !reflect.DeepEqual(an.DNS, bn.DNS) {
			return true
		}

		aPorts, bPorts := networkPortMap(an), networkPortMap(bn)
		if !reflect.DeepEqual(aPorts, bPorts) {
			return true
		}
	}
	return false
}

// networkPortMap takes a network resource and returns a AllocatedPorts.
// The value for dynamic ports is disregarded even if it is set. This
// makes this function suitable for comparing two network resources for changes.
func networkPortMap(n *structs.NetworkResource) structs.AllocatedPorts {
	var m structs.AllocatedPorts
	for _, p := range n.ReservedPorts {
		m = append(m, structs.AllocatedPortMapping{
			Label:  p.Label,
			Value:  p.Value,
			To:     p.To,
			HostIP: p.HostNetwork,
		})
	}
	for _, p := range n.DynamicPorts {
		m = append(m, structs.AllocatedPortMapping{
			Label:  p.Label,
			Value:  -1,
			To:     p.To,
			HostIP: p.HostNetwork,
		})
	}
	return m
}

func affinitiesUpdated(jobA, jobB *structs.Job, taskGroup string) bool {
	var aAffinities []*structs.Affinity
	var bAffinities []*structs.Affinity

	tgA := jobA.LookupTaskGroup(taskGroup)
	tgB := jobB.LookupTaskGroup(taskGroup)

	// Append jobA job and task group level affinities
	aAffinities = append(aAffinities, jobA.Affinities...)
	aAffinities = append(aAffinities, tgA.Affinities...)

	// Append jobB job and task group level affinities
	bAffinities = append(bAffinities, jobB.Affinities...)
	bAffinities = append(bAffinities, tgB.Affinities...)

	// append task affinities
	for _, task := range tgA.Tasks {
		aAffinities = append(aAffinities, task.Affinities...)
	}

	for _, task := range tgB.Tasks {
		bAffinities = append(bAffinities, task.Affinities...)
	}

	// Check for equality
	if len(aAffinities) != len(bAffinities) {
		return true
	}

	return !reflect.DeepEqual(aAffinities, bAffinities)
}

func spreadsUpdated(jobA, jobB *structs.Job, taskGroup string) bool {
	var aSpreads []*structs.Spread
	var bSpreads []*structs.Spread

	tgA := jobA.LookupTaskGroup(taskGroup)
	tgB := jobB.LookupTaskGroup(taskGroup)

	// append jobA and task group level spreads
	aSpreads = append(aSpreads, jobA.Spreads...)
	aSpreads = append(aSpreads, tgA.Spreads...)

	// append jobB and task group level spreads
	bSpreads = append(bSpreads, jobB.Spreads...)
	bSpreads = append(bSpreads, tgB.Spreads...)

	// Check for equality
	if len(aSpreads) != len(bSpreads) {
		return true
	}

	return !reflect.DeepEqual(aSpreads, bSpreads)
}

// setStatus is used to update the status of the evaluation
func setStatus(logger log.Logger, planner Planner,
	eval, nextEval, spawnedBlocked *structs.Evaluation,
	tgMetrics map[string]*structs.AllocMetric, status, desc string,
	queuedAllocs map[string]int, deploymentID string) error {

	logger.Debug("setting eval status", "status", status)
	newEval := eval.Copy()
	newEval.Status = status
	newEval.StatusDescription = desc
	newEval.DeploymentID = deploymentID
	newEval.FailedTGAllocs = tgMetrics
	if nextEval != nil {
		newEval.NextEval = nextEval.ID
	}
	if spawnedBlocked != nil {
		newEval.BlockedEval = spawnedBlocked.ID
	}
	if queuedAllocs != nil {
		newEval.QueuedAllocations = queuedAllocs
	}

	return planner.UpdateEval(newEval)
}

// inplaceUpdate attempts to update allocations in-place where possible. It
// returns the allocs that couldn't be done inplace and then those that could.
func inplaceUpdate(ctx Context, eval *structs.Evaluation, job *structs.Job,
	stack Stack, updates []allocTuple) (destructive, inplace []allocTuple) {

	// doInplace manipulates the updates map to make the current allocation
	// an inplace update.
	doInplace := func(cur, last, inplaceCount *int) {
		updates[*cur], updates[*last-1] = updates[*last-1], updates[*cur]
		*cur--
		*last--
		*inplaceCount++
	}

	ws := memdb.NewWatchSet()
	n := len(updates)
	inplaceCount := 0
	for i := 0; i < n; i++ {
		// Get the update
		update := updates[i]

		// Check if the task drivers or config has changed, requires
		// a rolling upgrade since that cannot be done in-place.
		existing := update.Alloc.Job
		if tasksUpdated(job, existing, update.TaskGroup.Name) {
			continue
		}

		// Terminal batch allocations are not filtered when they are completed
		// successfully. We should avoid adding the allocation to the plan in
		// the case that it is an in-place update to avoid both additional data
		// in the plan and work for the clients.
		if update.Alloc.TerminalStatus() {
			doInplace(&i, &n, &inplaceCount)
			continue
		}

		// Get the existing node
		node, err := ctx.State().NodeByID(ws, update.Alloc.NodeID)
		if err != nil {
			ctx.Logger().Error("failed to get node", "node_id", update.Alloc.NodeID, "error", err)
			continue
		}
		if node == nil {
			continue
		}

		// The alloc is on a node that's now in an ineligible DC
		if !slices.Contains(job.Datacenters, node.Datacenter) {
			continue
		}

		// Set the existing node as the base set
		stack.SetNodes([]*structs.Node{node})

		// Stage an eviction of the current allocation. This is done so that
		// the current allocation is discounted when checking for feasibility.
		// Otherwise we would be trying to fit the tasks current resources and
		// updated resources. After select is called we can remove the evict.
		ctx.Plan().AppendStoppedAlloc(update.Alloc, allocInPlace, "", "")

		// Attempt to match the task group
		option := stack.Select(update.TaskGroup,
			&SelectOptions{AllocName: update.Alloc.Name})

		// Pop the allocation
		ctx.Plan().PopUpdate(update.Alloc)

		// Skip if we could not do an in-place update
		if option == nil {
			continue
		}

		// Restore the network and device offers from the existing allocation.
		// We do not allow network resources (reserved/dynamic ports)
		// to be updated. This is guarded in taskUpdated, so we can
		// safely restore those here.
		for task, resources := range option.TaskResources {
			var networks structs.Networks
			var devices []*structs.AllocatedDeviceResource
			if update.Alloc.AllocatedResources != nil {
				if tr, ok := update.Alloc.AllocatedResources.Tasks[task]; ok {
					networks = tr.Networks
					devices = tr.Devices
				}
			} else if tr, ok := update.Alloc.TaskResources[task]; ok {
				networks = tr.Networks
			}

			// Add the networks and devices back
			resources.Networks = networks
			resources.Devices = devices
		}

		// Create a shallow copy
		newAlloc := new(structs.Allocation)
		*newAlloc = *update.Alloc

		// Update the allocation
		newAlloc.EvalID = eval.ID
		newAlloc.Job = nil       // Use the Job in the Plan
		newAlloc.Resources = nil // Computed in Plan Apply
		newAlloc.AllocatedResources = &structs.AllocatedResources{
			Tasks:          option.TaskResources,
			TaskLifecycles: option.TaskLifecycles,
			Shared: structs.AllocatedSharedResources{
				DiskMB:   int64(update.TaskGroup.EphemeralDisk.SizeMB),
				Ports:    update.Alloc.AllocatedResources.Shared.Ports,
				Networks: update.Alloc.AllocatedResources.Shared.Networks.Copy(),
			},
		}
		newAlloc.Metrics = ctx.Metrics()
		ctx.Plan().AppendAlloc(newAlloc, nil)

		// Remove this allocation from the slice
		doInplace(&i, &n, &inplaceCount)
	}

	if len(updates) > 0 {
		ctx.Logger().Debug("made in-place updates", "in-place", inplaceCount, "total_updates", len(updates))
	}
	return updates[:n], updates[n:]
}

// desiredUpdates takes the diffResult as well as the set of inplace and
// destructive updates and returns a map of task groups to their set of desired
// updates.
func desiredUpdates(diff *diffResult, inplaceUpdates,
	destructiveUpdates []allocTuple) map[string]*structs.DesiredUpdates {
	desiredTgs := make(map[string]*structs.DesiredUpdates)

	for _, tuple := range diff.place {
		name := tuple.TaskGroup.Name
		des, ok := desiredTgs[name]
		if !ok {
			des = &structs.DesiredUpdates{}
			desiredTgs[name] = des
		}

		des.Place++
	}

	for _, tuple := range diff.stop {
		name := tuple.Alloc.TaskGroup
		des, ok := desiredTgs[name]
		if !ok {
			des = &structs.DesiredUpdates{}
			desiredTgs[name] = des
		}

		des.Stop++
	}

	for _, tuple := range diff.ignore {
		name := tuple.TaskGroup.Name
		des, ok := desiredTgs[name]
		if !ok {
			des = &structs.DesiredUpdates{}
			desiredTgs[name] = des
		}

		des.Ignore++
	}

	for _, tuple := range diff.migrate {
		name := tuple.TaskGroup.Name
		des, ok := desiredTgs[name]
		if !ok {
			des = &structs.DesiredUpdates{}
			desiredTgs[name] = des
		}

		des.Migrate++
	}

	for _, tuple := range inplaceUpdates {
		name := tuple.TaskGroup.Name
		des, ok := desiredTgs[name]
		if !ok {
			des = &structs.DesiredUpdates{}
			desiredTgs[name] = des
		}

		des.InPlaceUpdate++
	}

	for _, tuple := range destructiveUpdates {
		name := tuple.TaskGroup.Name
		des, ok := desiredTgs[name]
		if !ok {
			des = &structs.DesiredUpdates{}
			desiredTgs[name] = des
		}

		des.DestructiveUpdate++
	}

	return desiredTgs
}

// adjustQueuedAllocations decrements the number of allocations pending per task
// group based on the number of allocations successfully placed
func adjustQueuedAllocations(logger log.Logger, result *structs.PlanResult, queuedAllocs map[string]int) {
	if result == nil {
		return
	}

	for _, allocations := range result.NodeAllocation {
		for _, allocation := range allocations {
			// Ensure that the allocation is newly created. We check that
			// the CreateIndex is equal to the ModifyIndex in order to check
			// that the allocation was just created. We do not check that
			// the CreateIndex is equal to the results AllocIndex because
			// the allocations we get back have gone through the planner's
			// optimistic snapshot and thus their indexes may not be
			// correct, but they will be consistent.
			if allocation.CreateIndex != allocation.ModifyIndex {
				continue
			}

			if _, ok := queuedAllocs[allocation.TaskGroup]; ok {
				queuedAllocs[allocation.TaskGroup]--
			} else {
				logger.Error("allocation placed but task group is not in list of unplaced allocations", "task_group", allocation.TaskGroup)
			}
		}
	}
}

// updateNonTerminalAllocsToLost updates the allocations which are in pending/running state
// on tainted node to lost, but only for allocs already DesiredStatus stop or evict
func updateNonTerminalAllocsToLost(plan *structs.Plan, tainted map[string]*structs.Node, allocs []*structs.Allocation) {
	for _, alloc := range allocs {
		node, ok := tainted[alloc.NodeID]
		if !ok {
			continue
		}

		// Only handle down nodes or nodes that are gone (node == nil)
		if node != nil && node.Status != structs.NodeStatusDown {
			continue
		}

		// If the alloc is already correctly marked lost, we're done
		if (alloc.DesiredStatus == structs.AllocDesiredStatusStop ||
			alloc.DesiredStatus == structs.AllocDesiredStatusEvict) &&
			(alloc.ClientStatus == structs.AllocClientStatusRunning ||
				alloc.ClientStatus == structs.AllocClientStatusPending) {
			plan.AppendStoppedAlloc(alloc, allocLost, structs.AllocClientStatusLost, "")
		}
	}
}

// genericAllocUpdateFn is a factory for the scheduler to create an allocUpdateType
// function to be passed into the reconciler. The factory takes objects that
// exist only in the scheduler context and returns a function that can be used
// by the reconciler to make decisions about how to update an allocation. The
// factory allows the reconciler to be unaware of how to determine the type of
// update necessary and can minimize the set of objects it is exposed to.
func genericAllocUpdateFn(ctx Context, stack Stack, evalID string) allocUpdateType {
	return func(existing *structs.Allocation, newJob *structs.Job, newTG *structs.TaskGroup) (ignore, destructive bool, updated *structs.Allocation) {
		// Same index, so nothing to do
		if existing.Job.JobModifyIndex == newJob.JobModifyIndex {
			return true, false, nil
		}

		// Check if the task drivers or config has changed, requires
		// a destructive upgrade since that cannot be done in-place.
		if tasksUpdated(newJob, existing.Job, newTG.Name) {
			return false, true, nil
		}

		// Terminal batch allocations are not filtered when they are completed
		// successfully. We should avoid adding the allocation to the plan in
		// the case that it is an in-place update to avoid both additional data
		// in the plan and work for the clients.
		if existing.TerminalStatus() {
			return true, false, nil
		}

		// Get the existing node
		ws := memdb.NewWatchSet()
		node, err := ctx.State().NodeByID(ws, existing.NodeID)
		if err != nil {
			ctx.Logger().Error("failed to get node", "node_id", existing.NodeID, "error", err)
			return true, false, nil
		}
		if node == nil {
			return false, true, nil
		}

		// The alloc is on a node that's now in an ineligible DC
		if !slices.Contains(newJob.Datacenters, node.Datacenter) {
			return false, true, nil
		}

		// Set the existing node as the base set
		stack.SetNodes([]*structs.Node{node})

		// Stage an eviction of the current allocation. This is done so that
		// the current allocation is discounted when checking for feasibility.
		// Otherwise we would be trying to fit the tasks current resources and
		// updated resources. After select is called we can remove the evict.
		ctx.Plan().AppendStoppedAlloc(existing, allocInPlace, "", "")

		// Attempt to match the task group
		option := stack.Select(newTG, &SelectOptions{AllocName: existing.Name})

		// Pop the allocation
		ctx.Plan().PopUpdate(existing)

		// Require destructive if we could not do an in-place update
		if option == nil {
			return false, true, nil
		}

		// Restore the network and device offers from the existing allocation.
		// We do not allow network resources (reserved/dynamic ports)
		// to be updated. This is guarded in taskUpdated, so we can
		// safely restore those here.
		for task, resources := range option.TaskResources {
			var networks structs.Networks
			var devices []*structs.AllocatedDeviceResource
			if existing.AllocatedResources != nil {
				if tr, ok := existing.AllocatedResources.Tasks[task]; ok {
					networks = tr.Networks
					devices = tr.Devices
				}
			} else if tr, ok := existing.TaskResources[task]; ok {
				networks = tr.Networks
			}

			// Add the networks back
			resources.Networks = networks
			resources.Devices = devices
		}

		// Create a shallow copy
		newAlloc := new(structs.Allocation)
		*newAlloc = *existing

		// Update the allocation
		newAlloc.EvalID = evalID
		newAlloc.Job = nil       // Use the Job in the Plan
		newAlloc.Resources = nil // Computed in Plan Apply
		newAlloc.AllocatedResources = &structs.AllocatedResources{
			Tasks:          option.TaskResources,
			TaskLifecycles: option.TaskLifecycles,
			Shared: structs.AllocatedSharedResources{
				DiskMB: int64(newTG.EphemeralDisk.SizeMB),
			},
		}

		// Since this is an inplace update, we should copy network and port
		// information from the original alloc. This is similar to how
		// we copy network info for task level networks above.
		//
		// existing.AllocatedResources is nil on Allocations created by
		// Nomad v0.8 or earlier.
		if existing.AllocatedResources != nil {
			newAlloc.AllocatedResources.Shared.Networks = existing.AllocatedResources.Shared.Networks
			newAlloc.AllocatedResources.Shared.Ports = existing.AllocatedResources.Shared.Ports
		}

		// Use metrics from existing alloc for in place upgrade
		// This is because if the inplace upgrade succeeded, any scoring metadata from
		// when it first went through the scheduler should still be preserved. Using scoring
		// metadata from the context would incorrectly replace it with metadata only from a single node that the
		// allocation is already on.
		newAlloc.Metrics = existing.Metrics.Copy()
		return false, false, newAlloc
	}
}
