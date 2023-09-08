// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"context"
	"fmt"
	"io"
	golog "log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/dustin/go-humanize"
	consulapi "github.com/hashicorp/consul/api"
	log "github.com/hashicorp/go-hclog"
	uuidparse "github.com/hashicorp/go-uuid"
	"github.com/hashicorp/nomad/client"
	clientconfig "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/command/agent/event"
	"github.com/hashicorp/nomad/helper/bufconndialer"
	"github.com/hashicorp/nomad/helper/escapingfs"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/lib/cpuset"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/deploymentwatcher"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/raft"
)

const (
	agentHttpCheckInterval  = 10 * time.Second
	agentHttpCheckTimeout   = 5 * time.Second
	serverRpcCheckInterval  = 10 * time.Second
	serverRpcCheckTimeout   = 3 * time.Second
	serverSerfCheckInterval = 10 * time.Second
	serverSerfCheckTimeout  = 3 * time.Second

	// roles used in identifying Consul entries for Nomad agents
	consulRoleServer = "server"
	consulRoleClient = "client"

	// DefaultRaftMultiplier is used as a baseline Raft configuration that
	// will be reliable on a very basic server.
	DefaultRaftMultiplier = 1

	// MaxRaftMultiplier is a fairly arbitrary upper bound that limits the
	// amount of performance detuning that's possible.
	MaxRaftMultiplier = 10
)

// Agent is a long running daemon that is used to run both
// clients and servers. Servers are responsible for managing
// state and making scheduling decisions. Clients can be
// scheduled to, and are responsible for interfacing with
// servers to run allocations.
type Agent struct {
	config     *Config
	configLock sync.Mutex

	logger     log.InterceptLogger
	auditor    event.Auditor
	httpLogger log.Logger
	logOutput  io.Writer

	// EnterpriseAgent holds information and methods for enterprise functionality
	EnterpriseAgent *EnterpriseAgent

	// consulService is Nomad's custom Consul client for managing services
	// and checks.
	consulService *consul.ServiceClient

	// consulProxies is the subset of Consul's Agent API Nomad uses.
	consulProxies *consul.ConnectProxies

	// consulCatalog is the subset of Consul's Catalog API Nomad uses.
	consulCatalog consul.CatalogAPI

	// consulConfigEntries is the subset of Consul's Configuration Entries API Nomad uses.
	consulConfigEntries consul.ConfigAPI

	// consulACLs is Nomad's subset of Consul's ACL API Nomad uses.
	consulACLs consul.ACLsAPI

	// client is the launched Nomad Client. Can be nil if the agent isn't
	// configured to run a client.
	client *client.Client

	// server is the launched Nomad Server. Can be nil if the agent isn't
	// configured to run a server.
	server *nomad.Server

	// pluginLoader is used to load plugins
	pluginLoader loader.PluginCatalog

	// pluginSingletonLoader is a plugin loader that will returns singleton
	// instances of the plugins.
	pluginSingletonLoader loader.PluginCatalog

	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex

	// builtinDialer dials the builtinListener. It is used for connecting
	// consul-template to the HTTP API in process. In the event this agent is
	// not running in client mode, these two fields will be nil.
	builtinListener net.Listener
	builtinDialer   *bufconndialer.BufConnWrapper

	// taskAPIServer is an HTTP server for attaching per-task listeners. Always
	// requires auth.
	taskAPIServer *builtinAPI

	inmemSink *metrics.InmemSink
}

// NewAgent is used to create a new agent with the given configuration
func NewAgent(config *Config, logger log.InterceptLogger, logOutput io.Writer, inmem *metrics.InmemSink) (*Agent, error) {
	a := &Agent{
		config:     config,
		logOutput:  logOutput,
		shutdownCh: make(chan struct{}),
		inmemSink:  inmem,
	}

	// Create the loggers
	a.logger = logger
	a.httpLogger = a.logger.ResetNamed("http")

	// Global logger should match internal logger as much as possible
	golog.SetFlags(golog.LstdFlags | golog.Lmicroseconds)

	if err := a.setupConsul(config.Consul); err != nil {
		return nil, fmt.Errorf("Failed to initialize Consul client: %v", err)
	}

	if err := a.setupServer(); err != nil {
		return nil, err
	}
	if err := a.setupClient(); err != nil {
		return nil, err
	}

	if err := a.setupEnterpriseAgent(logger); err != nil {
		return nil, err
	}
	if a.client == nil && a.server == nil {
		return nil, fmt.Errorf("must have at least client or server mode enabled")
	}

	return a, nil
}

