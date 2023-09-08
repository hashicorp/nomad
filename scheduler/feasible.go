// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/helper/constraints/semver"
	"github.com/hashicorp/nomad/nomad/structs"
	psstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	"golang.org/x/exp/constraints"
)

const (
	FilterConstraintHostVolumes                    = "missing compatible host volumes"
	FilterConstraintCSIPluginTemplate              = "CSI plugin %s is missing from client %s"
	FilterConstraintCSIPluginUnhealthyTemplate     = "CSI plugin %s is unhealthy on client %s"
	FilterConstraintCSIPluginMaxVolumesTemplate    = "CSI plugin %s has the maximum number of volumes on client %s"
	FilterConstraintCSIVolumesLookupFailed         = "CSI volume lookup failed"
	FilterConstraintCSIVolumeNotFoundTemplate      = "missing CSI Volume %s"
	FilterConstraintCSIVolumeNoReadTemplate        = "CSI volume %s is unschedulable or has exhausted its available reader claims"
	FilterConstraintCSIVolumeNoWriteTemplate       = "CSI volume %s is unschedulable or is read-only"
	FilterConstraintCSIVolumeInUseTemplate         = "CSI volume %s has exhausted its available writer claims"
	FilterConstraintCSIVolumeGCdAllocationTemplate = "CSI volume %s has exhausted its available writer claims and is claimed by a garbage collected allocation %s; waiting for claim to be released"
	FilterConstraintDrivers                        = "missing drivers"
	FilterConstraintDevices                        = "missing devices"
	FilterConstraintsCSIPluginTopology             = "did not meet topology requirement"
)

var (
	// predatesBridgeFingerprint returns true if the constraint matches a version
	// of nomad that predates the addition of the bridge network finger-printer,
	// which was added in Nomad v0.12
	predatesBridgeFingerprint = mustBridgeConstraint()
)

func mustBridgeConstraint() version.Constraints {
	versionC, err := version.NewConstraint("< 0.12")
	if err != nil {
		panic(err)
	}
	return versionC
}

// FeasibleIterator is used to iteratively yield nodes that
// match feasibility constraints. The iterators may manage
// some state for performance optimizations.
type FeasibleIterator interface {
	// Next yields a feasible node or nil if exhausted
	Next() *structs.Node

	// Reset is invoked when an allocation has been placed
	// to reset any stale state.
	Reset()
}

// ContextualIterator is an iterator that can have the job and task group set
// on it.
type ContextualIterator interface {
	SetJob(*structs.Job)
	SetTaskGroup(*structs.TaskGroup)
}

// FeasibilityChecker is used to check if a single node meets feasibility
// constraints.
type FeasibilityChecker interface {
	Feasible(*structs.Node) bool
}

// StaticIterator is a FeasibleIterator which returns nodes
// in a static order. This is used at the base of the iterator
// chain only for testing due to deterministic behavior.
type StaticIterator struct {
	ctx    Context
	nodes  []*structs.Node
	offset int
	seen   int
}

// NewStaticIterator constructs a random iterator from a list of nodes
func NewStaticIterator(ctx Context, nodes []*structs.Node) *StaticIterator {
	iter := &StaticIterator{
		ctx:   ctx,
		nodes: nodes,
	}
	return iter
}

func (iter *StaticIterator) Next() *structs.Node {
	// Check if exhausted
	n := len(iter.nodes)
	if iter.offset == n || iter.seen == n {
		if iter.seen != n { // seen has been Reset() to 0
			iter.offset = 0
		} else {
			return nil
		}
	}

	// Return the next offset, use this one
	offset := iter.offset
	iter.offset += 1
	iter.seen += 1
	iter.ctx.Metrics().EvaluateNode()
	return iter.nodes[offset]
}

func (iter *StaticIterator) Reset() {
	iter.seen = 0
}

func (iter *StaticIterator) SetNodes(nodes []*structs.Node) {
	iter.nodes = nodes
	iter.offset = 0
	iter.seen = 0
}

// NewRandomIterator constructs a static iterator from a list of nodes
// after applying the Fisher-Yates algorithm for a random shuffle. This
// is applied in-place
func NewRandomIterator(ctx Context, nodes []*structs.Node) *StaticIterator {
	// shuffle with the Fisher-Yates algorithm
	idx, _ := ctx.State().LatestIndex()
	shuffleNodes(ctx.Plan(), idx, nodes)

	// Create a static iterator
	return NewStaticIterator(ctx, nodes)
}

// HostVolumeChecker is a FeasibilityChecker which returns whether a node has
// the host volumes necessary to schedule a task group.
type HostVolumeChecker struct {
	ctx Context

	// volumes is a map[HostVolumeName][]RequestedVolume. The requested volumes are
	// a slice because a single task group may request the same volume multiple times.
	volumes map[string][]*structs.VolumeRequest
}

// NewHostVolumeChecker creates a HostVolumeChecker from a set of volumes
func NewHostVolumeChecker(ctx Context) *HostVolumeChecker {
	return &HostVolumeChecker{
		ctx: ctx,
	}
}

// SetVolumes takes the volumes required by a task group and updates the checker.
func (h *HostVolumeChecker) SetVolumes(allocName string, volumes map[string]*structs.VolumeRequest) {
	lookupMap := make(map[string][]*structs.VolumeRequest)
	// Convert the map from map[DesiredName]Request to map[Source][]Request to improve
	// lookup performance. Also filter non-host volumes.
	for _, req := range volumes {
		if req.Type != structs.VolumeTypeHost {
			continue
		}

		if req.PerAlloc {
			// provide a unique volume source per allocation
			copied := req.Copy()
			copied.Source = copied.Source + structs.AllocSuffix(allocName)
			lookupMap[copied.Source] = append(lookupMap[copied.Source], copied)
		} else {
			lookupMap[req.Source] = append(lookupMap[req.Source], req)
		}
	}
	h.volumes = lookupMap
}

