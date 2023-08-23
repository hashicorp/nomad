// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"io"
	"net"
	"os"
	"runtime"
	"slices"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/deploymentwatcher"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/scheduler"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

const (
	DefaultRegion   = "global"
	DefaultDC       = "dc1"
	DefaultSerfPort = 4648
)

func DefaultRPCAddr() *net.TCPAddr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 4647}
}

// Config is used to parameterize the server
type Config struct {
	// BootstrapExpect mode is used to automatically bring up a
	// collection of Nomad servers. This can be used to automatically
	// bring up a collection of nodes.
	//
	// The BootstrapExpect can be of any of the following values:
	//  1: Server will form a single node cluster and become a leader immediately
	//  N, larger than 1: Server will wait until it's connected to N servers
	//      before attempting leadership and forming the cluster.  No Raft Log operation
	//      will succeed until then.
	//  0: Server will wait to get a Raft configuration from another node and may not
	//      attempt to form a cluster or establish leadership on its own.
	BootstrapExpect int

	// DataDir is the directory to store our state in
	DataDir string

	// DevMode is used for development purposes only and limits the
	// use of persistence or state.
	DevMode bool

	// EnableDebug is used to enable debugging RPC endpoints
	// in the absence of ACLs
	EnableDebug bool

	// EnableEventBroker is used to enable or disable state store
	// event publishing
	EnableEventBroker bool

	// EventBufferSize is the amount of events to hold in memory.
	EventBufferSize int64

	// JobMaxSourceSize limits the maximum size of a jobs source hcl/json
	// before being discarded automatically. A value of zero indicates no job
	// sources will be stored.
	JobMaxSourceSize int

	// LogOutput is the location to write logs to. If this is not set,
	// logs will go to stderr.
	LogOutput io.Writer

	// Logger is the logger used by the server.
	Logger log.InterceptLogger

	// RPCAddr is the RPC address used by Nomad. This should be reachable
	// by the other servers and clients
	RPCAddr *net.TCPAddr

	// ClientRPCAdvertise is the address that is advertised to client nodes for
	// the RPC endpoint. This can differ from the RPC address, if for example
	// the RPCAddr is unspecified "0.0.0.0:4646", but this address must be
	// reachable
	ClientRPCAdvertise *net.TCPAddr

	// ServerRPCAdvertise is the address that is advertised to other servers for
	// the RPC endpoint. This can differ from the RPC address, if for example
	// the RPCAddr is unspecified "0.0.0.0:4646", but this address must be
	// reachable
	ServerRPCAdvertise *net.TCPAddr

	// RaftConfig is the configuration used for Raft in the local DC
	RaftConfig *raft.Config

	// RaftTimeout is applied to any network traffic for raft. Defaults to 10s.
	RaftTimeout time.Duration

	// (Enterprise-only) NonVoter is used to prevent this server from being added
	// as a voting member of the Raft cluster.
	NonVoter bool

	// (Enterprise-only) RedundancyZone is the redundancy zone to use for this server.
	RedundancyZone string

	// (Enterprise-only) UpgradeVersion is the custom upgrade version to use when
	// performing upgrade migrations.
	UpgradeVersion string

	// SerfConfig is the configuration for the serf cluster
	SerfConfig *serf.Config

	// Node name is the name we use to advertise. Defaults to hostname.
	NodeName string

	// NodeID is the uuid of this server.
	NodeID string

	// Region is the region this Nomad server belongs to.
	Region string

	// AuthoritativeRegion is the region which is treated as the authoritative source
	// for ACLs and Policies. This provides a single source of truth to resolve conflicts.
	AuthoritativeRegion string

	// Datacenter is the datacenter this Nomad server belongs to.
	Datacenter string

	// Build is a string that is gossiped around, and can be used to help
	// operators track which versions are actively deployed
	Build string

	// Revision is a string that carries the version.GitCommit of Nomad that
	// was compiled.
	Revision string

	// NumSchedulers is the number of scheduler thread that are run.
	// This can be as many as one per core, or zero to disable this server
	// from doing any scheduling work.
	NumSchedulers int

	// EnabledSchedulers controls the set of sub-schedulers that are
	// enabled for this server to handle. This will restrict the evaluations
	// that the workers dequeue for processing.
	EnabledSchedulers []string

	// ReconcileInterval controls how often we reconcile the strongly
	// consistent store with the Serf info. This is used to handle nodes
	// that are force removed, as well as intermittent unavailability during
	// leader election.
	ReconcileInterval time.Duration

	// EvalGCInterval is how often we dispatch a job to GC evaluations
	EvalGCInterval time.Duration

	// EvalGCThreshold is how "old" an evaluation must be to be eligible
	// for GC. This gives users some time to debug a failed evaluation.
	//
	// Please note that the rules for GC of evaluations which belong to a batch
	// job are separate and controlled by `BatchEvalGCThreshold`
	EvalGCThreshold time.Duration

	// BatchEvalGCThreshold is how "old" an evaluation must be to be eligible
	// for GC if the eval belongs to a batch job.
	BatchEvalGCThreshold time.Duration

	// JobGCInterval is how often we dispatch a job to GC jobs that are
	// available for garbage collection.
	JobGCInterval time.Duration

	// JobGCThreshold is how old a job must be before it eligible for GC. This gives
	// the user time to inspect the job.
	JobGCThreshold time.Duration

	// NodeGCInterval is how often we dispatch a job to GC failed nodes.
	NodeGCInterval time.Duration

	// NodeGCThreshold is how "old" a node must be to be eligible
	// for GC. This gives users some time to view and debug a failed nodes.
	NodeGCThreshold time.Duration

	// DeploymentGCInterval is how often we dispatch a job to GC terminal
	// deployments.
	DeploymentGCInterval time.Duration

	// DeploymentGCThreshold is how "old" a deployment must be to be eligible
	// for GC. This gives users some time to view terminal deployments.
	DeploymentGCThreshold time.Duration

	// CSIPluginGCInterval is how often we dispatch a job to GC unused plugins.
	CSIPluginGCInterval time.Duration

	// CSIPluginGCThreshold is how "old" a plugin must be to be eligible
	// for GC. This gives users some time to debug plugins.
	CSIPluginGCThreshold time.Duration

	// CSIVolumeClaimGCInterval is how often we dispatch a job to GC
	// volume claims.
	CSIVolumeClaimGCInterval time.Duration

	// CSIVolumeClaimGCThreshold is how "old" a volume must be to be
	// eligible for GC. This gives users some time to debug volumes.
	CSIVolumeClaimGCThreshold time.Duration

	// OneTimeTokenGCInterval is how often we dispatch a job to GC
	// one-time tokens.
	OneTimeTokenGCInterval time.Duration

	// ACLTokenExpirationGCInterval is how often we dispatch a job to GC
	// expired ACL tokens.
	ACLTokenExpirationGCInterval time.Duration

	// ACLTokenExpirationGCThreshold controls how "old" an expired ACL token
	// must be to be collected by GC.
	ACLTokenExpirationGCThreshold time.Duration

	// RootKeyGCInterval is how often we dispatch a job to GC
	// encryption key metadata
	RootKeyGCInterval time.Duration

	// RootKeyGCThreshold is how "old" encryption key metadata must be
	// to be eligible for GC.
	RootKeyGCThreshold time.Duration

	// RootKeyRotationThreshold is how "old" an active key can be
	// before it's rotated
	RootKeyRotationThreshold time.Duration

	// VariablesRekeyInterval is how often we dispatch a job to
	// rekey any variables associated with a key in the Rekeying state
	VariablesRekeyInterval time.Duration

	// EvalNackTimeout controls how long we allow a sub-scheduler to
	// work on an evaluation before we consider it failed and Nack it.
	// This allows that evaluation to be handed to another sub-scheduler
	// to work on. Defaults to 60 seconds. This should be long enough that
	// no evaluation hits it unless the sub-scheduler has failed.
	EvalNackTimeout time.Duration

	// EvalDeliveryLimit is the limit of attempts we make to deliver and
	// process an evaluation. This is used so that an eval that will never
	// complete eventually fails out of the system.
	EvalDeliveryLimit int

	// EvalNackInitialReenqueueDelay is the delay applied before reenqueuing a
	// Nacked evaluation for the first time. This value should be small as the
	// initial Nack can be due to a down machine and the eval should be retried
	// quickly for liveliness.
	EvalNackInitialReenqueueDelay time.Duration

	// EvalNackSubsequentReenqueueDelay is the delay applied before reenqueuing
	// an evaluation that has been Nacked more than once. This delay is
	// compounding after the first Nack. This value should be significantly
	// longer than the initial delay as the purpose it severs is to apply
	// back-pressure as evaluations are being Nacked either due to scheduler
	// failures or because they are hitting their Nack timeout, both of which
	// are signs of high server resource usage.
	EvalNackSubsequentReenqueueDelay time.Duration

	// EvalFailedFollowupBaselineDelay is the minimum time waited before
	// retrying a failed evaluation.
	EvalFailedFollowupBaselineDelay time.Duration

	// EvalReapCancelableInterval is the interval for the periodic reaping of
	// cancelable evaluations. Cancelable evaluations are canceled whenever any
	// eval is ack'd but this sweeps up on quiescent clusters. This config value
	// exists only for testing.
	EvalReapCancelableInterval time.Duration

	// EvalFailedFollowupDelayRange defines the range of additional time from
	// the baseline in which to wait before retrying a failed evaluation. The
	// additional delay is selected from this range randomly.
	EvalFailedFollowupDelayRange time.Duration

	// NodePlanRejectionEnabled controls if node rejection tracker is enabled.
	NodePlanRejectionEnabled bool

	// NodePlanRejectionThreshold is the number of times a node must have a
	// plan rejection before it is set as ineligible.
	NodePlanRejectionThreshold int

	// NodePlanRejectionWindow is the time window used to track plan
	// rejections for nodes.
	NodePlanRejectionWindow time.Duration

	// MinHeartbeatTTL is the minimum time between heartbeats.
	// This is used as a floor to prevent excessive updates.
	MinHeartbeatTTL time.Duration

	// MaxHeartbeatsPerSecond is the maximum target rate of heartbeats
	// being processed per second. This allows the TTL to be increased
	// to meet the target rate.
	MaxHeartbeatsPerSecond float64

	// HeartbeatGrace is the additional time given as a grace period
	// beyond the TTL to account for network and processing delays
	// as well as clock skew.
	HeartbeatGrace time.Duration

	// FailoverHeartbeatTTL is the TTL applied to heartbeats after
	// a new leader is elected, since we no longer know the status
	// of all the heartbeats.
	FailoverHeartbeatTTL time.Duration

	// ConsulConfig is this Agent's Consul configuration
	ConsulConfig *config.ConsulConfig

	// ConsulConfigs is a map of Consul configurations, here to support features
	// in Nomad Enterprise. The default Consul config pointer above will be
	// found in this map under the name "default"
	ConsulConfigs map[string]*config.ConsulConfig

	// VaultConfig is this Agent's default Vault configuration
	VaultConfig *config.VaultConfig

	// VaultConfigs is a map of Vault configurations, here to support features
	// in Nomad Enterprise. The default Vault config pointer above will be found
	// in this map under the name "default"
	VaultConfigs map[string]*config.VaultConfig

	// RPCHoldTimeout is how long an RPC can be "held" before it is errored.
	// This is used to paper over a loss of leadership by instead holding RPCs,
	// so that the caller experiences a slow response rather than an error.
	// This period is meant to be long enough for a leader election to take
	// place, and a small jitter is applied to avoid a thundering herd.
	RPCHoldTimeout time.Duration

	// TLSConfig holds various TLS related configurations
	TLSConfig *config.TLSConfig

	// ACLEnabled controls if ACL enforcement and management is enabled.
	ACLEnabled bool

	// ReplicationBackoff is how much we backoff when replication errors.
	// This is a tunable knob for testing primarily.
	ReplicationBackoff time.Duration

	// ReplicationToken is the ACL Token Secret ID used to fetch from
	// the Authoritative Region.
	ReplicationToken string

	// TokenMinExpirationTTL is used to enforce the lowest acceptable value for
	// ACL token expiration.
	ACLTokenMinExpirationTTL time.Duration

	// TokenMaxExpirationTTL is used to enforce the highest acceptable value
	// for ACL token expiration.
	ACLTokenMaxExpirationTTL time.Duration

	// SentinelGCInterval is the interval that we GC unused policies.
	SentinelGCInterval time.Duration

	// SentinelConfig is this Agent's Sentinel configuration
	SentinelConfig *config.SentinelConfig

	// StatsCollectionInterval is the interval at which the Nomad server
	// publishes metrics which are periodic in nature like updating gauges
	StatsCollectionInterval time.Duration

	// DisableDispatchedJobSummaryMetrics allows for ignore dispatched jobs when
	// publishing Job summary metrics
	DisableDispatchedJobSummaryMetrics bool

	// DisableRPCRateMetricsLabels drops the label for the identity of the
	// requester when publishing metrics on RPC rate on the server. This may be
	// useful to control metrics collection costs in environments where request
	// rate is well-controlled but cardinality of requesters is high.
	DisableRPCRateMetricsLabels bool

	// AutopilotConfig is used to apply the initial autopilot config when
	// bootstrapping.
	AutopilotConfig *structs.AutopilotConfig

	// ServerHealthInterval is the frequency with which the health of the
	// servers in the cluster will be updated.
	ServerHealthInterval time.Duration

	// AutopilotInterval is the frequency with which the leader will perform
	// autopilot tasks, such as promoting eligible non-voters and removing
	// dead servers.
	AutopilotInterval time.Duration

	// DefaultSchedulerConfig configures the initial scheduler config to be persisted in Raft.
	// Once the cluster is bootstrapped, and Raft persists the config (from here or through API)
	// and this value is ignored.
	DefaultSchedulerConfig structs.SchedulerConfiguration `hcl:"default_scheduler_config"`

	// RPCHandshakeTimeout is the deadline by which RPC handshakes must
	// complete. The RPC handshake includes the first byte read as well as
	// the TLS handshake and subsequent byte read if TLS is enabled.
	//
	// The deadline is reset after the first byte is read so when TLS is
	// enabled RPC connections may take (timeout * 2) to complete.
	//
	// 0 means no timeout.
	RPCHandshakeTimeout time.Duration

	// RPCMaxConnsPerClient is the maximum number of concurrent RPC
	// connections from a single IP address. nil/0 means no limit.
	RPCMaxConnsPerClient int

	// LicenseConfig stores information about the Enterprise license loaded for the server.
	LicenseConfig *LicenseConfig

	// SearchConfig provides knobs for Search API.
	SearchConfig *structs.SearchConfig

	// RaftBoltNoFreelistSync configures whether freelist syncing is enabled.
	RaftBoltNoFreelistSync bool

	// AgentShutdown is used to call agent.Shutdown from the context of a Server
	// It is used primarily for licensing
	AgentShutdown func() error

	// DeploymentQueryRateLimit is in queries per second and is used by the
	// DeploymentWatcher to throttle the amount of simultaneously deployments
	DeploymentQueryRateLimit float64

	// JobDefaultPriority is the default Job priority if not specified.
	JobDefaultPriority int

	// JobMaxPriority is an upper bound on the Job priority.
	JobMaxPriority int

	// JobTrackedVersions is the number of historic Job versions that are kept.
	JobTrackedVersions int
}