// convertServerConfig takes an agent config and log output and returns a Nomad
// Config. There may be missing fields that must be set by the agent. To do this
// call finalizeServerConfig.
func convertServerConfig(agentConfig *Config) (*nomad.Config, error) {
	conf := agentConfig.NomadConfig
	if conf == nil {
		conf = nomad.DefaultConfig()
	}
	conf.DevMode = agentConfig.DevMode
	conf.EnableDebug = agentConfig.EnableDebug

	conf.Build = agentConfig.Version.VersionNumber()
	conf.Revision = agentConfig.Version.Revision
	if agentConfig.Region != "" {
		conf.Region = agentConfig.Region
	}

	// Set the Authoritative Region if set, otherwise default to
	// the same as the local region.
	if agentConfig.Server.AuthoritativeRegion != "" {
		conf.AuthoritativeRegion = agentConfig.Server.AuthoritativeRegion
	} else if agentConfig.Region != "" {
		conf.AuthoritativeRegion = agentConfig.Region
	}

	if agentConfig.Datacenter != "" {
		conf.Datacenter = agentConfig.Datacenter
	}
	if agentConfig.NodeName != "" {
		conf.NodeName = agentConfig.NodeName
	}
	if agentConfig.Server.BootstrapExpect > 0 {
		conf.BootstrapExpect = agentConfig.Server.BootstrapExpect
	}
	if agentConfig.DataDir != "" {
		conf.DataDir = filepath.Join(agentConfig.DataDir, "server")
	}
	if agentConfig.Server.DataDir != "" {
		conf.DataDir = agentConfig.Server.DataDir
	}
	if agentConfig.Server.RaftProtocol != 0 {
		conf.RaftConfig.ProtocolVersion = raft.ProtocolVersion(agentConfig.Server.RaftProtocol)
	}
	if v := conf.RaftConfig.ProtocolVersion; v != 3 {
		return nil, fmt.Errorf("raft_protocol must be 3 in Nomad v1.4 and later, got %d", v)
	}
	raftMultiplier := int(DefaultRaftMultiplier)
	if agentConfig.Server.RaftMultiplier != nil && *agentConfig.Server.RaftMultiplier != 0 {
		raftMultiplier = *agentConfig.Server.RaftMultiplier
		if raftMultiplier < 1 || raftMultiplier > MaxRaftMultiplier {
			return nil, fmt.Errorf("raft_multiplier cannot be %d. Must be between 1 and %d", *agentConfig.Server.RaftMultiplier, MaxRaftMultiplier)
		}
	}

	if vPtr := agentConfig.Server.RaftTrailingLogs; vPtr != nil {
		if *vPtr < 1 {
			return nil, fmt.Errorf("raft_trailing_logs must be non-negative, got %d", *vPtr)
		}
		conf.RaftConfig.TrailingLogs = uint64(*vPtr)
	}

	if vPtr := agentConfig.Server.RaftSnapshotInterval; vPtr != nil {
		dur, err := time.ParseDuration(*vPtr)
		if err != nil {
			return nil, err
		}
		if dur < 5*time.Millisecond {
			return nil, fmt.Errorf("raft_snapshot_interval must be greater than 5ms, got %q", *vPtr)
		}
		conf.RaftConfig.SnapshotInterval = dur
	}

	if vPtr := agentConfig.Server.RaftSnapshotThreshold; vPtr != nil {
		if *vPtr < 1 {
			return nil, fmt.Errorf("raft_snapshot_threshold must be non-negative, got %d", *vPtr)
		}
		conf.RaftConfig.SnapshotThreshold = uint64(*vPtr)
	}

	conf.RaftConfig.ElectionTimeout *= time.Duration(raftMultiplier)
	conf.RaftConfig.HeartbeatTimeout *= time.Duration(raftMultiplier)
	conf.RaftConfig.LeaderLeaseTimeout *= time.Duration(raftMultiplier)
	conf.RaftConfig.CommitTimeout *= time.Duration(raftMultiplier)

	if agentConfig.Server.NumSchedulers != nil {
		conf.NumSchedulers = *agentConfig.Server.NumSchedulers
	}
	if len(agentConfig.Server.EnabledSchedulers) != 0 {
		// Convert to a set and require the core scheduler
		set := make(map[string]struct{}, 4)
		set[structs.JobTypeCore] = struct{}{}
		for _, sched := range agentConfig.Server.EnabledSchedulers {
			set[sched] = struct{}{}
		}

		schedulers := make([]string, 0, len(set))
		for k := range set {
			schedulers = append(schedulers, k)
		}

		conf.EnabledSchedulers = schedulers

	}
	if agentConfig.ACL.Enabled {
		conf.ACLEnabled = true
	}
	if agentConfig.ACL.ReplicationToken != "" {
		conf.ReplicationToken = agentConfig.ACL.ReplicationToken
	}
	if agentConfig.ACL.TokenMinExpirationTTL != 0 {
		conf.ACLTokenMinExpirationTTL = agentConfig.ACL.TokenMinExpirationTTL
	}
	if agentConfig.ACL.TokenMaxExpirationTTL != 0 {
		conf.ACLTokenMaxExpirationTTL = agentConfig.ACL.TokenMaxExpirationTTL
	}
	if agentConfig.Sentinel != nil {
		conf.SentinelConfig = agentConfig.Sentinel
	}
	if agentConfig.Server.NonVotingServer {
		conf.NonVoter = true
	}
	if agentConfig.Server.RedundancyZone != "" {
		conf.RedundancyZone = agentConfig.Server.RedundancyZone
	}
	if agentConfig.Server.UpgradeVersion != "" {
		conf.UpgradeVersion = agentConfig.Server.UpgradeVersion
	}
	if agentConfig.Server.EnableEventBroker != nil {
		conf.EnableEventBroker = *agentConfig.Server.EnableEventBroker
	}
	if agentConfig.Server.EventBufferSize != nil {
		if *agentConfig.Server.EventBufferSize < 0 {
			return nil, fmt.Errorf("Invalid Config, event_buffer_size must be non-negative")
		}
		conf.EventBufferSize = int64(*agentConfig.Server.EventBufferSize)
	}
	if agentConfig.Autopilot != nil {
		if agentConfig.Autopilot.CleanupDeadServers != nil {
			conf.AutopilotConfig.CleanupDeadServers = *agentConfig.Autopilot.CleanupDeadServers
		}
		if agentConfig.Autopilot.ServerStabilizationTime != 0 {
			conf.AutopilotConfig.ServerStabilizationTime = agentConfig.Autopilot.ServerStabilizationTime
		}
		if agentConfig.Autopilot.LastContactThreshold != 0 {
			conf.AutopilotConfig.LastContactThreshold = agentConfig.Autopilot.LastContactThreshold
		}
		if agentConfig.Autopilot.MaxTrailingLogs != 0 {
			conf.AutopilotConfig.MaxTrailingLogs = uint64(agentConfig.Autopilot.MaxTrailingLogs)
		}
		if agentConfig.Autopilot.MinQuorum != 0 {
			conf.AutopilotConfig.MinQuorum = uint(agentConfig.Autopilot.MinQuorum)
		}
		if agentConfig.Autopilot.EnableRedundancyZones != nil {
			conf.AutopilotConfig.EnableRedundancyZones = *agentConfig.Autopilot.EnableRedundancyZones
		}
		if agentConfig.Autopilot.DisableUpgradeMigration != nil {
			conf.AutopilotConfig.DisableUpgradeMigration = *agentConfig.Autopilot.DisableUpgradeMigration
		}
		if agentConfig.Autopilot.EnableCustomUpgrades != nil {
			conf.AutopilotConfig.EnableCustomUpgrades = *agentConfig.Autopilot.EnableCustomUpgrades
		}
	}

	jobMaxPriority := structs.JobDefaultMaxPriority
	if agentConfig.Server.JobMaxPriority != nil && *agentConfig.Server.JobMaxPriority != 0 {
		jobMaxPriority = *agentConfig.Server.JobMaxPriority
		if jobMaxPriority < structs.JobDefaultMaxPriority || jobMaxPriority > structs.JobMaxPriority {
			return nil, fmt.Errorf("job_max_priority cannot be %d. Must be between %d and %d", *agentConfig.Server.JobMaxPriority, structs.JobDefaultMaxPriority, structs.JobMaxPriority)
		}
	}
	jobDefaultPriority := structs.JobDefaultPriority
	if agentConfig.Server.JobDefaultPriority != nil && *agentConfig.Server.JobDefaultPriority != 0 {
		jobDefaultPriority = *agentConfig.Server.JobDefaultPriority
		if jobDefaultPriority < structs.JobDefaultPriority || jobDefaultPriority >= jobMaxPriority {
			return nil, fmt.Errorf("job_default_priority cannot be %d. Must be between %d and %d", *agentConfig.Server.JobDefaultPriority, structs.JobDefaultPriority, jobMaxPriority)
		}
	}
	conf.JobMaxPriority = jobMaxPriority
	conf.JobDefaultPriority = jobDefaultPriority

	if agentConfig.Server.JobTrackedVersions != nil {
		if *agentConfig.Server.JobTrackedVersions <= 0 {
			return nil, fmt.Errorf("job_tracked_versions must be greater than 0")
		}
		conf.JobTrackedVersions = *agentConfig.Server.JobTrackedVersions
	}

	// Set up the bind addresses
	rpcAddr, err := net.ResolveTCPAddr("tcp", agentConfig.normalizedAddrs.RPC)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse RPC address %q: %v", agentConfig.normalizedAddrs.RPC, err)
	}
	serfAddr, err := net.ResolveTCPAddr("tcp", agentConfig.normalizedAddrs.Serf)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse Serf address %q: %v", agentConfig.normalizedAddrs.Serf, err)
	}
	conf.RPCAddr.Port = rpcAddr.Port
	conf.RPCAddr.IP = rpcAddr.IP
	conf.SerfConfig.MemberlistConfig.BindPort = serfAddr.Port
	conf.SerfConfig.MemberlistConfig.BindAddr = serfAddr.IP.String()
	conf.SerfConfig.RejoinAfterLeave = agentConfig.Server.RejoinAfterLeave

	// Set up the advertise addresses
	rpcAddr, err = net.ResolveTCPAddr("tcp", agentConfig.AdvertiseAddrs.RPC)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse RPC advertise address %q: %v", agentConfig.AdvertiseAddrs.RPC, err)
	}
	serfAddr, err = net.ResolveTCPAddr("tcp", agentConfig.AdvertiseAddrs.Serf)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse Serf advertise address %q: %v", agentConfig.AdvertiseAddrs.Serf, err)
	}

	// Server address is the serf advertise address and rpc port. This is the
	// address that all servers should be able to communicate over RPC with.
	serverAddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(serfAddr.IP.String(), fmt.Sprintf("%d", rpcAddr.Port)))
	if err != nil {
		return nil, fmt.Errorf("Failed to resolve Serf advertise address %q: %v", agentConfig.AdvertiseAddrs.Serf, err)
	}

	conf.SerfConfig.MemberlistConfig.AdvertiseAddr = serfAddr.IP.String()
	conf.SerfConfig.MemberlistConfig.AdvertisePort = serfAddr.Port
	conf.ClientRPCAdvertise = rpcAddr
	conf.ServerRPCAdvertise = serverAddr

	// Set up gc threshold and heartbeat grace period
	if gcThreshold := agentConfig.Server.NodeGCThreshold; gcThreshold != "" {
		dur, err := time.ParseDuration(gcThreshold)
		if err != nil {
			return nil, err
		}
		conf.NodeGCThreshold = dur
	}
	if gcInterval := agentConfig.Server.JobGCInterval; gcInterval != "" {
		dur, err := time.ParseDuration(gcInterval)
		if err != nil {
			return nil, fmt.Errorf("failed to parse job_gc_interval: %v", err)
		} else if dur <= time.Duration(0) {
			return nil, fmt.Errorf("job_gc_interval should be greater than 0s")
		}
		conf.JobGCInterval = dur
	}
	if gcThreshold := agentConfig.Server.JobGCThreshold; gcThreshold != "" {
		dur, err := time.ParseDuration(gcThreshold)
		if err != nil {
			return nil, err
		}
		conf.JobGCThreshold = dur
	}
	if gcThreshold := agentConfig.Server.EvalGCThreshold; gcThreshold != "" {
		dur, err := time.ParseDuration(gcThreshold)
		if err != nil {
			return nil, err
		}
		conf.EvalGCThreshold = dur
	}
	if gcThreshold := agentConfig.Server.BatchEvalGCThreshold; gcThreshold != "" {
		dur, err := time.ParseDuration(gcThreshold)
		if err != nil {
			return nil, err
		}
		conf.BatchEvalGCThreshold = dur
	}
	if gcThreshold := agentConfig.Server.DeploymentGCThreshold; gcThreshold != "" {
		dur, err := time.ParseDuration(gcThreshold)
		if err != nil {
			return nil, err
		}
		conf.DeploymentGCThreshold = dur
	}
	if gcInterval := agentConfig.Server.CSIVolumeClaimGCInterval; gcInterval != "" {
		dur, err := time.ParseDuration(gcInterval)
		if err != nil {
			return nil, err
		} else if dur <= time.Duration(0) {
			return nil, fmt.Errorf("csi_volume_claim_gc_interval should be greater than 0s")
		}
		conf.CSIVolumeClaimGCInterval = dur
	}
	if gcThreshold := agentConfig.Server.CSIVolumeClaimGCThreshold; gcThreshold != "" {
		dur, err := time.ParseDuration(gcThreshold)
		if err != nil {
			return nil, err
		}
		conf.CSIVolumeClaimGCThreshold = dur
	}
	if gcThreshold := agentConfig.Server.CSIPluginGCThreshold; gcThreshold != "" {
		dur, err := time.ParseDuration(gcThreshold)
		if err != nil {
			return nil, err
		}
		conf.CSIPluginGCThreshold = dur
	}
	if gcThreshold := agentConfig.Server.ACLTokenGCThreshold; gcThreshold != "" {
		dur, err := time.ParseDuration(gcThreshold)
		if err != nil {
			return nil, err
		}
		conf.ACLTokenExpirationGCThreshold = dur
	}
	if gcThreshold := agentConfig.Server.RootKeyGCThreshold; gcThreshold != "" {
		dur, err := time.ParseDuration(gcThreshold)
		if err != nil {
			return nil, err
		}
		conf.RootKeyGCThreshold = dur
	}
	if gcInterval := agentConfig.Server.RootKeyGCInterval; gcInterval != "" {
		dur, err := time.ParseDuration(gcInterval)
		if err != nil {
			return nil, err
		}
		conf.RootKeyGCInterval = dur
	}
	if rotationThreshold := agentConfig.Server.RootKeyRotationThreshold; rotationThreshold != "" {
		dur, err := time.ParseDuration(rotationThreshold)
		if err != nil {
			return nil, err
		}
		conf.RootKeyRotationThreshold = dur
	}

	if heartbeatGrace := agentConfig.Server.HeartbeatGrace; heartbeatGrace != 0 {
		conf.HeartbeatGrace = heartbeatGrace
	}
	if min := agentConfig.Server.MinHeartbeatTTL; min != 0 {
		conf.MinHeartbeatTTL = min
	}
	if maxHPS := agentConfig.Server.MaxHeartbeatsPerSecond; maxHPS != 0 {
		conf.MaxHeartbeatsPerSecond = maxHPS
	}
	if failoverTTL := agentConfig.Server.FailoverHeartbeatTTL; failoverTTL != 0 {
		conf.FailoverHeartbeatTTL = failoverTTL
	}

	if *agentConfig.Consul.AutoAdvertise && agentConfig.Consul.ServerServiceName == "" {
		return nil, fmt.Errorf("server_service_name must be set when auto_advertise is enabled")
	}

	// handle system scheduler preemption default
	if agentConfig.Server.DefaultSchedulerConfig != nil {
		conf.DefaultSchedulerConfig = *agentConfig.Server.DefaultSchedulerConfig
	}

	// Add the Consul and Vault configs
	conf.ConsulConfig = agentConfig.Consul
	for _, consulConfig := range agentConfig.Consuls {
		conf.ConsulConfigs[consulConfig.Name] = consulConfig
	}

	conf.VaultConfig = agentConfig.Vault
	for _, vaultConfig := range agentConfig.Vaults {
		conf.VaultConfigs[vaultConfig.Name] = vaultConfig
	}

	// Set the TLS config
	conf.TLSConfig = agentConfig.TLSConfig

	// Setup telemetry related config
	conf.StatsCollectionInterval = agentConfig.Telemetry.collectionInterval
	conf.DisableDispatchedJobSummaryMetrics = agentConfig.Telemetry.DisableDispatchedJobSummaryMetrics
	conf.DisableRPCRateMetricsLabels = agentConfig.Telemetry.DisableRPCRateMetricsLabels

	if d, err := time.ParseDuration(agentConfig.Limits.RPCHandshakeTimeout); err != nil {
		return nil, fmt.Errorf("error parsing rpc_handshake_timeout: %v", err)
	} else if d < 0 {
		return nil, fmt.Errorf("rpc_handshake_timeout must be >= 0")
	} else {
		conf.RPCHandshakeTimeout = d
	}

	// Set max rpc conns; nil/0 == unlimited
	// Leave a little room for streaming RPCs
	minLimit := config.LimitsNonStreamingConnsPerClient + 5
	if agentConfig.Limits.RPCMaxConnsPerClient == nil || *agentConfig.Limits.RPCMaxConnsPerClient == 0 {
		conf.RPCMaxConnsPerClient = 0
	} else if limit := *agentConfig.Limits.RPCMaxConnsPerClient; limit <= minLimit {
		return nil, fmt.Errorf("rpc_max_conns_per_client must be > %d; found: %d", minLimit, limit)
	} else {
		conf.RPCMaxConnsPerClient = limit
	}

	// Set deployment rate limit
	if rate := agentConfig.Server.DeploymentQueryRateLimit; rate == 0 {
		conf.DeploymentQueryRateLimit = deploymentwatcher.LimitStateQueriesPerSecond
	} else if rate > 0 {
		conf.DeploymentQueryRateLimit = rate
	} else {
		return nil, fmt.Errorf("deploy_query_rate_limit must be greater than 0")
	}

	// Set plan rejection tracker configuration.
	if planRejectConf := agentConfig.Server.PlanRejectionTracker; planRejectConf != nil {
		if planRejectConf.Enabled != nil {
			conf.NodePlanRejectionEnabled = *planRejectConf.Enabled
		}
		conf.NodePlanRejectionThreshold = planRejectConf.NodeThreshold

		if planRejectConf.NodeWindow == 0 {
			return nil, fmt.Errorf("plan_rejection_tracker.node_window must be greater than 0")
		} else {
			conf.NodePlanRejectionWindow = planRejectConf.NodeWindow
		}
	}

	// Add Enterprise license configs
	conf.LicenseConfig = &nomad.LicenseConfig{
		BuildDate:         agentConfig.Version.BuildDate,
		AdditionalPubKeys: agentConfig.Server.licenseAdditionalPublicKeys,
		LicenseEnvBytes:   agentConfig.Server.LicenseEnv,
		LicensePath:       agentConfig.Server.LicensePath,
	}

	// Add the search configuration
	if search := agentConfig.Server.Search; search != nil {
		conf.SearchConfig = &structs.SearchConfig{
			FuzzyEnabled:  search.FuzzyEnabled,
			LimitQuery:    search.LimitQuery,
			LimitResults:  search.LimitResults,
			MinTermLength: search.MinTermLength,
		}
	}

	// Set the raft bolt parameters
	if bolt := agentConfig.Server.RaftBoltConfig; bolt != nil {
		conf.RaftBoltNoFreelistSync = bolt.NoFreelistSync
	}

	// Interpret job_max_source_size as bytes from string value
	if agentConfig.Server.JobMaxSourceSize == nil {
		agentConfig.Server.JobMaxSourceSize = pointer.Of("1M")
	}
	jobMaxSourceBytes, err := humanize.ParseBytes(*agentConfig.Server.JobMaxSourceSize)
	if err != nil {
		return nil, fmt.Errorf("failed to parse max job source bytes: %w", err)
	}
	conf.JobMaxSourceSize = int(jobMaxSourceBytes)

	return conf, nil
}