func (h *HostVolumeChecker) Feasible(candidate *structs.Node) bool {
	if h.hasVolumes(candidate) {
		return true
	}

	h.ctx.Metrics().FilterNode(candidate, FilterConstraintHostVolumes)
	return false
}

func (h *HostVolumeChecker) hasVolumes(n *structs.Node) bool {
	rLen := len(h.volumes)
	hLen := len(n.HostVolumes)

	// Fast path: Requested no volumes. No need to check further.
	if rLen == 0 {
		return true
	}

	// Fast path: Requesting more volumes than the node has, can't meet the criteria.
	if rLen > hLen {
		return false
	}

	for source, requests := range h.volumes {
		nodeVolume, ok := n.HostVolumes[source]
		if !ok {
			return false
		}

		// If the volume supports being mounted as ReadWrite, we do not need to
		// do further validation for readonly placement.
		if !nodeVolume.ReadOnly {
			continue
		}

		// The Volume can only be mounted ReadOnly, validate that no requests for
		// it are ReadWrite.
		for _, req := range requests {
			if !req.ReadOnly {
				return false
			}
		}
	}

	return true
}

type CSIVolumeChecker struct {
	ctx       Context
	namespace string
	jobID     string
	volumes   map[string]*structs.VolumeRequest
}

func NewCSIVolumeChecker(ctx Context) *CSIVolumeChecker {
	return &CSIVolumeChecker{
		ctx: ctx,
	}
}

func (c *CSIVolumeChecker) SetJobID(jobID string) {
	c.jobID = jobID
}

func (c *CSIVolumeChecker) SetNamespace(namespace string) {
	c.namespace = namespace
}

func (c *CSIVolumeChecker) SetVolumes(allocName string, volumes map[string]*structs.VolumeRequest) {

	xs := make(map[string]*structs.VolumeRequest)

	// Filter to only CSI Volumes
	for alias, req := range volumes {
		if req.Type != structs.VolumeTypeCSI {
			continue
		}
		if req.PerAlloc {
			// provide a unique volume source per allocation
			copied := req.Copy()
			copied.Source = copied.Source + structs.AllocSuffix(allocName)
			xs[alias] = copied
		} else {
			xs[alias] = req
		}
	}
	c.volumes = xs
}

func (c *CSIVolumeChecker) Feasible(n *structs.Node) bool {
	ok, failReason := c.isFeasible(n)
	if ok {
		return true
	}

	c.ctx.Metrics().FilterNode(n, failReason)
	return false
}

func (c *CSIVolumeChecker) isFeasible(n *structs.Node) (bool, string) {
	// We can mount the volume if
	// - if required, a healthy controller plugin is running the driver
	// - the volume has free claims, or this job owns the claims
	// - this node is running the node plugin, implies matching topology

	// Fast path: Requested no volumes. No need to check further.
	if len(c.volumes) == 0 {
		return true, ""
	}

	ws := memdb.NewWatchSet()

	// Find the count per plugin for this node, so that can enforce MaxVolumes
	pluginCount := map[string]int64{}
	iter, err := c.ctx.State().CSIVolumesByNodeID(ws, "", n.ID)
	if err != nil {
		return false, FilterConstraintCSIVolumesLookupFailed
	}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		vol, ok := raw.(*structs.CSIVolume)
		if !ok {
			continue
		}
		pluginCount[vol.PluginID] += 1
	}

	// For volume requests, find volumes and determine feasibility
	for _, req := range c.volumes {
		vol, err := c.ctx.State().CSIVolumeByID(ws, c.namespace, req.Source)
		if err != nil {
			return false, FilterConstraintCSIVolumesLookupFailed
		}
		if vol == nil {
			return false, fmt.Sprintf(FilterConstraintCSIVolumeNotFoundTemplate, req.Source)
		}

		// Check that this node has a healthy running plugin with the right PluginID
		plugin, ok := n.CSINodePlugins[vol.PluginID]
		if !ok {
			return false, fmt.Sprintf(FilterConstraintCSIPluginTemplate, vol.PluginID, n.ID)
		}
		if !plugin.Healthy {
			return false, fmt.Sprintf(FilterConstraintCSIPluginUnhealthyTemplate, vol.PluginID, n.ID)
		}
		if pluginCount[vol.PluginID] >= plugin.NodeInfo.MaxVolumes {
			return false, fmt.Sprintf(FilterConstraintCSIPluginMaxVolumesTemplate, vol.PluginID, n.ID)
		}

		// CSI spec: "If requisite is specified, the provisioned
		// volume MUST be accessible from at least one of the
		// requisite topologies."
		if len(vol.Topologies) > 0 {
			if !plugin.NodeInfo.AccessibleTopology.MatchFound(vol.Topologies) {
				return false, FilterConstraintsCSIPluginTopology
			}
		}

		if req.ReadOnly {
			if !vol.ReadSchedulable() {
				return false, fmt.Sprintf(FilterConstraintCSIVolumeNoReadTemplate, vol.ID)
			}
		} else {
			if !vol.WriteSchedulable() {
				return false, fmt.Sprintf(FilterConstraintCSIVolumeNoWriteTemplate, vol.ID)
			}
			if !vol.HasFreeWriteClaims() {
				for id := range vol.WriteAllocs {
					a, err := c.ctx.State().AllocByID(ws, id)
					// the alloc for this blocking claim has been
					// garbage collected but the volumewatcher hasn't
					// finished releasing the claim (and possibly
					// detaching the volume), so we need to block
					// until it can be scheduled
					if err != nil || a == nil {
						return false, fmt.Sprintf(
							FilterConstraintCSIVolumeGCdAllocationTemplate, vol.ID, id)
					} else if a.Namespace != c.namespace || a.JobID != c.jobID {
						// the blocking claim is for another live job
						// so it's legitimately blocking more write
						// claims
						return false, fmt.Sprintf(
							FilterConstraintCSIVolumeInUseTemplate, vol.ID)
					}
				}
			}
		}
	}

	return true, ""
}

