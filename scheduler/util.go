// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"encoding/binary"
	"fmt"
	"maps"
	"math/rand"
	"slices"

	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
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

// readyNodesInDCsAndPool returns all the ready nodes in the given datacenters
// and pool, and a mapping of each data center to the count of ready nodes.
func readyNodesInDCsAndPool(state State, dcs []string, pool string) ([]*structs.Node, map[string]struct{}, map[string]int, error) {
	// Index the DCs
	dcMap := make(map[string]int)

	// Scan the nodes
	ws := memdb.NewWatchSet()
	var out []*structs.Node
	notReady := map[string]struct{}{}

	var iter memdb.ResultIterator
	var err error

	if pool == structs.NodePoolAll || pool == "" {
		iter, err = state.Nodes(ws)
	} else {
		iter, err = state.NodesByNodePool(ws, pool)
	}
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
		if node.IsInAnyDC(dcs) {
			out = append(out, node)
			dcMap[node.Datacenter]++
		}
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

// comparison records the _first_ detected difference between two groups during
// a comparison in tasksUpdated
//
// This is useful to provide context when debugging the result of tasksUpdated.
type comparison struct {
	modified bool
	label    string
	before   any
	after    any
}

func difference(label string, before, after any) comparison {
	// push string formatting into String(), so that we never call it in the
	// hot path unless someone adds a log line to debug with this result
	return comparison{
		modified: true,
		label:    label,
		before:   before,
		after:    after,
	}
}

func (c comparison) String() string {
	return fmt.Sprintf("%s changed; before: %#v, after: %#v", c.label, c.before, c.after)
}

// same indicates no destructive difference between two task groups
var same = comparison{modified: false}

// tasksUpdated creates a comparison between task groups to see if the tasks, their
// drivers, environment variables or config have been modified.
func tasksUpdated(jobA, jobB *structs.Job, taskGroup string) comparison {
	a := jobA.LookupTaskGroup(taskGroup)
	b := jobB.LookupTaskGroup(taskGroup)

	// If the number of tasks do not match, clearly there is an update
	if lenA, lenB := len(a.Tasks), len(b.Tasks); lenA != lenB {
		return difference("number of tasks", lenA, lenB)
	}

	// Check ephemeral disk
	if !a.EphemeralDisk.Equal(b.EphemeralDisk) {
		return difference("ephemeral disk", a.EphemeralDisk, b.EphemeralDisk)
	}

	// Check that the network resources haven't changed
	if c := networkUpdated(a.Networks, b.Networks); c.modified {
		return c
	}

	// Check Affinities
	if c := affinitiesUpdated(jobA, jobB, taskGroup); c.modified {
		return c
	}

	// Check Spreads
	if c := spreadsUpdated(jobA, jobB, taskGroup); c.modified {
		return c
	}

	// Check consul updated
	if c := consulUpdated(a.Consul, b.Consul); c.modified {
		return c
	}

	// Check connect service(s) updated
	if c := connectServiceUpdated(a.Services, b.Services); c.modified {
		return c
	}

	// Check if volumes are updated (no task driver can support
	// altering mounts in-place)
	if !maps.EqualFunc(a.Volumes, b.Volumes, func(a, b *structs.VolumeRequest) bool { return a.Equal(b) }) {
		return difference("volume request", a.Volumes, b.Volumes)
	}

	// Check if restart.render_templates is updated
	// this requires a destructive update for template hook to receive the new config
	if c := renderTemplatesUpdated(a.RestartPolicy, b.RestartPolicy,
		"group restart render_templates"); c.modified {
		return c
	}

	// Check each task
	for _, at := range a.Tasks {
		bt := b.LookupTask(at.Name)
		if bt == nil {
			return difference("task deleted", at.Name, "(nil)")
		}
		if at.Driver != bt.Driver {
			return difference("task driver", at.Driver, bt.Driver)
		}
		if at.User != bt.User {
			return difference("task user", at.User, bt.User)
		}
		if !helper.OpaqueMapsEqual(at.Config, bt.Config) {
			return difference("task config", at.Config, bt.Config)
		}
		if !maps.Equal(at.Env, bt.Env) {
			return difference("task env", at.Env, bt.Env)
		}
		if !slices.EqualFunc(at.Artifacts, bt.Artifacts, func(a, b *structs.TaskArtifact) bool { return a.Equal(b) }) {
			return difference("task artifacts", at.Artifacts, bt.Artifacts)
		}
		if !at.Vault.Equal(bt.Vault) {
			return difference("task vault", at.Vault, bt.Vault)
		}
		if c := consulUpdated(at.Consul, bt.Consul); c.modified {
			return c
		}
		if !slices.EqualFunc(at.Templates, bt.Templates, func(a, b *structs.Template) bool { return a.Equal(b) }) {
			return difference("task templates", at.Templates, bt.Templates)
		}
		if !at.CSIPluginConfig.Equal(bt.CSIPluginConfig) {
			return difference("task csi config", at.CSIPluginConfig, bt.CSIPluginConfig)
		}
		if !slices.EqualFunc(at.VolumeMounts, bt.VolumeMounts, func(a, b *structs.VolumeMount) bool { return a.Equal(b) }) {
			return difference("task volume mount", at.VolumeMounts, bt.VolumeMounts)
		}

		// Check the metadata
		metaA := jobA.CombinedTaskMeta(taskGroup, at.Name)
		metaB := jobB.CombinedTaskMeta(taskGroup, bt.Name)
		if !maps.Equal(metaA, metaB) {
			return difference("task meta", metaA, metaB)
		}

		// Inspect the network to see if the dynamic ports are different
		if c := networkUpdated(at.Resources.Networks, bt.Resources.Networks); c.modified {
			return c
		}

		if c := nonNetworkResourcesUpdated(at.Resources, bt.Resources); c.modified {
			return c
		}

		// Inspect Identities being exposed
		if !at.Identity.Equal(bt.Identity) {
			return difference("task identity", at.Identity, bt.Identity)
		}

		if !slices.EqualFunc(at.Identities, bt.Identities, func(a, b *structs.WorkloadIdentity) bool { return a.Equal(b) }) {
			return difference("task identity", at.Identities, bt.Identities)
		}

		// Most LogConfig updates are in-place but if we change Disabled we need
		// to recreate the task to stop/start log collection and change the
		// stdout/stderr of the task
		if at.LogConfig.Disabled != bt.LogConfig.Disabled {
			return difference("task log disabled", at.LogConfig.Disabled, bt.LogConfig.Disabled)
		}

		// Check volume mount updates
		if c := volumeMountsUpdated(at.VolumeMounts, bt.VolumeMounts); c.modified {
			return c
		}

		// Check if restart.render_templates is updated
		if c := renderTemplatesUpdated(at.RestartPolicy, bt.RestartPolicy,
			"task restart render_templates"); c.modified {
			return c
		}

	}

	// none of the fields that trigger a destructive update were modified,
	// indicating this group can be updated in-place or ignored
	return same
}

func nonNetworkResourcesUpdated(a, b *structs.Resources) comparison {
	// Inspect the non-network resources
	switch {
	case a.CPU != b.CPU:
		return difference("task cpu", a.CPU, b.CPU)
	case a.Cores != b.Cores:
		return difference("task cores", a.Cores, b.Cores)
	case a.MemoryMB != b.MemoryMB:
		return difference("task memory", a.MemoryMB, b.MemoryMB)
	case a.MemoryMaxMB != b.MemoryMaxMB:
		return difference("task memory max", a.MemoryMaxMB, b.MemoryMaxMB)
	case !a.Devices.Equal(&b.Devices):
		return difference("task devices", a.Devices, b.Devices)
	case !a.NUMA.Equal(b.NUMA):
		return difference("numa", a.NUMA, b.NUMA)
	case a.SecretsMB != b.SecretsMB:
		return difference("task secrets", a.SecretsMB, b.SecretsMB)
	}
	return same
}

// consulUpdated returns true if the Consul namespace or cluster in the task
// group has been changed.
//
// This is treated as a destructive update unlike ordinary Consul service
// configuration because Namespaces and Cluster directly impact networking
// validity among Consul intentions.  Forcing the task through a reschedule is a
// sure way of breaking no-longer valid network connections.
func consulUpdated(consulA, consulB *structs.Consul) comparison {
	// job.ConsulNamespace is pushed down to the TGs, just check those
	if a, b := consulA.GetNamespace(), consulB.GetNamespace(); a != b {
		return difference("consul namespace", a, b)
	}

	// if either are nil, we can treat this as a non-destructive update
	if consulA != nil && consulB != nil {
		if a, b := consulA.Cluster, consulB.Cluster; a != b {
			return difference("consul cluster", a, b)
		}

		if a, b := consulA.Partition, consulB.Partition; a != b {
			return difference("consul partition", a, b)
		}
	}

	return same
}

// connectServiceUpdated returns true if any services with a connect block have
// been changed in such a way that requires a destructive update.
//
// Ordinary services can be updated in-place by updating the service definition
// in Consul. Connect service changes mostly require destroying the task.
func connectServiceUpdated(servicesA, servicesB []*structs.Service) comparison {
	for _, serviceA := range servicesA {
		if serviceA.Connect != nil {
			for _, serviceB := range servicesB {
				if serviceA.Name == serviceB.Name {
					if c := connectUpdated(serviceA.Connect, serviceB.Connect); c.modified {
						return c
					}
					// Part of the Connect plumbing is derived from port label,
					// if that changes we need to destroy the task.
					if serviceA.PortLabel != serviceB.PortLabel {
						return difference("connect service port label", serviceA.PortLabel, serviceB.PortLabel)
					}

					if serviceA.Cluster != serviceB.Cluster {
						return difference("connect service cluster", serviceA.Cluster, serviceB.Cluster)
					}

					break
				}
			}
		}
	}
	return same
}

func volumeMountsUpdated(a, b []*structs.VolumeMount) comparison {
	setA := set.HashSetFrom(a)
	setB := set.HashSetFrom(b)

	if setA.Equal(setB) {
		return same
	}

	return difference("volume mounts", a, b)
}

// volumeMountUpdated returns true if the definition of the volume mount
// has been updated in a manner that will requires the task to be recreated.
func volumeMountUpdated(mountA, mountB *structs.VolumeMount) comparison {
	if mountA != nil && mountB == nil {
		difference("volume mount removed", mountA, mountB)
	}

	if mountA != nil && mountB != nil &&
		mountA.SELinuxLabel != mountB.SELinuxLabel {
		return difference("volume mount selinux label", mountA.SELinuxLabel, mountB.SELinuxLabel)
	}

	return same
}

// connectUpdated returns true if the connect block has been updated in a manner
// that will require a destructive update.
//
// Fields that can be updated through consul-sync do not need a destructive
// update.
func connectUpdated(connectA, connectB *structs.ConsulConnect) comparison {
	if connectA == nil && connectB == nil {
		return same
	}

	if connectA == nil && connectB != nil {
		return difference("connect added", connectA, connectB)
	}

	if connectA != nil && connectB == nil {
		return difference("connect removed", connectA, connectB)
	}

	if connectA.Native != connectB.Native {
		return difference("connect native", connectA.Native, connectB.Native)
	}

	if !connectA.Gateway.Equal(connectB.Gateway) {
		return difference("connect gateway", connectA.Gateway, connectB.Gateway)
	}

	if !connectA.SidecarTask.Equal(connectB.SidecarTask) {
		return difference("connect sidecar task", connectA.SidecarTask, connectB.SidecarTask)
	}

	// not everything in sidecar_service needs task destruction
	if c := connectSidecarServiceUpdated(connectA.SidecarService, connectB.SidecarService); c.modified {
		return c
	}

	return same
}

func connectSidecarServiceUpdated(ssA, ssB *structs.ConsulSidecarService) comparison {
	if ssA == nil && ssB == nil {
		return same
	}

	if ssA == nil && ssB != nil {
		return difference("connect service add", ssA, ssB)
	}

	if ssA != nil && ssB == nil {
		return difference("connect service delete", ssA, ssB)
	}

	if ssA.Port != ssB.Port {
		return difference("connect port", ssA.Port, ssB.Port)
	}

	// sidecar_service.tags (handled in-place via registration)

	// sidecar_service.proxy (handled in-place via registration + xDS)

	return same
}

func networkUpdated(netA, netB []*structs.NetworkResource) comparison {
	if lenNetA, lenNetB := len(netA), len(netB); lenNetA != lenNetB {
		return difference("network lengths", lenNetA, lenNetB)
	}
	for idx := range netA {
		an := netA[idx]
		bn := netB[idx]

		if an.Mode != bn.Mode {
			return difference("network mode", an.Mode, bn.Mode)
		}

		if an.MBits != bn.MBits {
			return difference("network mbits", an.MBits, bn.MBits)
		}

		if an.Hostname != bn.Hostname {
			return difference("network hostname", an.Hostname, bn.Hostname)
		}

		if !an.DNS.Equal(bn.DNS) {
			return difference("network dns", an.DNS, bn.DNS)
		}

		if !an.CNI.Equal(bn.CNI) {
			return difference("network cni", an.CNI, bn.CNI)
		}

		aPorts, bPorts := networkPortMap(an), networkPortMap(bn)
		if !aPorts.Equal(bPorts) {
			return difference("network port map", aPorts, bPorts)
		}
	}
	return same
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

func affinitiesUpdated(jobA, jobB *structs.Job, taskGroup string) comparison {
	var affinitiesA structs.Affinities
	var affinitiesB structs.Affinities

	// accumulate job affinities

	affinitiesA = append(affinitiesA, jobA.Affinities...)
	affinitiesB = append(affinitiesB, jobB.Affinities...)

	tgA := jobA.LookupTaskGroup(taskGroup)
	tgB := jobB.LookupTaskGroup(taskGroup)

	// append group level affinities

	affinitiesA = append(affinitiesA, tgA.Affinities...)
	affinitiesB = append(affinitiesB, tgB.Affinities...)

	// append task level affinities for A

	for _, task := range tgA.Tasks {
		affinitiesA = append(affinitiesA, task.Affinities...)
	}

	// append task level affinities for B
	for _, task := range tgB.Tasks {
		affinitiesB = append(affinitiesB, task.Affinities...)
	}

	// finally check if all the affinities from both jobs match
	if !affinitiesA.Equal(&affinitiesB) {
		return difference("affinities", affinitiesA, affinitiesB)
	}

	return same
}

func spreadsUpdated(jobA, jobB *structs.Job, taskGroup string) comparison {
	var spreadsA []*structs.Spread
	var spreadsB []*structs.Spread

	// accumulate job spreads

	spreadsA = append(spreadsA, jobA.Spreads...)
	spreadsB = append(spreadsB, jobB.Spreads...)

	tgA := jobA.LookupTaskGroup(taskGroup)
	tgB := jobB.LookupTaskGroup(taskGroup)

	// append group spreads
	spreadsA = append(spreadsA, tgA.Spreads...)
	spreadsB = append(spreadsB, tgB.Spreads...)

	if !slices.EqualFunc(spreadsA, spreadsB, func(a, b *structs.Spread) bool {
		return a.Equal(b)
	}) {
		return difference("spreads", spreadsA, spreadsB)
	}

	return same
}

// renderTemplatesUpdated returns the difference in the RestartPolicy's
// render_templates field, if set
func renderTemplatesUpdated(a, b *structs.RestartPolicy, msg string) comparison {

	noRenderA := a == nil || !a.RenderTemplates
	noRenderB := b == nil || !b.RenderTemplates

	if noRenderA && !noRenderB {
		return difference(msg, false, true)
	}
	if !noRenderA && noRenderB {
		return difference(msg, true, false)
	}

	return same // both nil, or one nil and the other false
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
		if c := tasksUpdated(job, existing, update.TaskGroup.Name); c.modified {
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
		if !node.IsInAnyDC(job.Datacenters) {
			continue
		}
		// The alloc is on a node that's now in an ineligible node pool
		if !node.IsInPool(job.NodePool) {
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
		if c := tasksUpdated(newJob, existing.Job, newTG.Name); c.modified {
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
		if !node.IsInAnyDC(newJob.Datacenters) {
			return false, true, nil
		}
		if !node.IsInPool(newJob.NodePool) {
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