// serverConfig is used to generate a new server configuration struct
// for initializing a nomad server.
func (a *Agent) serverConfig() (*nomad.Config, error) {
	c, err := convertServerConfig(a.config)
	if err != nil {
		return nil, err
	}

	a.finalizeServerConfig(c)
	return c, nil
}

// finalizeServerConfig sets configuration fields on the server config that are
// not statically convertible and are from the agent.
func (a *Agent) finalizeServerConfig(c *nomad.Config) {
	// Setup the logging
	c.Logger = a.logger
	c.LogOutput = a.logOutput
	c.AgentShutdown = func() error { return a.Shutdown() }
}

// clientConfig is used to generate a new client configuration struct for
// initializing a Nomad client.
func (a *Agent) clientConfig() (*clientconfig.Config, error) {
	c, err := convertClientConfig(a.config)
	if err != nil {
		return nil, err
	}

	if err = a.finalizeClientConfig(c); err != nil {
		return nil, err
	}

	return c, nil
}

// finalizeClientConfig sets configuration fields on the client config that are
// not statically convertible and are from the agent.
func (a *Agent) finalizeClientConfig(c *clientconfig.Config) error {
	// Setup the logging
	c.Logger = a.logger

	// If we are running a server, append both its bind and advertise address so
	// we are able to at least talk to the local server even if that isn't
	// configured explicitly. This handles both running server and client on one
	// host and -dev mode.
	if a.server != nil {
		advertised := a.config.AdvertiseAddrs
		normalized := a.config.normalizedAddrs

		if advertised == nil || advertised.RPC == "" {
			return fmt.Errorf("AdvertiseAddrs is nil or empty")
		} else if normalized == nil || normalized.RPC == "" {
			return fmt.Errorf("normalizedAddrs is nil or empty")
		}

		if normalized.RPC == advertised.RPC {
			c.Servers = append(c.Servers, normalized.RPC)
		} else {
			c.Servers = append(c.Servers, normalized.RPC, advertised.RPC)
		}
	}

	// Setup the plugin loaders
	c.PluginLoader = a.pluginLoader
	c.PluginSingletonLoader = a.pluginSingletonLoader

	// Log deprecation messages about Consul related configuration in client
	// options
	var invalidConsulKeys []string
	for key := range c.Options {
		if strings.HasPrefix(key, "consul") {
			invalidConsulKeys = append(invalidConsulKeys, fmt.Sprintf("options.%s", key))
		}
	}
	if len(invalidConsulKeys) > 0 {
		a.logger.Warn("invalid consul keys", "keys", strings.Join(invalidConsulKeys, ","))
		a.logger.Warn(`Nomad client ignores consul related configuration in client options.
		Please refer to the guide https://www.nomadproject.io/docs/agent/configuration/consul.html
		to configure Nomad to work with Consul.`)
	}

	return nil
}