func (c *Config) Copy() *Config {
	if c == nil {
		return nil
	}

	nc := *c

	// Can't deep copy interfaces
	// LogOutput io.Writer
	// Logger log.InterceptLogger
	// PluginLoader loader.PluginCatalog
	// PluginSingletonLoader loader.PluginCatalog

	nc.RPCAddr = pointer.Copy(c.RPCAddr)
	nc.ClientRPCAdvertise = pointer.Copy(c.ClientRPCAdvertise)
	nc.ServerRPCAdvertise = pointer.Copy(c.ServerRPCAdvertise)
	nc.RaftConfig = pointer.Copy(c.RaftConfig)
	nc.SerfConfig = pointer.Copy(c.SerfConfig)
	nc.EnabledSchedulers = slices.Clone(c.EnabledSchedulers)
	nc.ConsulConfig = c.ConsulConfig.Copy()
	nc.ConsulConfigs = helper.DeepCopyMap(c.ConsulConfigs)
	nc.VaultConfig = c.VaultConfig.Copy()
	nc.VaultConfigs = helper.DeepCopyMap(c.VaultConfigs)
	nc.TLSConfig = c.TLSConfig.Copy()
	nc.SentinelConfig = c.SentinelConfig.Copy()
	nc.AutopilotConfig = c.AutopilotConfig.Copy()
	nc.LicenseConfig = c.LicenseConfig.Copy()
	nc.SearchConfig = c.SearchConfig.Copy()

	return &nc
}