// NetworkChecker is a FeasibilityChecker which returns whether a node has the
// network resources necessary to schedule the task group
type NetworkChecker struct {
	ctx         Context
	networkMode string
	ports       []structs.Port
}

func NewNetworkChecker(ctx Context) *NetworkChecker {
	return &NetworkChecker{ctx: ctx, networkMode: "host"}
}

func (c *NetworkChecker) SetNetwork(network *structs.NetworkResource) {
	c.networkMode = network.Mode
	if c.networkMode == "" {
		c.networkMode = "host"
	}

	c.ports = make([]structs.Port, len(network.DynamicPorts)+len(network.ReservedPorts))
	c.ports = append(c.ports, network.DynamicPorts...)
	c.ports = append(c.ports, network.ReservedPorts...)
}

func (c *NetworkChecker) Feasible(option *structs.Node) bool {
	// Allow jobs not requiring any network resources
	if c.networkMode == "none" {
		return true
	}

	if !c.hasNetwork(option) {

		// special case - if the client is running a version older than 0.12 but
		// the server is 0.12 or newer, we need to maintain an upgrade path for
		// jobs looking for a bridge network that will not have been fingerprinted
		// on the client (which was added in 0.12)
		if c.networkMode == "bridge" {
			sv, err := version.NewSemver(option.Attributes["nomad.version"])
			if err == nil && predatesBridgeFingerprint.Check(sv) {
				return true
			}
		}

		c.ctx.Metrics().FilterNode(option, "missing network")
		return false
	}

	if c.ports != nil {
		if !c.hasHostNetworks(option) {
			return false
		}
	}

	return true
}

func (c *NetworkChecker) hasHostNetworks(option *structs.Node) bool {
	for _, port := range c.ports {
		if port.HostNetwork != "" {
			hostNetworkValue, hostNetworkOk := resolveTarget(port.HostNetwork, option)
			if !hostNetworkOk {
				c.ctx.Metrics().FilterNode(option, fmt.Sprintf("invalid host network %q template for port %q", port.HostNetwork, port.Label))
				return false
			}
			found := false
			for _, net := range option.NodeResources.NodeNetworks {
				if net.HasAlias(hostNetworkValue) {
					found = true
					break
				}
			}
			if !found {
				c.ctx.Metrics().FilterNode(option, fmt.Sprintf("missing host network %q for port %q", hostNetworkValue, port.Label))
				return false
			}
		}
	}
	return true
}

func (c *NetworkChecker) hasNetwork(option *structs.Node) bool {
	if option.NodeResources == nil {
		return false
	}

	for _, nw := range option.NodeResources.Networks {
		mode := nw.Mode
		if mode == "" {
			mode = "host"
		}
		if mode == c.networkMode {
			return true
		}
	}

	return false
}

// DriverChecker is a FeasibilityChecker which returns whether a node has the
// drivers necessary to scheduler a task group.
type DriverChecker struct {
	ctx     Context
	drivers map[string]struct{}
}

// NewDriverChecker creates a DriverChecker from a set of drivers
func NewDriverChecker(ctx Context, drivers map[string]struct{}) *DriverChecker {
	return &DriverChecker{
		ctx:     ctx,
		drivers: drivers,
	}
}

func (c *DriverChecker) SetDrivers(d map[string]struct{}) {
	c.drivers = d
}

func (c *DriverChecker) Feasible(option *structs.Node) bool {
	// Use this node if possible
	if c.hasDrivers(option) {
		return true
	}
	c.ctx.Metrics().FilterNode(option, FilterConstraintDrivers)
	return false
}

// hasDrivers is used to check if the node has all the appropriate
// drivers for this task group. Drivers are registered as node attribute
// like "driver.docker=1" with their corresponding version.
func (c *DriverChecker) hasDrivers(option *structs.Node) bool {
	for driver := range c.drivers {
		driverStr := fmt.Sprintf("driver.%s", driver)

		// COMPAT: Remove in 0.10: As of Nomad 0.8, nodes have a DriverInfo that
		// corresponds with every driver. As a Nomad server might be on a later
		// version than a Nomad client, we need to check for compatibility here
		// to verify the client supports this.
		if driverInfo, ok := option.Drivers[driver]; ok {
			if driverInfo == nil {
				c.ctx.Logger().Named("driver_checker").Warn("node has no driver info set", "node_id", option.ID, "driver", driver)
				return false
			}

			if driverInfo.Detected && driverInfo.Healthy {
				continue
			} else {
				return false
			}
		}

		value, ok := option.Attributes[driverStr]
		if !ok {
			return false
		}

		enabled, err := strconv.ParseBool(value)
		if err != nil {
			c.ctx.Logger().Named("driver_checker").Warn("node has invalid driver setting", "node_id", option.ID, "driver", driver, "val", value)
			return false
		}

		if !enabled {
			return false
		}
	}

	return true
}

// DistinctHostsIterator is a FeasibleIterator which returns nodes that pass the
// distinct_hosts constraint. The constraint ensures that multiple allocations
// do not exist on the same node.
type DistinctHostsIterator struct {
	ctx    Context
	source FeasibleIterator
	tg     *structs.TaskGroup
	job    *structs.Job

	// Store whether the Job or TaskGroup has a distinct_hosts constraints so
	// they don't have to be calculated every time Next() is called.
	tgDistinctHosts  bool
	jobDistinctHosts bool
}

// NewDistinctHostsIterator creates a DistinctHostsIterator from a source.
func NewDistinctHostsIterator(ctx Context, source FeasibleIterator) *DistinctHostsIterator {
	return &DistinctHostsIterator{
		ctx:    ctx,
		source: source,
	}
}

