// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"time"

	"github.com/hashicorp/raft"
)

// RaftServer has information about a server in the Raft configuration.
type RaftServer struct {
	// ID is the unique ID for the server. These are currently the same
	// as the address, but they will be changed to a real GUID in a future
	// release of Nomad.
	ID raft.ServerID

	// Node is the node name of the server, as known by Nomad, or this
	// will be set to "(unknown)" otherwise.
	Node string

	// Address is the IP:port of the server, used for Raft communications.
	Address raft.ServerAddress

	// Leader is true if this server is the current cluster leader.
	Leader bool

	// Voter is true if this server has a vote in the cluster. This might
	// be false if the server is staging and still coming online, or if
	// it's a non-voting server, which will be added in a future release of
	// Nomad.
	Voter bool

	// RaftProtocol is the version of the Raft protocol spoken by this server.
	RaftProtocol string
}

// RaftConfigurationResponse is returned when querying for the current Raft
// configuration.
type RaftConfigurationResponse struct {
	// Servers has the list of servers in the Raft configuration.
	Servers []*RaftServer

	// Index has the Raft index of this configuration.
	Index uint64
}

// RaftPeerByAddressRequest is used by the Operator endpoint to apply a Raft
// operation on a specific Raft peer by address in the form of "IP:port".
type RaftPeerByAddressRequest struct {
	// Address is the peer to remove, in the form "IP:port".
	Address raft.ServerAddress

	// WriteRequest holds the Region for this request.
	WriteRequest
}

// RaftPeerByIDRequest is used by the Operator endpoint to apply a Raft
// operation on a specific Raft peer by ID.
type RaftPeerByIDRequest struct {
	// ID is the peer ID to remove.
	ID raft.ServerID

	// WriteRequest holds the Region for this request.
	WriteRequest
}

// AutopilotSetConfigRequest is used by the Operator endpoint to update the
// current Autopilot configuration of the cluster.
type AutopilotSetConfigRequest struct {
	// Datacenter is the target this request is intended for.
	Datacenter string

	// Config is the new Autopilot configuration to use.
	Config AutopilotConfig

	// CAS controls whether to use check-and-set semantics for this request.
	CAS bool

	// WriteRequest holds the ACL token to go along with this request.
	WriteRequest
}

// RequestDatacenter returns the datacenter for a given request.
func (op *AutopilotSetConfigRequest) RequestDatacenter() string {
	return op.Datacenter
}

// AutopilotConfig is the internal config for the Autopilot mechanism.
type AutopilotConfig struct {
	// CleanupDeadServers controls whether to remove dead servers when a new
	// server is added to the Raft peers.
	CleanupDeadServers bool

	// ServerStabilizationTime is the minimum amount of time a server must be
	// in a stable, healthy state before it can be added to the cluster. Only
	// applicable with Raft protocol version 3 or higher.
	ServerStabilizationTime time.Duration

	// LastContactThreshold is the limit on the amount of time a server can go
	// without leader contact before being considered unhealthy.
	LastContactThreshold time.Duration

	// MaxTrailingLogs is the amount of entries in the Raft Log that a server can
	// be behind before being considered unhealthy.
	MaxTrailingLogs uint64

	// MinQuorum sets the minimum number of servers required in a cluster
	// before autopilot can prune dead servers.
	MinQuorum uint

	// (Enterprise-only) EnableRedundancyZones specifies whether to enable redundancy zones.
	EnableRedundancyZones bool

	// (Enterprise-only) DisableUpgradeMigration will disable Autopilot's upgrade migration
	// strategy of waiting until enough newer-versioned servers have been added to the
	// cluster before promoting them to voters.
	DisableUpgradeMigration bool

	// (Enterprise-only) EnableCustomUpgrades specifies whether to enable using custom
	// upgrade versions when performing migrations.
	EnableCustomUpgrades bool

	// CreateIndex/ModifyIndex store the create/modify indexes of this configuration.
	CreateIndex uint64
	ModifyIndex uint64
}

func (a *AutopilotConfig) Copy() *AutopilotConfig {
	if a == nil {
		return nil
	}

	na := *a
	return &na
}

// SchedulerAlgorithm is an enum string that encapsulates the valid options for a
// SchedulerConfiguration block's SchedulerAlgorithm. These modes will allow the
// scheduler to be user-selectable.
type SchedulerAlgorithm string

const (
	// SchedulerAlgorithmBinpack indicates that the scheduler should spread
	// allocations as evenly as possible over the available hardware.
	SchedulerAlgorithmBinpack SchedulerAlgorithm = "binpack"

	// SchedulerAlgorithmSpread indicates that the scheduler should spread
	// allocations as evenly as possible over the available hardware.
	SchedulerAlgorithmSpread SchedulerAlgorithm = "spread"
)

