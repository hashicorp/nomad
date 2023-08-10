// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"golang.org/x/exp/maps"
)

// CSITopology is a map of topological domains to topological segments.
// A topological domain is a sub-division of a cluster, like "region",
// "zone", "rack", etc.
//
// According to CSI, there are a few requirements for the keys within this map:
//   - Valid keys have two segments: an OPTIONAL prefix and name, separated
//     by a slash (/), for example: "com.company.example/zone".
//   - The key name segment is REQUIRED. The prefix is OPTIONAL.
//   - The key name MUST be 63 characters or less, begin and end with an
//     alphanumeric character ([a-z0-9A-Z]), and contain only dashes (-),
//     underscores (_), dots (.), or alphanumerics in between, for example
//     "zone".
//   - The key prefix MUST be 63 characters or less, begin and end with a
//     lower-case alphanumeric character ([a-z0-9]), contain only
//     dashes (-), dots (.), or lower-case alphanumerics in between, and
//     follow domain name notation format
//     (https://tools.ietf.org/html/rfc1035#section-2.3.1).
//   - The key prefix SHOULD include the plugin's host company name and/or
//     the plugin name, to minimize the possibility of collisions with keys
//     from other plugins.
//   - If a key prefix is specified, it MUST be identical across all
//     topology keys returned by the SP (across all RPCs).
//   - Keys MUST be case-insensitive. Meaning the keys "Zone" and "zone"
//     MUST not both exist.
//   - Each value (topological segment) MUST contain 1 or more strings.
//   - Each string MUST be 63 characters or less and begin and end with an
//     alphanumeric character with '-', '_', '.', or alphanumerics in
//     between.
//
// However, Nomad applies lighter restrictions to these, as they are already
// only referenced by plugin within the scheduler and as such collisions and
// related concerns are less of an issue. We may implement these restrictions
// in the future.
type CSITopology struct {
	Segments map[string]string
}

func (t *CSITopology) Copy() *CSITopology {
	if t == nil {
		return nil
	}

	return &CSITopology{
		Segments: maps.Clone(t.Segments),
	}
}

func (t *CSITopology) Equal(o *CSITopology) bool {
	if t == nil || o == nil {
		return t == o
	}
	return maps.Equal(t.Segments, o.Segments)
}

func (t *CSITopology) MatchFound(o []*CSITopology) bool {
	if t == nil || o == nil || len(o) == 0 {
		return false
	}

	for _, other := range o {
		if t.Equal(other) {
			return true
		}
	}
	return false
}

// CSITopologyRequest are the topologies submitted as options to the
// storage provider at the time the volume was created. The storage
// provider will return a single topology.
type CSITopologyRequest struct {
	Required  []*CSITopology
	Preferred []*CSITopology
}

func (tr *CSITopologyRequest) Equal(o *CSITopologyRequest) bool {
	if tr == nil && o == nil {
		return true
	}
	if tr == nil && o != nil || tr != nil && o == nil {
		return false
	}
	if len(tr.Required) != len(o.Required) || len(tr.Preferred) != len(o.Preferred) {
		return false
	}
	for i, topo := range tr.Required {
		if !topo.Equal(o.Required[i]) {
			return false
		}
	}
	for i, topo := range tr.Preferred {
		if !topo.Equal(o.Preferred[i]) {
			return false
		}
	}
	return true
}

// CSINodeInfo is the fingerprinted data from a CSI Plugin that is specific to
// the Node API.
type CSINodeInfo struct {
	// ID is the identity of a given nomad client as observed by the storage
	// provider.
	ID string

	// MaxVolumes is the maximum number of volumes that can be attached to the
	// current host via this provider.
	// If 0 then unlimited volumes may be attached.
	MaxVolumes int64

	// AccessibleTopology specifies where (regions, zones, racks, etc.) the node is
	// accessible from within the storage provider.
	//
	// A plugin that returns this field MUST also set the `RequiresTopologies`
	// property.
	//
	// This field is OPTIONAL. If it is not specified, then we assume that the
	// the node is not subject to any topological constraint, and MAY
	// schedule workloads that reference any volume V, such that there are
	// no topological constraints declared for V.
	//
	// Example 1:
	//   accessible_topology =
	//     {"region": "R1", "zone": "Z2"}
	// Indicates the node exists within the "region" "R1" and the "zone"
	// "Z2" within the storage provider.
	AccessibleTopology *CSITopology

	// RequiresNodeStageVolume indicates whether the client should Stage/Unstage
	// volumes on this node.
	RequiresNodeStageVolume bool

	// SupportsStats indicates plugin support for GET_VOLUME_STATS
	SupportsStats bool

	// SupportsExpand indicates plugin support for EXPAND_VOLUME
	SupportsExpand bool

	// SupportsCondition indicates plugin support for VOLUME_CONDITION
	SupportsCondition bool
}