// convertClientConfig takes an agent config and log output and returns a client
// Config. There may be missing fields that must be set by the agent. To do this
// call finalizeServerConfig
func convertClientConfig(agentConfig *Config) (*clientconfig.Config, error) {
	// Set up the configuration
	conf := agentConfig.ClientConfig
	if conf == nil {
		conf = clientconfig.DefaultConfig()
	}

	conf.Servers = agentConfig.Client.Servers
	conf.DevMode = agentConfig.DevMode
	conf.EnableDebug = agentConfig.EnableDebug

	if agentConfig.Region != "" {
		conf.Region = agentConfig.Region
	}
	if agentConfig.DataDir != "" {
		conf.StateDir = filepath.Join(agentConfig.DataDir, "client")
		conf.AllocDir = filepath.Join(agentConfig.DataDir, "alloc")
	}
	if agentConfig.Client.StateDir != "" {
		conf.StateDir = agentConfig.Client.StateDir
	}
	if agentConfig.Client.AllocDir != "" {
		conf.AllocDir = agentConfig.Client.AllocDir
	}
	if agentConfig.Client.NetworkInterface != "" {
		conf.NetworkInterface = agentConfig.Client.NetworkInterface
	}
	conf.ChrootEnv = agentConfig.Client.ChrootEnv
	conf.Options = agentConfig.Client.Options
	if agentConfig.Client.NetworkSpeed != 0 {
		conf.NetworkSpeed = agentConfig.Client.NetworkSpeed
	}
	if agentConfig.Client.CpuCompute != 0 {
		conf.CpuCompute = agentConfig.Client.CpuCompute
	}
	if agentConfig.Client.MemoryMB != 0 {
		conf.MemoryMB = agentConfig.Client.MemoryMB
	}
	if agentConfig.Client.DiskTotalMB != 0 {
		conf.DiskTotalMB = agentConfig.Client.DiskTotalMB
	}
	if agentConfig.Client.DiskFreeMB != 0 {
		conf.DiskFreeMB = agentConfig.Client.DiskFreeMB
	}
	if agentConfig.Client.MaxKillTimeout != "" {
		dur, err := time.ParseDuration(agentConfig.Client.MaxKillTimeout)
		if err != nil {
			return nil, fmt.Errorf("Error parsing max kill timeout: %s", err)
		}
		conf.MaxKillTimeout = dur
	}
	conf.ClientMaxPort = uint(agentConfig.Client.ClientMaxPort)
	conf.ClientMinPort = uint(agentConfig.Client.ClientMinPort)
	conf.MaxDynamicPort = agentConfig.Client.MaxDynamicPort
	conf.MinDynamicPort = agentConfig.Client.MinDynamicPort
	conf.DisableRemoteExec = agentConfig.Client.DisableRemoteExec

	if agentConfig.Client.TemplateConfig != nil {
		conf.TemplateConfig = agentConfig.Client.TemplateConfig.Copy()
	}

	hvMap := make(map[string]*structs.ClientHostVolumeConfig, len(agentConfig.Client.HostVolumes))
	for _, v := range agentConfig.Client.HostVolumes {
		hvMap[v.Name] = v
	}
	conf.HostVolumes = hvMap

	// Setup the node
	conf.Node = new(structs.Node)
	conf.Node.Datacenter = agentConfig.Datacenter
	conf.Node.Name = agentConfig.NodeName
	conf.Node.Meta = agentConfig.Client.Meta
	conf.Node.NodeClass = agentConfig.Client.NodeClass
	conf.Node.NodePool = agentConfig.Client.NodePool

	// Set up the HTTP advertise address
	conf.Node.HTTPAddr = agentConfig.AdvertiseAddrs.HTTP

	// Canonicalize Node struct
	conf.Node.Canonicalize()

	// Reserve resources on the node.
	// COMPAT(0.10): Remove in 0.10
	r := conf.Node.Reserved
	if r == nil {
		r = new(structs.Resources)
		conf.Node.Reserved = r
	}
	r.CPU = agentConfig.Client.Reserved.CPU
	r.MemoryMB = agentConfig.Client.Reserved.MemoryMB
	r.DiskMB = agentConfig.Client.Reserved.DiskMB

	res := conf.Node.ReservedResources
	if res == nil {
		res = new(structs.NodeReservedResources)
		conf.Node.ReservedResources = res
	}
	res.Cpu.CpuShares = int64(agentConfig.Client.Reserved.CPU)
	res.Memory.MemoryMB = int64(agentConfig.Client.Reserved.MemoryMB)
	res.Disk.DiskMB = int64(agentConfig.Client.Reserved.DiskMB)
	res.Networks.ReservedHostPorts = agentConfig.Client.Reserved.ReservedPorts
	if agentConfig.Client.Reserved.Cores != "" {
		cores, err := cpuset.Parse(agentConfig.Client.Reserved.Cores)
		if err != nil {
			return nil, fmt.Errorf("failed to parse client > reserved > cores value %q: %v", agentConfig.Client.Reserved.Cores, err)
		}
		res.Cpu.ReservedCpuCores = cores.ToSlice()
	}

	conf.Version = agentConfig.Version

	if *agentConfig.Consul.AutoAdvertise && agentConfig.Consul.ClientServiceName == "" {
		return nil, fmt.Errorf("client_service_name must be set when auto_advertise is enabled")
	}

	conf.ConsulConfig = agentConfig.Consul
	for _, consulConfig := range agentConfig.Consuls {
		conf.ConsulConfigs[consulConfig.Name] = consulConfig
	}

	conf.VaultConfig = agentConfig.Vault
	for _, vaultConfig := range agentConfig.Vaults {
		conf.VaultConfigs[vaultConfig.Name] = vaultConfig
	}

	// Set up Telemetry configuration
	conf.StatsCollectionInterval = agentConfig.Telemetry.collectionInterval
	conf.PublishNodeMetrics = agentConfig.Telemetry.PublishNodeMetrics
	conf.PublishAllocationMetrics = agentConfig.Telemetry.PublishAllocationMetrics

	// Set the TLS related configs
	conf.TLSConfig = agentConfig.TLSConfig
	conf.Node.TLSEnabled = conf.TLSConfig.EnableHTTP

	// Set the GC related configs
	conf.GCInterval = agentConfig.Client.GCInterval
	conf.GCParallelDestroys = agentConfig.Client.GCParallelDestroys
	conf.GCDiskUsageThreshold = agentConfig.Client.GCDiskUsageThreshold
	conf.GCInodeUsageThreshold = agentConfig.Client.GCInodeUsageThreshold
	conf.GCMaxAllocs = agentConfig.Client.GCMaxAllocs
	if agentConfig.Client.NoHostUUID != nil {
		conf.NoHostUUID = *agentConfig.Client.NoHostUUID
	} else {
		// Default no_host_uuid to true
		conf.NoHostUUID = true
	}

	// Setup the ACLs
	conf.ACLEnabled = agentConfig.ACL.Enabled
	conf.ACLTokenTTL = agentConfig.ACL.TokenTTL
	conf.ACLPolicyTTL = agentConfig.ACL.PolicyTTL
	conf.ACLRoleTTL = agentConfig.ACL.RoleTTL

	// Setup networking configuration
	conf.CNIPath = agentConfig.Client.CNIPath
	conf.CNIConfigDir = agentConfig.Client.CNIConfigDir
	conf.BridgeNetworkName = agentConfig.Client.BridgeNetworkName
	conf.BridgeNetworkAllocSubnet = agentConfig.Client.BridgeNetworkSubnet
	conf.BridgeNetworkHairpinMode = agentConfig.Client.BridgeNetworkHairpinMode

	for _, hn := range agentConfig.Client.HostNetworks {
		conf.HostNetworks[hn.Name] = hn
	}
	conf.BindWildcardDefaultHostNetwork = agentConfig.Client.BindWildcardDefaultHostNetwork

	if agentConfig.Client.NomadServiceDiscovery != nil {
		conf.NomadServiceDiscovery = *agentConfig.Client.NomadServiceDiscovery
	}

	artifactConfig, err := clientconfig.ArtifactConfigFromAgent(agentConfig.Client.Artifact)
	if err != nil {
		return nil, fmt.Errorf("invalid artifact config: %v", err)
	}
	conf.Artifact = artifactConfig

	drainConfig, err := clientconfig.DrainConfigFromAgent(agentConfig.Client.Drain)
	if err != nil {
		return nil, fmt.Errorf("invalid drain_on_shutdown config: %v", err)
	}
	conf.Drain = drainConfig

	return conf, nil
}