func (iter *DistinctHostsIterator) SetTaskGroup(tg *structs.TaskGroup) {
	iter.tg = tg
	iter.tgDistinctHosts = iter.hasDistinctHostsConstraint(tg.Constraints)
}

func (iter *DistinctHostsIterator) SetJob(job *structs.Job) {
	iter.job = job
	iter.jobDistinctHosts = iter.hasDistinctHostsConstraint(job.Constraints)
}

func (iter *DistinctHostsIterator) hasDistinctHostsConstraint(constraints []*structs.Constraint) bool {
	for _, con := range constraints {
		if con.Operand == structs.ConstraintDistinctHosts {
			// distinct_hosts defaults to true
			if con.RTarget == "" {
				return true
			}
			enabled, err := strconv.ParseBool(con.RTarget)
			// If the value is unparsable as a boolean, fall back to the old behavior
			// of enforcing the constraint when it appears.
			return err != nil || enabled
		}
	}

	return false
}

func (iter *DistinctHostsIterator) Next() *structs.Node {
	for {
		// Get the next option from the source
		option := iter.source.Next()

		// Hot-path if the option is nil or there are no distinct_hosts or
		// distinct_property constraints.
		hosts := iter.jobDistinctHosts || iter.tgDistinctHosts
		if option == nil || !hosts {
			return option
		}

		// Check if the host constraints are satisfied
		if !iter.satisfiesDistinctHosts(option) {
			iter.ctx.Metrics().FilterNode(option, structs.ConstraintDistinctHosts)
			continue
		}

		return option
	}
}

// satisfiesDistinctHosts checks if the node satisfies a distinct_hosts
// constraint either specified at the job level or the TaskGroup level.
func (iter *DistinctHostsIterator) satisfiesDistinctHosts(option *structs.Node) bool {
	// Check if there is no constraint set.
	if !(iter.jobDistinctHosts || iter.tgDistinctHosts) {
		return true
	}

	// Get the proposed allocations
	proposed, err := iter.ctx.ProposedAllocs(option.ID)
	if err != nil {
		iter.ctx.Logger().Named("distinct_hosts").Error("failed to get proposed allocations", "error", err)
		return false
	}

	// Skip the node if the task group has already been allocated on it.
	for _, alloc := range proposed {
		// If the job has a distinct_hosts constraint we only need an alloc
		// collision on the JobID but if the constraint is on the TaskGroup then
		// we need both a job and TaskGroup collision.
		jobCollision := alloc.JobID == iter.job.ID
		taskCollision := alloc.TaskGroup == iter.tg.Name
		if iter.jobDistinctHosts && jobCollision || jobCollision && taskCollision {
			return false
		}
	}

	return true
}

func (iter *DistinctHostsIterator) Reset() {
	iter.source.Reset()
}

// DistinctPropertyIterator is a FeasibleIterator which returns nodes that pass the
// distinct_property constraint. The constraint ensures that multiple allocations
// do not use the same value of the given property.
type DistinctPropertyIterator struct {
	ctx    Context
	source FeasibleIterator
	tg     *structs.TaskGroup
	job    *structs.Job

	hasDistinctPropertyConstraints bool
	jobPropertySets                []*propertySet
	groupPropertySets              map[string][]*propertySet
}

// NewDistinctPropertyIterator creates a DistinctPropertyIterator from a source.
func NewDistinctPropertyIterator(ctx Context, source FeasibleIterator) *DistinctPropertyIterator {
	return &DistinctPropertyIterator{
		ctx:               ctx,
		source:            source,
		groupPropertySets: make(map[string][]*propertySet),
	}
}

func (iter *DistinctPropertyIterator) SetTaskGroup(tg *structs.TaskGroup) {
	iter.tg = tg

	// Build the property set at the taskgroup level
	if _, ok := iter.groupPropertySets[tg.Name]; !ok {
		for _, c := range tg.Constraints {
			if c.Operand != structs.ConstraintDistinctProperty {
				continue
			}

			pset := NewPropertySet(iter.ctx, iter.job)
			pset.SetTGConstraint(c, tg.Name)
			iter.groupPropertySets[tg.Name] = append(iter.groupPropertySets[tg.Name], pset)
		}
	}

	// Check if there is a distinct property
	iter.hasDistinctPropertyConstraints = len(iter.jobPropertySets) != 0 || len(iter.groupPropertySets[tg.Name]) != 0
}

func (iter *DistinctPropertyIterator) SetJob(job *structs.Job) {
	iter.job = job

	// Build the property set at the job level
	for _, c := range job.Constraints {
		if c.Operand != structs.ConstraintDistinctProperty {
			continue
		}

		pset := NewPropertySet(iter.ctx, job)
		pset.SetJobConstraint(c)
		iter.jobPropertySets = append(iter.jobPropertySets, pset)
	}
}

func (iter *DistinctPropertyIterator) Next() *structs.Node {
	for {
		// Get the next option from the source
		option := iter.source.Next()

		// Hot path if there is nothing to check
		if option == nil || !iter.hasDistinctPropertyConstraints {
			return option
		}

		// Check if the constraints are met
		if !iter.satisfiesProperties(option, iter.jobPropertySets) ||
			!iter.satisfiesProperties(option, iter.groupPropertySets[iter.tg.Name]) {
			continue
		}

		return option
	}
}

// satisfiesProperties returns whether the option satisfies the set of
// properties. If not it will be filtered.
func (iter *DistinctPropertyIterator) satisfiesProperties(option *structs.Node, set []*propertySet) bool {
	for _, ps := range set {
		if satisfies, reason := ps.SatisfiesDistinctProperties(option, iter.tg.Name); !satisfies {
			iter.ctx.Metrics().FilterNode(option, reason)
			return false
		}
	}

	return true
}