func (n *CSINodeInfo) Copy() *CSINodeInfo {
	if n == nil {
		return nil
	}

	nc := new(CSINodeInfo)
	*nc = *n
	nc.AccessibleTopology = n.AccessibleTopology.Copy()

	return nc
}

// CSIControllerInfo is the fingerprinted data from a CSI Plugin that is specific to
// the Controller API.
type CSIControllerInfo struct {

	// SupportsCreateDelete indicates plugin support for CREATE_DELETE_VOLUME
	SupportsCreateDelete bool

	// SupportsPublishVolume is true when the controller implements the
	// methods required to attach and detach volumes. If this is false Nomad
	// should skip the controller attachment flow.
	SupportsAttachDetach bool

	// SupportsListVolumes is true when the controller implements the
	// ListVolumes RPC. NOTE: This does not guarantee that attached nodes will
	// be returned unless SupportsListVolumesAttachedNodes is also true.
	SupportsListVolumes bool

	// SupportsGetCapacity indicates plugin support for GET_CAPACITY
	SupportsGetCapacity bool

	// SupportsCreateDeleteSnapshot indicates plugin support for
	// CREATE_DELETE_SNAPSHOT
	SupportsCreateDeleteSnapshot bool

	// SupportsListSnapshots indicates plugin support for LIST_SNAPSHOTS
	SupportsListSnapshots bool

	// SupportsClone indicates plugin support for CLONE_VOLUME
	SupportsClone bool

	// SupportsReadOnlyAttach is set to true when the controller returns the
	// ATTACH_READONLY capability.
	SupportsReadOnlyAttach bool

	// SupportsExpand indicates plugin support for EXPAND_VOLUME
	SupportsExpand bool

	// SupportsListVolumesAttachedNodes indicates whether the plugin will
	// return attached nodes data when making ListVolume RPCs (plugin support
	// for LIST_VOLUMES_PUBLISHED_NODES)
	SupportsListVolumesAttachedNodes bool

	// SupportsCondition indicates plugin support for VOLUME_CONDITION
	SupportsCondition bool

	// SupportsGet indicates plugin support for GET_VOLUME
	SupportsGet bool
}

func (c *CSIControllerInfo) Copy() *CSIControllerInfo {
	if c == nil {
		return nil
	}

	nc := new(CSIControllerInfo)
	*nc = *c

	return nc
}

// CSIInfo is the current state of a single CSI Plugin. This is updated regularly
// as plugin health changes on the node.
type CSIInfo struct {
	PluginID          string
	AllocID           string
	Healthy           bool
	HealthDescription string
	UpdateTime        time.Time

	Provider        string // vendor name from CSI GetPluginInfoResponse
	ProviderVersion string // vendor version from CSI GetPluginInfoResponse

	// RequiresControllerPlugin is set when the CSI Plugin returns the
	// CONTROLLER_SERVICE capability. When this is true, the volumes should not be
	// scheduled on this client until a matching controller plugin is available.
	RequiresControllerPlugin bool

	// RequiresTopologies is set when the CSI Plugin returns the
	// VOLUME_ACCESSIBLE_CONSTRAINTS capability. When this is true, we must
	// respect the Volume and Node Topology information.
	RequiresTopologies bool

	// CSI Specific metadata
	ControllerInfo *CSIControllerInfo `json:",omitempty"`
	NodeInfo       *CSINodeInfo       `json:",omitempty"`
}