// setupServer is used to setup the server if enabled
func (a *Agent) setupServer() error {
	if !a.config.Server.Enabled {
		return nil
	}

	// Setup the configuration
	conf, err := a.serverConfig()
	if err != nil {
		return fmt.Errorf("server config setup failed: %s", err)
	}

	// Generate a node ID and persist it if it is the first instance, otherwise
	// read the persisted node ID.
	if err := a.setupNodeID(conf); err != nil {
		return fmt.Errorf("setting up server node ID failed: %s", err)
	}

	// Sets up the keyring for gossip encryption
	if err := a.setupKeyrings(conf); err != nil {
		return fmt.Errorf("failed to configure keyring: %v", err)
	}

	// Create the server
	server, err := nomad.NewServer(conf, a.consulCatalog, a.consulConfigEntries, a.consulACLs)
	if err != nil {
		return fmt.Errorf("server setup failed: %v", err)
	}
	a.server = server

	// Consul check addresses default to bind but can be toggled to use advertise
	rpcCheckAddr := a.config.normalizedAddrs.RPC
	serfCheckAddr := a.config.normalizedAddrs.Serf
	if *a.config.Consul.ChecksUseAdvertise {
		rpcCheckAddr = a.config.AdvertiseAddrs.RPC
		serfCheckAddr = a.config.AdvertiseAddrs.Serf
	}

	// Create the Nomad Server services for Consul
	if *a.config.Consul.AutoAdvertise {
		httpServ := &structs.Service{
			Name:      a.config.Consul.ServerServiceName,
			PortLabel: a.config.AdvertiseAddrs.HTTP,
			Tags:      append([]string{consul.ServiceTagHTTP}, a.config.Consul.Tags...),
		}
		const isServer = true
		if check := a.agentHTTPCheck(isServer); check != nil {
			httpServ.Checks = []*structs.ServiceCheck{check}
		}
		rpcServ := &structs.Service{
			Name:      a.config.Consul.ServerServiceName,
			PortLabel: a.config.AdvertiseAddrs.RPC,
			Tags:      append([]string{consul.ServiceTagRPC}, a.config.Consul.Tags...),
			Checks: []*structs.ServiceCheck{
				{
					Name:      a.config.Consul.ServerRPCCheckName,
					Type:      "tcp",
					Interval:  serverRpcCheckInterval,
					Timeout:   serverRpcCheckTimeout,
					PortLabel: rpcCheckAddr,
				},
			},
		}
		serfServ := &structs.Service{
			Name:      a.config.Consul.ServerServiceName,
			PortLabel: a.config.AdvertiseAddrs.Serf,
			Tags:      append([]string{consul.ServiceTagSerf}, a.config.Consul.Tags...),
			Checks: []*structs.ServiceCheck{
				{
					Name:      a.config.Consul.ServerSerfCheckName,
					Type:      "tcp",
					Interval:  serverSerfCheckInterval,
					Timeout:   serverSerfCheckTimeout,
					PortLabel: serfCheckAddr,
				},
			},
		}

		// Add the http port check if TLS isn't enabled
		consulServices := []*structs.Service{
			rpcServ,
			serfServ,
			httpServ,
		}
		if err := a.consulService.RegisterAgent(consulRoleServer, consulServices); err != nil {
			return err
		}
	}

	return nil
}