func (iter *DistinctPropertyIterator) Reset() {
	iter.source.Reset()

	for _, ps := range iter.jobPropertySets {
		ps.PopulateProposed()
	}

	for _, sets := range iter.groupPropertySets {
		for _, ps := range sets {
			ps.PopulateProposed()
		}
	}
}

// ConstraintChecker is a FeasibilityChecker which returns nodes that match a
// given set of constraints. This is used to filter on job, task group, and task
// constraints.
type ConstraintChecker struct {
	ctx         Context
	constraints []*structs.Constraint
}

// NewConstraintChecker creates a ConstraintChecker for a set of constraints
func NewConstraintChecker(ctx Context, constraints []*structs.Constraint) *ConstraintChecker {
	return &ConstraintChecker{
		ctx:         ctx,
		constraints: constraints,
	}
}

func (c *ConstraintChecker) SetConstraints(constraints []*structs.Constraint) {
	c.constraints = constraints
}

func (c *ConstraintChecker) Feasible(option *structs.Node) bool {
	// Use this node if possible
	for _, constraint := range c.constraints {
		if !c.meetsConstraint(constraint, option) {
			c.ctx.Metrics().FilterNode(option, constraint.String())
			return false
		}
	}
	return true
}

func (c *ConstraintChecker) meetsConstraint(constraint *structs.Constraint, option *structs.Node) bool {
	// Resolve the targets. Targets that are not present are treated as `nil`.
	// This is to allow for matching constraints where a target is not present.
	lVal, lOk := resolveTarget(constraint.LTarget, option)
	rVal, rOk := resolveTarget(constraint.RTarget, option)

	// Check if satisfied
	return checkConstraint(c.ctx, constraint.Operand, lVal, rVal, lOk, rOk)
}

// resolveTarget is used to resolve the LTarget and RTarget of a Constraint.
func resolveTarget(target string, node *structs.Node) (string, bool) {
	// If no prefix, this must be a literal value
	if !strings.HasPrefix(target, "${") {
		return target, true
	}

	// Handle the interpolations
	switch {
	case "${node.unique.id}" == target:
		return node.ID, true

	case "${node.datacenter}" == target:
		return node.Datacenter, true

	case "${node.unique.name}" == target:
		return node.Name, true

	case "${node.class}" == target:
		return node.NodeClass, true

	case "${node.pool}" == target:
		return node.NodePool, true

	case strings.HasPrefix(target, "${attr."):
		attr := strings.TrimSuffix(strings.TrimPrefix(target, "${attr."), "}")
		val, ok := node.Attributes[attr]
		return val, ok

	case strings.HasPrefix(target, "${meta."):
		meta := strings.TrimSuffix(strings.TrimPrefix(target, "${meta."), "}")
		val, ok := node.Meta[meta]
		return val, ok

	default:
		return "", false
	}
}

// checkConstraint checks if a constraint is satisfied. The lVal and rVal
// interfaces may be nil.
func checkConstraint(ctx Context, operand string, lVal, rVal interface{}, lFound, rFound bool) bool {
	// Check for constraints not handled by this checker.
	switch operand {
	case structs.ConstraintDistinctHosts, structs.ConstraintDistinctProperty:
		return true
	default:
		break
	}

	switch operand {
	case "=", "==", "is":
		return lFound && rFound && reflect.DeepEqual(lVal, rVal)
	case "!=", "not":
		return !reflect.DeepEqual(lVal, rVal)
	case "<", "<=", ">", ">=":
		return lFound && rFound && checkOrder(operand, lVal, rVal)
	case structs.ConstraintAttributeIsSet:
		return lFound
	case structs.ConstraintAttributeIsNotSet:
		return !lFound
	case structs.ConstraintVersion:
		parser := newVersionConstraintParser(ctx)
		return lFound && rFound && checkVersionMatch(ctx, parser, lVal, rVal)
	case structs.ConstraintSemver:
		parser := newSemverConstraintParser(ctx)
		return lFound && rFound && checkVersionMatch(ctx, parser, lVal, rVal)
	case structs.ConstraintRegex:
		return lFound && rFound && checkRegexpMatch(ctx, lVal, rVal)
	case structs.ConstraintSetContains, structs.ConstraintSetContainsAll:
		return lFound && rFound && checkSetContainsAll(ctx, lVal, rVal)
	case structs.ConstraintSetContainsAny:
		return lFound && rFound && checkSetContainsAny(lVal, rVal)
	default:
		return false
	}
}

// checkAffinity checks if a specific affinity is satisfied
func checkAffinity(ctx Context, operand string, lVal, rVal interface{}, lFound, rFound bool) bool {
	return checkConstraint(ctx, operand, lVal, rVal, lFound, rFound)
}

// checkAttributeAffinity checks if an affinity is satisfied
func checkAttributeAffinity(ctx Context, operand string, lVal, rVal *psstructs.Attribute, lFound, rFound bool) bool {
	return checkAttributeConstraint(ctx, operand, lVal, rVal, lFound, rFound)
}

// checkOrder returns the result of (lVal operand rVal). The comparison is
// done as integers if possible, or floats if possible, and lexically otherwise.
func checkOrder(operand string, lVal, rVal any) bool {
	left, leftOK := lVal.(string)
	right, rightOK := rVal.(string)
	if !leftOK || !rightOK {
		return false
	}
	if result, ok := checkIntegralOrder(operand, left, right); ok {
		return result
	}
	if result, ok := checkFloatOrder(operand, left, right); ok {
		return result
	}
	return checkLexicalOrder(operand, left, right)
}

// checkIntegralOrder compares lVal and rVal as integers if possible, or false otherwise.
func checkIntegralOrder(op, lVal, rVal string) (bool, bool) {
	left, lErr := strconv.ParseInt(lVal, 10, 64)
	if lErr != nil {
		return false, false
	}
	right, rErr := strconv.ParseInt(rVal, 10, 64)
	if rErr != nil {
		return false, false
	}
	return compareOrder(op, left, right), true
}