func (c *CSIInfo) Copy() *CSIInfo {
	if c == nil {
		return nil
	}

	nc := new(CSIInfo)
	*nc = *c
	nc.ControllerInfo = c.ControllerInfo.Copy()
	nc.NodeInfo = c.NodeInfo.Copy()

	return nc
}

func (c *CSIInfo) SetHealthy(hs bool) {
	c.Healthy = hs
	if hs {
		c.HealthDescription = "healthy"
	} else {
		c.HealthDescription = "unhealthy"
	}
}

func (c *CSIInfo) Equal(o *CSIInfo) bool {
	if c == nil && o == nil {
		return c == o
	}

	nc := *c
	nc.UpdateTime = time.Time{}
	no := *o
	no.UpdateTime = time.Time{}

	return reflect.DeepEqual(nc, no)
}

func (c *CSIInfo) IsController() bool {
	if c == nil || c.ControllerInfo == nil {
		return false
	}
	return true
}

func (c *CSIInfo) IsNode() bool {
	if c == nil || c.NodeInfo == nil {
		return false
	}
	return true
}

// DriverInfo is the current state of a single driver. This is updated
// regularly as driver health changes on the node.
type DriverInfo struct {
	Attributes        map[string]string
	Detected          bool
	Healthy           bool
	HealthDescription string
	UpdateTime        time.Time
}

func (di *DriverInfo) Copy() *DriverInfo {
	if di == nil {
		return nil
	}

	cdi := new(DriverInfo)
	*cdi = *di
	cdi.Attributes = maps.Clone(di.Attributes)
	return cdi
}

// MergeHealthCheck merges information from a health check for a drier into a
// node's driver info
func (di *DriverInfo) MergeHealthCheck(other *DriverInfo) {
	di.Healthy = other.Healthy
	di.HealthDescription = other.HealthDescription
	di.UpdateTime = other.UpdateTime
}

// MergeFingerprintInfo merges information from fingerprinting a node for a
// driver into a node's driver info for that driver.
func (di *DriverInfo) MergeFingerprintInfo(other *DriverInfo) {
	di.Detected = other.Detected
	di.Attributes = other.Attributes
}

// HealthCheckEquals determines if two driver info objects are equal. As this
// is used in the process of health checking, we only check the fields that are
// computed by the health checker. In the future, this will be merged.
func (di *DriverInfo) HealthCheckEquals(other *DriverInfo) bool {
	if di == nil && other == nil {
		return true
	}

	if di.Healthy != other.Healthy {
		return false
	}

	if di.HealthDescription != other.HealthDescription {
		return false
	}

	return true
}

// NodeMetaApplyRequest is used to update Node metadata on Client agents.
type NodeMetaApplyRequest struct {
	QueryOptions // Client RPCs must use QueryOptions to set AllowStale=true

	// NodeID is the node being targeted by this request (or the node
	// receiving this request if NodeID is empty).
	NodeID string

	// Meta is the new Node metadata being applied and differs slightly
	// from Node.Meta as nil values are used to unset Node.Meta keys.
	Meta map[string]*string
}

func (n *NodeMetaApplyRequest) Validate() error {
	if len(n.Meta) == 0 {
		return fmt.Errorf("missing required Meta object")
	}
	for k := range n.Meta {
		if k == "" {
			return fmt.Errorf("metadata keys must not be empty")
		}

		// Validate keys are dotted identifiers since their primary use case is in
		// constraints as interpolated hcl variables.
		// https://github.com/hashicorp/hcl/blob/v2.16.0/hclsyntax/spec.md#identifiers
		for _, part := range strings.Split(k, ".") {
			if !hclsyntax.ValidIdentifier(part) {
				return fmt.Errorf("%q is invalid; metadata keys must be valid dotted hcl identifiers", k)
			}
		}
	}

	return nil
}

// NodeMetaResponse is used to read Node metadata directly from Client agents.
type NodeMetaResponse struct {
	// Meta is the merged static + dynamic Node metadata
	Meta map[string]string

	// Dynamic is the dynamic Node metadata (set via API)
	Dynamic map[string]*string

	// Static is the static Node metadata (set via agent configuration)
	Static map[string]string
}
