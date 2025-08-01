// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/hashicorp/go-hclog"
	metrics "github.com/hashicorp/go-metrics/compat"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/nomad/helper/uuid"
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

func (t *CSITopology) Contains(o *CSITopology) bool {
	if t == nil || o == nil {
		return t == o
	}

	for k, ov := range o.Segments {
		if tv, ok := t.Segments[k]; !ok || tv != ov {
			return false
		}
	}

	return true
}

func (t *CSITopology) MatchFound(o []*CSITopology) bool {
	if t == nil || o == nil || len(o) == 0 {
		return false
	}

	for _, other := range o {
		if t.Contains(other) {
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

// ScheduleStateApplyRequest is used to set the pause state of a specific task running
// on Client.
type ScheduleStateApplyRequest struct {
	QueryOptions // Client RPCs must use QueryOptions

	// NodeID is the node being targeted by this request (or the node receiving
	// this request if NodeID is empty).
	NodeID string

	// AllocID is the allocation being targeted by this request.
	AllocID string

	// TaskName is the name of the task being targeted by this request.
	TaskName string

	// State is the state to apply to the task being targeted by this request.
	ScheduleState TaskScheduleState
}

func (r *ScheduleStateApplyRequest) Validate() error {
	if r.AllocID == "" {
		return errors.New("alloc id must be set")
	}

	if r.TaskName == "" {
		return errors.New("task name must be set")
	}

	switch r.ScheduleState {
	case TaskScheduleStateRun:
	case TaskScheduleStateForceRun:
	case TaskScheduleStateSchedPause:
	case TaskScheduleStateForcePause:
	default:
		return errors.New("not a valid task schedule state")
	}

	return nil
}

// ScheduleStateReadRequest is used to read the current pause state of a specific
// task running on a client.
type ScheduleStateReadRequest struct {
	QueryOptions // Client RPCs must use QueryOptions

	// NodeID is the node being targeted by this request (or the node receiving
	// this request if NodeID is empty).
	NodeID string

	// AllocID is the allocation being targeted by this request.
	AllocID string

	// TaskName is the name of the task being targeted by this request.
	TaskName string
}

func (r *ScheduleStateReadRequest) Validate() error {
	if r.AllocID == "" {
		return errors.New("alloc id must be set")
	}

	if r.TaskName == "" {
		return errors.New("task name must be set")
	}

	return nil
}

// ScheduleStateResponse contains the current pause state of a specific task.
type ScheduleStateResponse struct {
	ScheduleState TaskScheduleState
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

// NodeIdentityClaims represents the claims for a Nomad node identity JWT.
type NodeIdentityClaims struct {
	NodeID         string `json:"nomad_node_id,omitempty"`
	NodePool       string `json:"nomad_node_pool,omitempty"`
	NodeClass      string `json:"nomad_node_class,omitempty"`
	NodeDatacenter string `json:"nomad_node_datacenter,omitempty"`
}

// GenerateNodeIdentityClaims creates a new NodeIdentityClaims for the given
// node and region. The returned claims will be ready for signing and returning
// to the node.
//
// The caller is responsible for ensuring that the passed arguments are valid.
func GenerateNodeIdentityClaims(node *Node, region string, ttl time.Duration) *IdentityClaims {

	// The time does not need to be passed into the function as an argument, as
	// we only create a single identity per node at a time. This explains the
	// difference with the workload identity claims, as each allocation can have
	// multiple identities.
	timeNow := time.Now().UTC()
	timeJWTNow := jwt.NewNumericDate(timeNow)

	claims := &IdentityClaims{
		NodeIdentityClaims: &NodeIdentityClaims{
			NodeID:         node.ID,
			NodePool:       node.NodePool,
			NodeClass:      node.NodeClass,
			NodeDatacenter: node.Datacenter,
		},
		Claims: jwt.Claims{
			ID:        uuid.Generate(),
			IssuedAt:  timeJWTNow,
			NotBefore: timeJWTNow,
		},
	}

	claims.setAudience([]string{IdentityDefaultAud})
	claims.setExpiry(timeNow, ttl)
	claims.setNodeSubject(node, region)

	return claims
}

// NodeRegisterRequest is used by the Node.Register RPC endpoint to register a
// node as being a schedulable entity.
type NodeRegisterRequest struct {
	Node      *Node
	NodeEvent *NodeEvent

	// CreateNodePool is used to indicate that the node's node pool should be
	// created along with the node registration if it doesn't exist.
	CreateNodePool bool

	WriteRequest
}

// ShouldGenerateNodeIdentity compliments the functionality within
// AuthenticateNodeIdentityGenerator to determine whether a new node identity
// should be generated within the RPC handler.
func (n *NodeRegisterRequest) ShouldGenerateNodeIdentity(
	authErr error,
	now time.Time,
	ttl time.Duration,
) bool {

	// In the event the error is because the node identity is expired, we should
	// generate a new identity. Without this, a disconnected node would never be
	// able to re-register. Any other error is not a reason to generate a new
	// identity.
	if authErr != nil {
		return errors.Is(authErr, jwt.ErrExpired)
	}

	// If an ACL token or client ID is set, a node is attempting to register for
	// the first time, or is re-registering using its secret ID. In either case,
	// we should generate a new identity.
	if n.identity.ACLToken != nil || n.identity.ClientID != "" {
		return true
	}

	// If we have reached this point, we can assume that the request is using a
	// node identity.
	claims := n.GetIdentity().GetClaims()

	// If the request was made with a node introduction identity, this is an
	// initial registration and we should generate a new identity.
	if claims.IsNodeIntroduction() {
		return true
	}

	// It is possible that the node has been restarted and had its configuration
	// updated. In this case, we should generate a new identity for the node, so
	// it reflects its new claims.
	if n.Node.NodePool != claims.NodeIdentityClaims.NodePool ||
		n.Node.NodeClass != claims.NodeIdentityClaims.NodeClass ||
		n.Node.Datacenter != claims.NodeIdentityClaims.NodeDatacenter {
		return true
	}

	// The final check is to see if the node identity is expiring.
	return claims.IsExpiring(now, ttl)
}

// NewRegistrationAllowed determines whether the node registration is allowed to
// proceed based on the node introduction enforcement level and the
// authenticated identity of the request.
//
// The function handles logging and emitting metrics based on the enforcement
// level, so the caller only needs to check the return value.
func (n *NodeRegisterRequest) NewRegistrationAllowed(
	logger hclog.Logger,
	authErr error,
	enforcementLvl string,
) bool {

	// If the enforcement level is set to "none", we allow the registration to
	// proceed without any checks. This is the pre-1.11 workflow that won't emit
	// any metrics or logs for node registrations.
	if enforcementLvl == NodeIntroductionEnforcementNone {
		return true
	}

	claims := n.GetIdentity().GetClaims()

	// If the request was made with a node introduction identity, check whether
	// the claims match the node's claims.
	var claimsMatch bool

	if claims.IsNodeIntroduction() {
		claimsMatch = claims.NodeIntroductionIdentityClaims.NodePool == n.Node.NodePool &&
			(claims.NodeIntroductionIdentityClaims.NodeName == "" ||
				claims.NodeIntroductionIdentityClaims.NodeName == n.Node.Name)
	}

	// If there was no authentication error and the identity claims match the
	// node's claims, the registration is allowed to proceed.
	if authErr == nil && claimsMatch {
		return true
	}

	// If we have reached this point, we know that the registration is dependent
	// on the enforcement level and that this request does not have suitable
	// claims. Emit our metric to indicate a node registration not using a valid
	// introduction token.
	metrics.IncrCounter([]string{"nomad", "client", "introduction_violation_num"}, 1)

	// Build a base set of logging pairs that will be used for the logging
	// message. This provides operators with information about the node that is
	// attempting to register without a valid introduction token.
	loggingPairs := []any{
		"enforcement_level", enforcementLvl,
		"node_id", n.Node.ID,
		"node_pool", n.Node.NodePool,
		"node_name", n.Node.Name,
	}

	// If the node used a node introduction identity, add the claims for
	// comparison to the logging pairs.
	if claims.IsNodeIntroduction() {
		loggingPairs = append(loggingPairs, claims.NodeIntroductionIdentityClaims.loggingPairs()...)
	}

	// Make some effort to log a message that indicates why the node is failing
	// to introduce itself properly.
	msg := "node registration introduction claims mismatch"

	if authErr != nil {
		msg = "node registration introduction authentication failure"
		loggingPairs = append(loggingPairs, "error", authErr)
	} else if n.identity.ACLToken == AnonymousACLToken {
		msg = "node registration without introduction token"
	}

	// Based on the enforcement level, log the message and return whether the
	// handler should allow the registration to proceed. The default is a
	// catchall that includes the strict enforcement level.
	switch enforcementLvl {
	case NodeIntroductionEnforcementWarn:
		logger.Warn(msg, loggingPairs...)
		return true
	default:
		logger.Error(msg, loggingPairs...)
		return false
	}
}

// NodeUpdateStatusRequest is used for Node.UpdateStatus endpoint
// to update the status of a node.
type NodeUpdateStatusRequest struct {
	NodeID string
	Status string

	// IdentitySigningKeyID is the ID of the root key used to sign the node's
	// identity. This is not provided by the client, but is set by the server,
	// so that the value can be propagated through Raft.
	IdentitySigningKeyID string

	// ForceIdentityRenewal is used to force the Nomad server to generate a new
	// identity for the node.
	ForceIdentityRenewal bool

	NodeEvent *NodeEvent
	UpdatedAt int64
	WriteRequest
}

// ShouldGenerateNodeIdentity determines whether the handler should generate a
// new node identity based on the caller identity information.
func (n *NodeUpdateStatusRequest) ShouldGenerateNodeIdentity(
	now time.Time,
	ttl time.Duration,
) bool {

	identity := n.GetIdentity()

	// If the client ID is set, we should generate a new identity as the node
	// has authenticated using its secret ID.
	if identity.ClientID != "" {
		return true
	}

	// Confirm we have a node identity and then check for forced renewal or
	// expiration.
	if identity.GetClaims().IsNode() {
		if n.ForceIdentityRenewal {
			return true
		}
		return n.GetIdentity().GetClaims().IsExpiring(now, ttl)
	}

	// No other conditions should generate a new identity. In the case of the
	// update status endpoint, this will likely be a Nomad server propagating
	// that a node has missed its heartbeat.
	return false
}

// IdentitySigningErrorIsTerminal determines if the RPC handler should return an
// error because it failed to sign a newly generated node identity.
//
// This is because a client might be connected to a follower at the point the
// root keyring is rotated. If the client heartbeats right at that moment and
// before the follower decrypts the key (e.g., network latency to external KMS),
// we will mark the node as down. This is despite identity being valid and the
// likelihood it will get a new identity signed on the next heartbeat.
func (n *NodeUpdateStatusRequest) IdentitySigningErrorIsTerminal(now time.Time) bool {

	identity := n.GetIdentity()

	// If the client has authenticated using a secret ID, we can continue to let
	// it do that, until we successfully generate a new identity.
	if identity.ClientID != "" {
		return false
	}

	// If the identity is a node identity, we can check if it is expiring. This
	// check is used to determine if the RPC handler should return an error, so
	// we use a short threshold of 10 minutes. This is to ensure we don't return
	// errors unless we absolutely have to.
	//
	// A threshold of 10 minutes more than covers another heartbeat on the
	// largest Nomad clusters, which can reach ~5 minutes.
	if identity.GetClaims().IsNode() {
		return n.GetIdentity().GetClaims().IsExpiringInThreshold(now.Add(10 * time.Minute))
	}

	// No other condition should result in the RPC handler returning an error
	// because we failed to sign the node identity. No caller should be able to
	// reach this point, as identity generation should be gated by
	// ShouldGenerateNodeIdentity.
	return false
}

// NodeUpdateResponse is used to respond to a node update. The object is a
// shared response used by the Node.Register, Node.Deregister,
// Node.BatchDeregister, Node.UpdateStatus, and Node.Evaluate RPCs.
type NodeUpdateResponse struct {
	HeartbeatTTL    time.Duration
	EvalIDs         []string
	EvalCreateIndex uint64
	NodeModifyIndex uint64

	// Features informs clients what enterprise features are allowed
	Features uint64

	// LeaderRPCAddr is the RPC address of the current Raft Leader.  If
	// empty, the current Nomad Server is in the minority of a partition.
	LeaderRPCAddr string

	// NumNodes is the number of Nomad nodes attached to this quorum of
	// Nomad Servers at the time of the response.  This value can
	// fluctuate based on the health of the cluster between heartbeats.
	NumNodes int32

	// Servers is the full list of known Nomad servers in the local
	// region.
	Servers []*NodeServerInfo

	// SchedulingEligibility is used to inform clients what the server-side
	// has for their scheduling status during heartbeats.
	SchedulingEligibility string

	// SignedIdentity is the newly signed node identity that the server has
	// generated. The node should check if this is set, and if so, update its
	// state with the new identity.
	SignedIdentity *string

	QueryMeta
}

const (
	// NodeIdentityRenewRPCMethod is the RPC method for instructing a client to
	// forcibly request a renewal of its node identity at the next heartbeat.
	//
	// Args: NodeIdentityRenewReq
	// Reply: NodeIdentityRenewResp
	NodeIdentityRenewRPCMethod = "NodeIdentity.Renew"
)

// NodeIdentityRenewReq is used to instruct the Nomad server to renew the client
// identity at its next heartbeat regardless of whether it is close to
// expiration.
type NodeIdentityRenewReq struct {
	NodeID string

	// This is a client RPC, so we must use query options which allow us to set
	// AllowStale=true.
	QueryOptions
}

type NodeIdentityRenewResp struct{}

// DefaultNodeIntroductionConfig returns a default and fully hydrated
// configuration object for the node introduction feature.
func DefaultNodeIntroductionConfig() *NodeIntroductionConfig {
	return &NodeIntroductionConfig{
		Enforcement:        NodeIntroductionEnforcementWarn,
		DefaultIdentityTTL: 5 * time.Minute,
		MaxIdentityTTL:     30 * time.Minute,
	}
}

const (
	// NodeIntroductionEnforcementNone means secure intro token, and the
	// pre-1.11 workflow that uses an empty authentication token can be used for
	// initial client registration. No enforcement is applied and no emits are
	// emitted based on the registration introduction.
	NodeIntroductionEnforcementNone = "none"

	// NodeIntroductionEnforcementWarn means secure intro token, and the
	// pre-1.11 workflow that uses an empty authentication token can be used for
	// initial client registration. The server will emit a log and a metric for
	// each registration that does not use an introduction token.
	NodeIntroductionEnforcementWarn = "warn"

	// NodeIntroductionEnforcementStrict means initial client registration must
	// use a secure introduction token. The server will reject any registration
	// that does not use an introduction token.
	NodeIntroductionEnforcementStrict = "strict"
)

// NodeIntroductionConfig is the server configuration block that configures
// the client introduction feature. This feature allows servers to validate
// requests and perform enforcement actions on client registrations.
type NodeIntroductionConfig struct {

	// Enforcement is the level of enforcement that the server will apply to
	// client registrations. This can be one of "none", "warn", or "strict"
	// which are defined as NodeIntroductionEnforcementNone,
	// NodeIntroductionEnforcementWarn, and NodeIntroductionEnforcementStrict
	// respectively.
	Enforcement string

	// DefaultIdentityTTL is the TTL assigned to client introduction identities
	// that are generated by a caller who did not provide a TTL.
	DefaultIdentityTTL time.Duration

	// MaxIdentityTTL is the maximum TTL that can be assigned to a client
	// introduction identity. This is used to validate the TTL provided by
	// caller and allows operators a method to limit TTL requests.
	MaxIdentityTTL time.Duration
}

// Copy creates a copy of the node introduction configuration block.
func (n *NodeIntroductionConfig) Copy() *NodeIntroductionConfig {
	if n == nil {
		return nil
	}

	newCI := *n
	return &newCI
}

// Validate checks that the node introduction configuration is valid.
func (n *NodeIntroductionConfig) Validate() error {

	if n == nil {
		return fmt.Errorf("cannot be empty")
	}

	var mErr *multierror.Error

	switch n.Enforcement {
	case NodeIntroductionEnforcementNone,
		NodeIntroductionEnforcementWarn,
		NodeIntroductionEnforcementStrict:
	default:
		mErr = multierror.Append(mErr, fmt.Errorf("invalid enforcement %q", n.Enforcement))
	}

	if n.DefaultIdentityTTL < 1 {
		mErr = multierror.Append(mErr, errors.New("default_identity_ttl must be greater than 0"))
	}

	if n.MaxIdentityTTL < 1 {
		mErr = multierror.Append(mErr, errors.New("max_identity_ttl must be greater than 0"))
	}

	if n.MaxIdentityTTL < n.DefaultIdentityTTL {
		mErr = multierror.Append(mErr, errors.New(
			"max_identity_ttl must be greater than or equal to default_identity_ttl",
		))
	}

	return mErr.ErrorOrNil()
}

// NodeIntroductionIdentityClaims contains the claims for node introduction.
type NodeIntroductionIdentityClaims struct {
	NodePool string `json:"nomad_node_pool"`
	NodeName string `json:"nomad_node_name"`
}

// GenerateNodeIntroductionIdentityClaims generates a new identity JWT for node
// introduction.
//
// The caller is responsible for ensuring that the passed arguments are valid.
func GenerateNodeIntroductionIdentityClaims(name, pool, region string, ttl time.Duration) *IdentityClaims {

	timeNow := time.Now().UTC()
	timeJWTNow := jwt.NewNumericDate(timeNow)

	claims := &IdentityClaims{
		NodeIntroductionIdentityClaims: &NodeIntroductionIdentityClaims{
			NodePool: pool,
			NodeName: name,
		},
		Claims: jwt.Claims{
			ID:        uuid.Generate(),
			IssuedAt:  timeJWTNow,
			NotBefore: timeJWTNow,
		},
	}

	claims.setAudience([]string{IdentityDefaultAud})
	claims.setExpiry(timeNow, ttl)
	claims.setNodeIntroductionSubject(name, pool, region)

	return claims
}

// loggingPairs returns a set of key-value pairs that can be used for logging
// purposes.
func (n *NodeIntroductionIdentityClaims) loggingPairs() []any {

	// The node pool is a required field on the node introduction identity, so
	// we can always include it in the logging pairs.
	pairs := []any{"claim_node_pool", n.NodePool}

	// The node name is optional, so we only include it if it is set.
	if n.NodeName != "" {
		pairs = append(pairs, "claim_node_name", n.NodeName)
	}

	return pairs
}