// checkFloatOrder compares lVal and rVal as floats if possible, or false otherwise.
func checkFloatOrder(op, lVal, rVal string) (bool, bool) {
	left, lErr := strconv.ParseFloat(lVal, 64)
	if lErr != nil {
		return false, false
	}
	right, rErr := strconv.ParseFloat(rVal, 64)
	if rErr != nil {
		return false, false
	}
	return compareOrder(op, left, right), true
}

// checkLexicalOrder compares lVal and rVal lexically.
func checkLexicalOrder(op string, lVal, rVal string) bool {
	return compareOrder[string](op, lVal, rVal)
}

// compareOrder returns the result of the expression (left op right)
func compareOrder[T constraints.Ordered](op string, left, right T) bool {
	switch op {
	case "<":
		return left < right
	case "<=":
		return left <= right
	case ">":
		return left > right
	case ">=":
		return left >= right
	default:
		return false
	}
}

// checkVersionMatch is used to compare a version on the
// left hand side with a set of constraints on the right hand side
func checkVersionMatch(_ Context, parse verConstraintParser, lVal, rVal interface{}) bool {
	// Parse the version
	var versionStr string
	switch v := lVal.(type) {
	case string:
		versionStr = v
	case int:
		versionStr = fmt.Sprintf("%d", v)
	default:
		return false
	}

	// Parse the version
	vers, err := version.NewVersion(versionStr)
	if err != nil {
		return false
	}

	// Constraint must be a string
	constraintStr, ok := rVal.(string)
	if !ok {
		return false
	}

	// Parse the constraints
	c := parse(constraintStr)
	if c == nil {
		return false
	}

	// Check the constraints against the version
	return c.Check(vers)
}

// checkAttributeVersionMatch is used to compare a version on the
// left hand side with a set of constraints on the right hand side
func checkAttributeVersionMatch(_ Context, parse verConstraintParser, lVal, rVal *psstructs.Attribute) bool {
	// Parse the version
	var versionStr string
	if s, ok := lVal.GetString(); ok {
		versionStr = s
	} else if i, ok := lVal.GetInt(); ok {
		versionStr = fmt.Sprintf("%d", i)
	} else {
		return false
	}

	// Parse the version
	vers, err := version.NewVersion(versionStr)
	if err != nil {
		return false
	}

	// Constraint must be a string
	constraintStr, ok := rVal.GetString()
	if !ok {
		return false
	}

	// Parse the constraints
	c := parse(constraintStr)
	if c == nil {
		return false
	}

	// Check the constraints against the version
	return c.Check(vers)
}

// checkRegexpMatch is used to compare a value on the
// left hand side with a regexp on the right hand side
func checkRegexpMatch(ctx Context, lVal, rVal interface{}) bool {
	// Ensure left-hand is string
	lStr, ok := lVal.(string)
	if !ok {
		return false
	}

	// Regexp must be a string
	regexpStr, ok := rVal.(string)
	if !ok {
		return false
	}

	// Check the cache
	cache := ctx.RegexpCache()
	re := cache[regexpStr]

	// Parse the regexp
	if re == nil {
		var err error
		re, err = regexp.Compile(regexpStr)
		if err != nil {
			return false
		}
		cache[regexpStr] = re
	}

	// Look for a match
	return re.MatchString(lStr)
}

// checkSetContainsAll is used to see if the left hand side contains the
// string on the right hand side
func checkSetContainsAll(_ Context, lVal, rVal interface{}) bool {
	// Ensure left-hand is string
	lStr, ok := lVal.(string)
	if !ok {
		return false
	}

	// Regexp must be a string
	rStr, ok := rVal.(string)
	if !ok {
		return false
	}

	input := strings.Split(lStr, ",")
	lookup := make(map[string]struct{}, len(input))
	for _, in := range input {
		cleaned := strings.TrimSpace(in)
		lookup[cleaned] = struct{}{}
	}

	for _, r := range strings.Split(rStr, ",") {
		cleaned := strings.TrimSpace(r)
		if _, ok := lookup[cleaned]; !ok {
			return false
		}
	}

	return true
}

// checkSetContainsAny is used to see if the left hand side contains any
// values on the right hand side
func checkSetContainsAny(lVal, rVal interface{}) bool {
	// Ensure left-hand is string
	lStr, ok := lVal.(string)
	if !ok {
		return false
	}

	// RHS must be a string
	rStr, ok := rVal.(string)
	if !ok {
		return false
	}

	input := strings.Split(lStr, ",")
	lookup := make(map[string]struct{}, len(input))
	for _, in := range input {
		cleaned := strings.TrimSpace(in)
		lookup[cleaned] = struct{}{}
	}

	for _, r := range strings.Split(rStr, ",") {
		cleaned := strings.TrimSpace(r)
		if _, ok := lookup[cleaned]; ok {
			return true
		}
	}

	return false
}

// FeasibilityWrapper is a FeasibleIterator which wraps both job and task group
// FeasibilityCheckers in which feasibility checking can be skipped if the
// computed node class has previously been marked as eligible or ineligible.
type FeasibilityWrapper struct {
	ctx         Context
	source      FeasibleIterator
	jobCheckers []FeasibilityChecker
	tgCheckers  []FeasibilityChecker
	tgAvailable []FeasibilityChecker
	tg          string
}

// NewFeasibilityWrapper returns a FeasibleIterator based on the passed source
// and FeasibilityCheckers.
func NewFeasibilityWrapper(ctx Context, source FeasibleIterator,
	jobCheckers, tgCheckers, tgAvailable []FeasibilityChecker) *FeasibilityWrapper {
	return &FeasibilityWrapper{
		ctx:         ctx,
		source:      source,
		jobCheckers: jobCheckers,
		tgCheckers:  tgCheckers,
		tgAvailable: tgAvailable,
	}
}