// SchedulerConfiguration is the config for controlling scheduler behavior
type SchedulerConfiguration struct {
	// SchedulerAlgorithm lets you select between available scheduling algorithms.
	SchedulerAlgorithm SchedulerAlgorithm `hcl:"scheduler_algorithm"`

	// PreemptionConfig specifies whether to enable eviction of lower
	// priority jobs to place higher priority jobs.
	PreemptionConfig PreemptionConfig `hcl:"preemption_config"`

	// MemoryOversubscriptionEnabled specifies whether memory oversubscription is enabled
	MemoryOversubscriptionEnabled bool `hcl:"memory_oversubscription_enabled"`

	// RejectJobRegistration disables new job registrations except with a
	// management ACL token
	RejectJobRegistration bool `hcl:"reject_job_registration"`

	// PauseEvalBroker is a boolean to control whether the evaluation broker
	// should be paused on the cluster leader. Only a single broker runs per
	// region, and it must be persisted to state so the parameter is consistent
	// during leadership transitions.
	PauseEvalBroker bool `hcl:"pause_eval_broker"`

	// CreateIndex/ModifyIndex store the create/modify indexes of this configuration.
	CreateIndex uint64
	ModifyIndex uint64
}

func (s *SchedulerConfiguration) Copy() *SchedulerConfiguration {
	if s == nil {
		return s
	}

	ns := *s
	return &ns
}

func (s *SchedulerConfiguration) EffectiveSchedulerAlgorithm() SchedulerAlgorithm {
	if s == nil || s.SchedulerAlgorithm == "" {
		return SchedulerAlgorithmBinpack
	}

	return s.SchedulerAlgorithm
}

// WithNodePool returns a new SchedulerConfiguration with the node pool
// scheduler configuration applied.
func (s *SchedulerConfiguration) WithNodePool(pool *NodePool) *SchedulerConfiguration {
	schedConfig := s.Copy()

	if pool == nil || pool.SchedulerConfiguration == nil {
		return schedConfig
	}

	poolConfig := pool.SchedulerConfiguration
	if poolConfig.SchedulerAlgorithm != "" {
		schedConfig.SchedulerAlgorithm = poolConfig.SchedulerAlgorithm
	}
	if poolConfig.MemoryOversubscriptionEnabled != nil {
		schedConfig.MemoryOversubscriptionEnabled = *poolConfig.MemoryOversubscriptionEnabled
	}

	return schedConfig
}

func (s *SchedulerConfiguration) Canonicalize() {
	if s != nil && s.SchedulerAlgorithm == "" {
		s.SchedulerAlgorithm = SchedulerAlgorithmBinpack
	}
}

func (s *SchedulerConfiguration) Validate() error {
	if s == nil {
		return nil
	}

	switch s.SchedulerAlgorithm {
	case "", SchedulerAlgorithmBinpack, SchedulerAlgorithmSpread:
	default:
		return fmt.Errorf("invalid scheduler algorithm: %v", s.SchedulerAlgorithm)
	}

	return nil
}

// SchedulerConfigurationResponse is the response object that wraps SchedulerConfiguration
type SchedulerConfigurationResponse struct {
	// SchedulerConfig contains scheduler config options
	SchedulerConfig *SchedulerConfiguration

	QueryMeta
}

// SchedulerSetConfigurationResponse is the response object used
// when updating scheduler configuration
type SchedulerSetConfigurationResponse struct {
	// Updated returns whether the config was actually updated
	// Only set when the request uses CAS
	Updated bool

	WriteMeta
}

// PreemptionConfig specifies whether preemption is enabled based on scheduler type
type PreemptionConfig struct {
	// SystemSchedulerEnabled specifies if preemption is enabled for system jobs
	SystemSchedulerEnabled bool `hcl:"system_scheduler_enabled"`

	// SysBatchSchedulerEnabled specifies if preemption is enabled for sysbatch jobs
	SysBatchSchedulerEnabled bool `hcl:"sysbatch_scheduler_enabled"`

	// BatchSchedulerEnabled specifies if preemption is enabled for batch jobs
	BatchSchedulerEnabled bool `hcl:"batch_scheduler_enabled"`

	// ServiceSchedulerEnabled specifies if preemption is enabled for service jobs
	ServiceSchedulerEnabled bool `hcl:"service_scheduler_enabled"`
}

// SchedulerSetConfigRequest is used by the Operator endpoint to update the
// current Scheduler configuration of the cluster.
type SchedulerSetConfigRequest struct {
	// Config is the new Scheduler configuration to use.
	Config SchedulerConfiguration

	// CAS controls whether to use check-and-set semantics for this request.
	CAS bool

	// WriteRequest holds the ACL token to go along with this request.
	WriteRequest
}

// SnapshotSaveRequest is used by the Operator endpoint to get a Raft snapshot
type SnapshotSaveRequest struct {
	QueryOptions
}

// SnapshotSaveResponse is the header for the streaming snapshot endpoint,
// and followed by the snapshot file content.
type SnapshotSaveResponse struct {

	// SnapshotChecksum returns the checksum of snapshot file in the format
	// `<algo>=<base64>` (e.g. `sha-256=...`)
	SnapshotChecksum string

	// ErrorCode is an http error code if an error is found, e.g. 403 for permission errors
	ErrorCode int `codec:",omitempty"`

	// ErrorMsg is the error message if an error is found, e.g. "Permission Denied"
	ErrorMsg string `codec:",omitempty"`

	QueryMeta
}

type SnapshotRestoreRequest struct {
	WriteRequest
}

type SnapshotRestoreResponse struct {
	ErrorCode int    `codec:",omitempty"`
	ErrorMsg  string `codec:",omitempty"`

	QueryMeta
}