// setupNodeID will pull the persisted node ID, if any, or create a random one
// and persist it.
func (a *Agent) setupNodeID(config *nomad.Config) error {
	// For dev mode we have no filesystem access so just make a node ID.
	if a.config.DevMode {
		config.NodeID = uuid.Generate()
		return nil
	}

	// Load saved state, if any. Since a user could edit this, we also
	// validate it. Saved state overwrites any configured node id
	fileID := filepath.Join(config.DataDir, "node-id")
	if _, err := os.Stat(fileID); err == nil {
		rawID, err := os.ReadFile(fileID)
		if err != nil {
			return err
		}

		nodeID := strings.TrimSpace(string(rawID))
		nodeID = strings.ToLower(nodeID)
		if _, err := uuidparse.ParseUUID(nodeID); err != nil {
			return err
		}
		config.NodeID = nodeID
		return nil
	}

	// If they've configured a node ID manually then just use that, as
	// long as it's valid.
	if config.NodeID != "" {
		config.NodeID = strings.ToLower(config.NodeID)
		if _, err := uuidparse.ParseUUID(config.NodeID); err != nil {
			return err
		}
		// Persist this configured nodeID to our data directory
		if err := escapingfs.EnsurePath(fileID, false); err != nil {
			return err
		}
		if err := os.WriteFile(fileID, []byte(config.NodeID), 0600); err != nil {
			return err
		}
		return nil
	}

	// If we still don't have a valid node ID, make one.
	if config.NodeID == "" {
		id := uuid.Generate()
		if err := escapingfs.EnsurePath(fileID, false); err != nil {
			return err
		}
		if err := os.WriteFile(fileID, []byte(id), 0600); err != nil {
			return err
		}

		config.NodeID = id
	}
	return nil
}