func (w *FeasibilityWrapper) SetTaskGroup(tg string) {
	w.tg = tg
}

func (w *FeasibilityWrapper) Reset() {
	w.source.Reset()
}

// Next returns an eligible node, only running the FeasibilityCheckers as needed
// based on the sources computed node class.
func (w *FeasibilityWrapper) Next() *structs.Node {
	evalElig := w.ctx.Eligibility()
	metrics := w.ctx.Metrics()

OUTER:
	for {
		// Get the next option from the source
		option := w.source.Next()
		if option == nil {
			return nil
		}

		// Check if the job has been marked as eligible or ineligible.
		jobEscaped, jobUnknown := false, false
		switch evalElig.JobStatus(option.ComputedClass) {
		case EvalComputedClassIneligible:
			// Fast path the ineligible case
			metrics.FilterNode(option, "computed class ineligible")
			continue
		case EvalComputedClassEscaped:
			jobEscaped = true
		case EvalComputedClassUnknown:
			jobUnknown = true
		}

		// Run the job feasibility checks.
		for _, check := range w.jobCheckers {
			feasible := check.Feasible(option)
			if !feasible {
				// If the job hasn't escaped, set it to be ineligible since it
				// failed a job check.
				if !jobEscaped {
					evalElig.SetJobEligibility(false, option.ComputedClass)
				}
				continue OUTER
			}
		}

		// Set the job eligibility if the constraints weren't escaped and it
		// hasn't been set before.
		if !jobEscaped && jobUnknown {
			evalElig.SetJobEligibility(true, option.ComputedClass)
		}

		// Check if the task group has been marked as eligible or ineligible.
		tgEscaped, tgUnknown := false, false
		switch evalElig.TaskGroupStatus(w.tg, option.ComputedClass) {
		case EvalComputedClassIneligible:
			// Fast path the ineligible case
			metrics.FilterNode(option, "computed class ineligible")
			continue
		case EvalComputedClassEligible:
			// Fast path the eligible case
			if w.available(option) {
				return option
			}
			// We match the class but are temporarily unavailable
			continue OUTER
		case EvalComputedClassEscaped:
			tgEscaped = true
		case EvalComputedClassUnknown:
			tgUnknown = true
		}

		// Run the task group feasibility checks.
		for _, check := range w.tgCheckers {
			feasible := check.Feasible(option)
			if !feasible {
				// If the task group hasn't escaped, set it to be ineligible
				// since it failed a check.
				if !tgEscaped {
					evalElig.SetTaskGroupEligibility(false, w.tg, option.ComputedClass)
				}
				continue OUTER
			}
		}

		// Set the task group eligibility if the constraints weren't escaped and
		// it hasn't been set before.
		if !tgEscaped && tgUnknown {
			evalElig.SetTaskGroupEligibility(true, w.tg, option.ComputedClass)
		}

		// tgAvailable handlers are available transiently, so we test them without
		// affecting the computed class
		if !w.available(option) {
			continue OUTER
		}

		return option
	}
}

// available checks transient feasibility checkers which depend on changing conditions,
// e.g. the health status of a plugin or driver
func (w *FeasibilityWrapper) available(option *structs.Node) bool {
	// If we don't have any availability checks, we're available
	if len(w.tgAvailable) == 0 {
		return true
	}

	for _, check := range w.tgAvailable {
		if !check.Feasible(option) {
			return false
		}
	}
	return true
}

// DeviceChecker is a FeasibilityChecker which returns whether a node has the
// devices necessary to scheduler a task group.
type DeviceChecker struct {
	ctx Context

	// required is the set of requested devices that must exist on the node
	required []*structs.RequestedDevice

	// requiresDevices indicates if the task group requires devices
	requiresDevices bool
}

// NewDeviceChecker creates a DeviceChecker
func NewDeviceChecker(ctx Context) *DeviceChecker {
	return &DeviceChecker{
		ctx: ctx,
	}
}

func (c *DeviceChecker) SetTaskGroup(tg *structs.TaskGroup) {
	c.required = nil
	for _, task := range tg.Tasks {
		c.required = append(c.required, task.Resources.Devices...)
	}
	c.requiresDevices = len(c.required) != 0
}

func (c *DeviceChecker) Feasible(option *structs.Node) bool {
	if c.hasDevices(option) {
		return true
	}

	c.ctx.Metrics().FilterNode(option, FilterConstraintDevices)
	return false
}

func (c *DeviceChecker) hasDevices(option *structs.Node) bool {
	if !c.requiresDevices {
		return true
	}

	// COMPAT(0.11): Remove in 0.11
	// The node does not have the new resources object so it can not have any
	// devices
	if option.NodeResources == nil {
		return false
	}

	// Check if the node has any devices
	nodeDevs := option.NodeResources.Devices
	if len(nodeDevs) == 0 {
		return false
	}

	// Create a mapping of node devices to the remaining count
	available := make(map[*structs.NodeDeviceResource]uint64, len(nodeDevs))
	for _, d := range nodeDevs {
		var healthy uint64 = 0
		for _, instance := range d.Instances {
			if instance.Healthy {
				healthy++
			}
		}
		if healthy != 0 {
			available[d] = healthy
		}
	}

	// Go through the required devices trying to find matches
OUTER:
	for _, req := range c.required {
		// Determine how many there are to place
		desiredCount := req.Count

		// Go through the device resources and see if we have a match
		for d, unused := range available {
			if unused == 0 {
				// Depleted
				continue
			}

			// First check we have enough instances of the device since this is
			// cheaper than checking the constraints
			if unused < desiredCount {
				continue
			}

			// Check the constraints
			if nodeDeviceMatches(c.ctx, d, req) {
				// Consume the instances
				available[d] -= desiredCount

				// Move on to the next request
				continue OUTER
			}
		}

		// We couldn't match the request for the device
		return false
	}

	// Only satisfied if there are no more devices to place
	return true
}