// DefaultConfig returns the default configuration. Only used as the basis for
// merging agent or test parameters.
func DefaultConfig() *Config {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	c := &Config{
		Region:                           DefaultRegion,
		AuthoritativeRegion:              DefaultRegion,
		Datacenter:                       DefaultDC,
		NodeName:                         hostname,
		NodeID:                           uuid.Generate(),
		RaftConfig:                       raft.DefaultConfig(),
		RaftTimeout:                      10 * time.Second,
		LogOutput:                        os.Stderr,
		RPCAddr:                          DefaultRPCAddr(),
		SerfConfig:                       serf.DefaultConfig(),
		NumSchedulers:                    1,
		ReconcileInterval:                60 * time.Second,
		EvalGCInterval:                   5 * time.Minute,
		EvalGCThreshold:                  1 * time.Hour,
		BatchEvalGCThreshold:             24 * time.Hour,
		JobGCInterval:                    5 * time.Minute,
		JobGCThreshold:                   4 * time.Hour,
		NodeGCInterval:                   5 * time.Minute,
		NodeGCThreshold:                  24 * time.Hour,
		DeploymentGCInterval:             5 * time.Minute,
		DeploymentGCThreshold:            1 * time.Hour,
		CSIPluginGCInterval:              5 * time.Minute,
		CSIPluginGCThreshold:             1 * time.Hour,
		CSIVolumeClaimGCInterval:         5 * time.Minute,
		CSIVolumeClaimGCThreshold:        5 * time.Minute,
		OneTimeTokenGCInterval:           10 * time.Minute,
		ACLTokenExpirationGCInterval:     5 * time.Minute,
		ACLTokenExpirationGCThreshold:    1 * time.Hour,
		RootKeyGCInterval:                10 * time.Minute,
		RootKeyGCThreshold:               1 * time.Hour,
		RootKeyRotationThreshold:         720 * time.Hour, // 30 days
		VariablesRekeyInterval:           10 * time.Minute,
		EvalNackTimeout:                  60 * time.Second,
		EvalDeliveryLimit:                3,
		EvalNackInitialReenqueueDelay:    1 * time.Second,
		EvalNackSubsequentReenqueueDelay: 20 * time.Second,
		EvalFailedFollowupBaselineDelay:  1 * time.Minute,
		EvalFailedFollowupDelayRange:     5 * time.Minute,
		EvalReapCancelableInterval:       5 * time.Second,
		MinHeartbeatTTL:                  10 * time.Second,
		MaxHeartbeatsPerSecond:           50.0,
		HeartbeatGrace:                   10 * time.Second,
		FailoverHeartbeatTTL:             300 * time.Second,
		NodePlanRejectionEnabled:         false,
		NodePlanRejectionThreshold:       15,
		NodePlanRejectionWindow:          10 * time.Minute,
		ConsulConfig:                     config.DefaultConsulConfig(),
		VaultConfig:                      config.DefaultVaultConfig(),
		RPCHoldTimeout:                   5 * time.Second,
		StatsCollectionInterval:          1 * time.Minute,
		TLSConfig:                        &config.TLSConfig{},
		ReplicationBackoff:               30 * time.Second,
		SentinelGCInterval:               30 * time.Second,
		LicenseConfig:                    &LicenseConfig{},
		EnableEventBroker:                true,
		EventBufferSize:                  100,
		ACLTokenMinExpirationTTL:         1 * time.Minute,
		ACLTokenMaxExpirationTTL:         24 * time.Hour,
		AutopilotConfig: &structs.AutopilotConfig{
			CleanupDeadServers:      true,
			LastContactThreshold:    200 * time.Millisecond,
			MaxTrailingLogs:         250,
			ServerStabilizationTime: 10 * time.Second,
		},
		ServerHealthInterval: 2 * time.Second,
		AutopilotInterval:    10 * time.Second,
		DefaultSchedulerConfig: structs.SchedulerConfiguration{
			SchedulerAlgorithm: structs.SchedulerAlgorithmBinpack,
			PreemptionConfig: structs.PreemptionConfig{
				SystemSchedulerEnabled:   true,
				SysBatchSchedulerEnabled: false,
				BatchSchedulerEnabled:    false,
				ServiceSchedulerEnabled:  false,
			},
		},
		DeploymentQueryRateLimit: deploymentwatcher.LimitStateQueriesPerSecond,
		JobDefaultPriority:       structs.JobDefaultPriority,
		JobMaxPriority:           structs.JobDefaultMaxPriority,
		JobTrackedVersions:       structs.JobDefaultTrackedVersions,
	}

	c.ConsulConfigs = map[string]*config.ConsulConfig{"default": c.ConsulConfig}
	c.VaultConfigs = map[string]*config.VaultConfig{"default": c.VaultConfig}

	// Enable all known schedulers by default
	c.EnabledSchedulers = make([]string, 0, len(scheduler.BuiltinSchedulers))
	for name := range scheduler.BuiltinSchedulers {
		c.EnabledSchedulers = append(c.EnabledSchedulers, name)
	}
	c.EnabledSchedulers = append(c.EnabledSchedulers, structs.JobTypeCore)

	// Default the number of schedulers to match the cores
	c.NumSchedulers = runtime.NumCPU()

	// Increase our reap interval to 3 days instead of 24h.
	c.SerfConfig.ReconnectTimeout = 3 * 24 * time.Hour

	// Serf should use the WAN timing, since we are using it
	// to communicate between DC's
	c.SerfConfig.MemberlistConfig = memberlist.DefaultWANConfig()
	c.SerfConfig.MemberlistConfig.BindPort = DefaultSerfPort

	// Disable shutdown on removal
	c.RaftConfig.ShutdownOnRemove = false

	// Default to Raft v3 since Nomad 1.3
	c.RaftConfig.ProtocolVersion = 3

	return c
}