// setupKeyrings is used to initialize and load keyrings during agent startup
func (a *Agent) setupKeyrings(config *nomad.Config) error {
	file := filepath.Join(a.config.DataDir, serfKeyring)

	if a.config.Server.EncryptKey == "" {
		goto LOAD
	}
	if _, err := os.Stat(file); err != nil {
		if err := initKeyring(file, a.config.Server.EncryptKey, a.logger); err != nil {
			return err
		}
	}

LOAD:
	if _, err := os.Stat(file); err == nil {
		config.SerfConfig.KeyringFile = file
	}
	if err := loadKeyringFile(config.SerfConfig); err != nil {
		return err
	}
	// Success!
	return nil
}

// setupClient is used to setup the client if enabled
func (a *Agent) setupClient() error {
	if !a.config.Client.Enabled {
		return nil
	}

	// Plugin setup must happen before the call to clientConfig, because it
	// copies the pointers to the plugin loaders from the Agent to the
	// Client config.
	if err := a.setupPlugins(); err != nil {
		return err
	}

	// Setup the configuration
	conf, err := a.clientConfig()
	if err != nil {
		return fmt.Errorf("client setup failed: %v", err)
	}

	// Reserve some ports for the plugins if we are on Windows
	if runtime.GOOS == "windows" {
		if err := a.reservePortsForClient(conf); err != nil {
			return err
		}
	}
	if conf.StateDBFactory == nil {
		conf.StateDBFactory = state.GetStateDBFactory(conf.DevMode)
	}

	// Set up a custom listener and dialer. This is used by Nomad clients when
	// running consul-template functions that utilize the Nomad API. We lazy
	// load this into the client config, therefore this needs to happen before
	// we call NewClient.
	a.builtinListener, a.builtinDialer = bufconndialer.New()
	conf.TemplateDialer = a.builtinDialer

	// Initialize builtin Task API server here for use in the client, but it
	// won't accept connections until the HTTP servers are created.
	a.taskAPIServer = newBuiltinAPI()
	conf.APIListenerRegistrar = a.taskAPIServer

	nomadClient, err := client.NewClient(
		conf, a.consulCatalog, a.consulProxies, a.consulService, nil)
	if err != nil {
		return fmt.Errorf("client setup failed: %v", err)
	}
	a.client = nomadClient

	// Create the Nomad Client  services for Consul
	if *a.config.Consul.AutoAdvertise {
		httpServ := &structs.Service{
			Name:      a.config.Consul.ClientServiceName,
			PortLabel: a.config.AdvertiseAddrs.HTTP,
			Tags:      append([]string{consul.ServiceTagHTTP}, a.config.Consul.Tags...),
		}
		const isServer = false
		if check := a.agentHTTPCheck(isServer); check != nil {
			httpServ.Checks = []*structs.ServiceCheck{check}
		}
		if err := a.consulService.RegisterAgent(consulRoleClient, []*structs.Service{httpServ}); err != nil {
			return err
		}
	}

	return nil
}

// agentHTTPCheck returns a health check for the agent's HTTP API if possible.
// If no HTTP health check can be supported nil is returned.
func (a *Agent) agentHTTPCheck(server bool) *structs.ServiceCheck {
	// Resolve the http check address
	httpCheckAddr := a.config.normalizedAddrs.HTTP[0]
	if *a.config.Consul.ChecksUseAdvertise {
		httpCheckAddr = a.config.AdvertiseAddrs.HTTP
	}
	check := structs.ServiceCheck{
		Name:      a.config.Consul.ClientHTTPCheckName,
		Type:      "http",
		Path:      "/v1/agent/health?type=client",
		Protocol:  "http",
		Interval:  agentHttpCheckInterval,
		Timeout:   agentHttpCheckTimeout,
		PortLabel: httpCheckAddr,
	}
	// Switch to endpoint that doesn't require a leader for servers
	if server {
		check.Name = a.config.Consul.ServerHTTPCheckName
		check.Path = "/v1/agent/health?type=server"
	}
	if !a.config.TLSConfig.EnableHTTP {
		// No HTTPS, return a plain http check
		return &check
	}
	if a.config.TLSConfig.VerifyHTTPSClient {
		a.logger.Warn("not registering Nomad HTTPS Health Check because verify_https_client enabled")
		return nil
	}

	// HTTPS enabled; skip verification
	check.Protocol = "https"
	check.TLSSkipVerify = true
	return &check
}

// reservePortsForClient reserves a range of ports for the client to use when
// it creates various plugins for log collection, executors, drivers, etc
func (a *Agent) reservePortsForClient(conf *clientconfig.Config) error {
	if conf.Node.ReservedResources == nil {
		conf.Node.ReservedResources = &structs.NodeReservedResources{}
	}

	res := conf.Node.ReservedResources.Networks.ReservedHostPorts
	if res == "" {
		res = fmt.Sprintf("%d-%d", conf.ClientMinPort, conf.ClientMaxPort)
	} else {
		res += fmt.Sprintf(",%d-%d", conf.ClientMinPort, conf.ClientMaxPort)
	}
	conf.Node.ReservedResources.Networks.ReservedHostPorts = res
	return nil
}

// Leave is used gracefully exit. Clients will inform servers
// of their departure so that allocations can be rescheduled.
func (a *Agent) Leave() error {
	if a.client != nil {
		if err := a.client.Leave(); err != nil {
			a.logger.Error("client leave failed", "error", err)
		}
	}
	if a.server != nil {
		if err := a.server.Leave(); err != nil {
			a.logger.Error("server leave failed", "error", err)
		}
	}
	return nil
}

// Shutdown is used to terminate the agent.
func (a *Agent) Shutdown() error {
	a.shutdownLock.Lock()
	defer a.shutdownLock.Unlock()

	if a.shutdown {
		return nil
	}

	a.logger.Info("requesting shutdown")
	if a.client != nil {
		// Task API must be closed separately from other HTTP servers and should
		// happen before the client is shutdown
		a.taskAPIServer.Shutdown()

		if err := a.client.Shutdown(); err != nil {
			a.logger.Error("client shutdown failed", "error", err)
		}
	}
	if a.server != nil {
		if err := a.server.Shutdown(); err != nil {
			a.logger.Error("server shutdown failed", "error", err)
		}
	}

	if err := a.consulService.Shutdown(); err != nil {
		a.logger.Error("shutting down Consul client failed", "error", err)
	}

	a.logger.Info("shutdown complete")
	a.shutdown = true
	close(a.shutdownCh)
	return nil
}

// RPC is used to make an RPC call to the Nomad servers
func (a *Agent) RPC(method string, args interface{}, reply interface{}) error {
	if a.server != nil {
		return a.server.RPC(method, args, reply)
	}
	return a.client.RPC(method, args, reply)
}