// nodeDeviceMatches checks if the device matches the request and its
// constraints. It doesn't check the count.
func nodeDeviceMatches(ctx Context, d *structs.NodeDeviceResource, req *structs.RequestedDevice) bool {
	if !d.ID().Matches(req.ID()) {
		return false
	}

	// There are no constraints to consider
	if len(req.Constraints) == 0 {
		return true
	}

	for _, c := range req.Constraints {
		// Resolve the targets
		lVal, lOk := resolveDeviceTarget(c.LTarget, d)
		rVal, rOk := resolveDeviceTarget(c.RTarget, d)

		// Check if satisfied
		if !checkAttributeConstraint(ctx, c.Operand, lVal, rVal, lOk, rOk) {
			return false
		}
	}

	return true
}

// resolveDeviceTarget is used to resolve the LTarget and RTarget of a Constraint
// when used on a device
func resolveDeviceTarget(target string, d *structs.NodeDeviceResource) (*psstructs.Attribute, bool) {
	// If no prefix, this must be a literal value
	if !strings.HasPrefix(target, "${") {
		return psstructs.ParseAttribute(target), true
	}

	// Handle the interpolations
	switch {
	case "${device.ids}" == target:
		ids := make([]string, len(d.Instances))
		for i, device := range d.Instances {
			ids[i] = device.ID
		}
		return psstructs.NewStringAttribute(strings.Join(ids, ",")), true

	case "${device.model}" == target:
		return psstructs.NewStringAttribute(d.Name), true

	case "${device.vendor}" == target:
		return psstructs.NewStringAttribute(d.Vendor), true

	case "${device.type}" == target:
		return psstructs.NewStringAttribute(d.Type), true

	case strings.HasPrefix(target, "${device.attr."):
		attr := strings.TrimPrefix(target, "${device.attr.")
		attr = strings.TrimSuffix(attr, "}")
		val, ok := d.Attributes[attr]
		return val, ok

	default:
		return nil, false
	}
}

// checkAttributeConstraint checks if a constraint is satisfied. nil equality
// comparisons are considered to be false.
func checkAttributeConstraint(ctx Context, operand string, lVal, rVal *psstructs.Attribute, lFound, rFound bool) bool {
	// Check for constraints not handled by this checker.
	switch operand {
	case structs.ConstraintDistinctHosts, structs.ConstraintDistinctProperty:
		return true
	default:
		break
	}

	switch operand {
	case "!=", "not":
		// Neither value was provided, nil != nil == false
		if !(lFound || rFound) {
			return false
		}

		// Only 1 value was provided, therefore nil != some == true
		if lFound != rFound {
			return true
		}

		// Both values were provided, so actually compare them
		v, ok := lVal.Compare(rVal)
		if !ok {
			return false
		}

		return v != 0

	case "<", "<=", ">", ">=", "=", "==", "is":
		if !(lFound && rFound) {
			return false
		}

		v, ok := lVal.Compare(rVal)
		if !ok {
			return false
		}

		switch operand {
		case "is", "==", "=":
			return v == 0
		case "<":
			return v == -1
		case "<=":
			return v != 1
		case ">":
			return v == 1
		case ">=":
			return v != -1
		default:
			return false
		}

	case structs.ConstraintVersion:
		if !(lFound && rFound) {
			return false
		}

		parser := newVersionConstraintParser(ctx)
		return checkAttributeVersionMatch(ctx, parser, lVal, rVal)

	case structs.ConstraintSemver:
		if !(lFound && rFound) {
			return false
		}

		parser := newSemverConstraintParser(ctx)
		return checkAttributeVersionMatch(ctx, parser, lVal, rVal)

	case structs.ConstraintRegex:
		if !(lFound && rFound) {
			return false
		}

		ls, ok := lVal.GetString()
		rs, ok2 := rVal.GetString()
		if !ok || !ok2 {
			return false
		}
		return checkRegexpMatch(ctx, ls, rs)
	case structs.ConstraintSetContains, structs.ConstraintSetContainsAll:
		if !(lFound && rFound) {
			return false
		}

		ls, ok := lVal.GetString()
		rs, ok2 := rVal.GetString()
		if !ok || !ok2 {
			return false
		}

		return checkSetContainsAll(ctx, ls, rs)
	case structs.ConstraintSetContainsAny:
		if !(lFound && rFound) {
			return false
		}

		ls, ok := lVal.GetString()
		rs, ok2 := rVal.GetString()
		if !ok || !ok2 {
			return false
		}

		return checkSetContainsAny(ls, rs)
	case structs.ConstraintAttributeIsSet:
		return lFound
	case structs.ConstraintAttributeIsNotSet:
		return !lFound
	default:
		return false
	}

}

// VerConstraints is the interface implemented by both go-verson constraints
// and semver constraints.
type VerConstraints interface {
	Check(v *version.Version) bool
	String() string
}

// verConstraintParser returns a version constraints implementation (go-version
// or semver).
type verConstraintParser func(verConstraint string) VerConstraints

func newVersionConstraintParser(ctx Context) verConstraintParser {
	cache := ctx.VersionConstraintCache()

	return func(cstr string) VerConstraints {
		if c := cache[cstr]; c != nil {
			return c
		}

		constraint, err := version.NewConstraint(cstr)
		if err != nil {
			return nil
		}
		cache[cstr] = constraint

		return constraint
	}
}

func newSemverConstraintParser(ctx Context) verConstraintParser {
	cache := ctx.SemverConstraintCache()

	return func(cstr string) VerConstraints {
		if c := cache[cstr]; c != nil {
			return c
		}

		constraint, err := semver.NewConstraint(cstr)
		if err != nil {
			return nil
		}
		cache[cstr] = constraint

		return constraint
	}
}