// Client returns the configured client or nil
func (a *Agent) Client() *client.Client {
	return a.client
}

// Server returns the configured server or nil
func (a *Agent) Server() *nomad.Server {
	return a.server
}

// Stats is used to return statistics for debugging and insight
// for various sub-systems
func (a *Agent) Stats() map[string]map[string]string {
	stats := make(map[string]map[string]string)
	if a.server != nil {
		subStat := a.server.Stats()
		for k, v := range subStat {
			stats[k] = v
		}
	}
	if a.client != nil {
		subStat := a.client.Stats()
		for k, v := range subStat {
			stats[k] = v
		}
	}
	return stats
}

// ShouldReload determines if we should reload the configuration and agent
// connections. If the TLS Configuration has not changed, we shouldn't reload.
func (a *Agent) ShouldReload(newConfig *Config) (agent, http bool) {
	a.configLock.Lock()
	defer a.configLock.Unlock()

	if newConfig.LogLevel != "" && newConfig.LogLevel != a.config.LogLevel {
		agent = true
	}

	isEqual, err := a.config.TLSConfig.CertificateInfoIsEqual(newConfig.TLSConfig)
	if err != nil {
		a.logger.Error("parsing TLS certificate", "error", err)
		return agent, false
	} else if !isEqual {
		return true, true
	}

	// Allow the ability to only reload HTTP connections
	if a.config.TLSConfig.EnableHTTP != newConfig.TLSConfig.EnableHTTP {
		http = true
		agent = true
	}

	// Allow the ability to only reload HTTP connections
	if a.config.TLSConfig.EnableRPC != newConfig.TLSConfig.EnableRPC {
		agent = true
	}

	if a.config.TLSConfig.RPCUpgradeMode != newConfig.TLSConfig.RPCUpgradeMode {
		agent = true
	}

	return agent, http
}

// Reload handles configuration changes for the agent. Provides a method that
// is easier to unit test, as this action is invoked via SIGHUP.
func (a *Agent) Reload(newConfig *Config) error {
	a.configLock.Lock()
	defer a.configLock.Unlock()

	current := a.config.Copy()

	updatedLogging := newConfig != nil && (newConfig.LogLevel != current.LogLevel)

	if newConfig == nil || newConfig.TLSConfig == nil && !updatedLogging {
		return fmt.Errorf("cannot reload agent with nil configuration")
	}

	if updatedLogging {
		current.LogLevel = newConfig.LogLevel
		a.logger.SetLevel(log.LevelFromString(current.LogLevel))
	}

	// Update eventer config
	if newConfig.Audit != nil {
		if err := a.entReloadEventer(newConfig.Audit); err != nil {
			return err
		}
	}
	// Allow auditor to call reopen regardless of config changes
	// This is primarily for enterprise audit logging to allow the underlying
	// file to be reopened if necessary
	if err := a.auditor.Reopen(); err != nil {
		return err
	}

	fullUpdateTLSConfig := func() {
		// Completely reload the agent's TLS configuration (moving from non-TLS to
		// TLS, or vice versa)
		// This does not handle errors in loading the new TLS configuration
		current.TLSConfig = newConfig.TLSConfig.Copy()
	}

	if !current.TLSConfig.IsEmpty() && !newConfig.TLSConfig.IsEmpty() {
		// This is just a TLS configuration reload, we don't need to refresh
		// existing network connections

		// Reload the certificates on the keyloader and on success store the
		// updated TLS config. It is important to reuse the same keyloader
		// as this allows us to dynamically reload configurations not only
		// on the Agent but on the Server and Client too (they are
		// referencing the same keyloader).
		keyloader := current.TLSConfig.GetKeyLoader()
		_, err := keyloader.LoadKeyPair(newConfig.TLSConfig.CertFile, newConfig.TLSConfig.KeyFile)
		if err != nil {
			return err
		}

		current.TLSConfig = newConfig.TLSConfig
		current.TLSConfig.KeyLoader = keyloader
		a.config = current
		return nil
	} else if newConfig.TLSConfig.IsEmpty() && !current.TLSConfig.IsEmpty() {
		a.logger.Warn("downgrading agent's existing TLS configuration to plaintext")
		fullUpdateTLSConfig()
	} else if !newConfig.TLSConfig.IsEmpty() && current.TLSConfig.IsEmpty() {
		a.logger.Info("upgrading from plaintext configuration to TLS")
		fullUpdateTLSConfig()
	}

	// Set agent config to the updated config
	a.config = current
	return nil
}

// GetConfig returns the current agent configuration. The Config should *not*
// be mutated directly. First call Config.Copy.
func (a *Agent) GetConfig() *Config {
	a.configLock.Lock()
	defer a.configLock.Unlock()

	return a.config
}

// GetMetricsSink returns the metrics sink.
func (a *Agent) GetMetricsSink() *metrics.InmemSink {
	return a.inmemSink
}

// setupConsul creates the Consul client and starts its main Run loop.
func (a *Agent) setupConsul(consulConfig *config.ConsulConfig) error {
	apiConf, err := consulConfig.ApiConfig()
	if err != nil {
		return err
	}

	consulClient, err := consulapi.NewClient(apiConf)
	if err != nil {
		return err
	}

	// Create Consul Catalog client for service discovery.
	a.consulCatalog = consulClient.Catalog()

	// Create Consul ConfigEntries client for managing Config Entries.
	a.consulConfigEntries = consulClient.ConfigEntries()

	// Create Consul ACL client for managing tokens.
	a.consulACLs = consulClient.ACL()

	// Create Consul Service client for service advertisement and checks.
	isClient := false
	if a.config.Client != nil && a.config.Client.Enabled {
		isClient = true
	}
	// Create Consul Agent client for looking info about the agent.
	consulAgentClient := consulClient.Agent()
	namespacesClient := consul.NewNamespacesClient(consulClient.Namespaces(), consulAgentClient)
	a.consulService = consul.NewServiceClient(consulAgentClient, namespacesClient, a.logger, isClient)
	a.consulProxies = consul.NewConnectProxiesClient(consulAgentClient)

	// Run the Consul service client's sync'ing main loop
	go a.consulService.Run()
	return nil
}

// noOpAuditor is a no-op Auditor that fulfills the
// event.Auditor interface.
type noOpAuditor struct{}

// Ensure noOpAuditor is an Auditor
var _ event.Auditor = &noOpAuditor{}

func (e *noOpAuditor) Event(ctx context.Context, eventType string, payload interface{}) error {
	return nil
}

func (e *noOpAuditor) Enabled() bool {
	return false
}

func (e *noOpAuditor) Reopen() error {
	return nil
}

func (e *noOpAuditor) SetEnabled(enabled bool) {}

func (e *noOpAuditor) DeliveryEnforced() bool { return false }
