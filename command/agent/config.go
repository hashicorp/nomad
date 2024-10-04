// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-secure-stdlib/listenerutil"
	sockaddr "github.com/hashicorp/go-sockaddr"
	"github.com/hashicorp/go-sockaddr/template"
	client "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/users"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/version"
)

// Config is the configuration for the Nomad agent.
//
// time.Duration values have two parts:
//   - a string field tagged with an hcl:"foo" and json:"-"
//   - a time.Duration field in the same struct and a call to duration
//     in config_parse.go ParseConfigFile
//
// All config structs should have an ExtraKeysHCL field to check for
// unexpected keys
type Config struct {
	// Region is the region this agent is in. Defaults to global.
	Region string `hcl:"region"`

	// Datacenter is the datacenter this agent is in. Defaults to dc1
	Datacenter string `hcl:"datacenter"`

	// NodeName is the name we register as. Defaults to hostname.
	NodeName string `hcl:"name"`

	// DataDir is the directory to store our state in
	DataDir string `hcl:"data_dir"`

	// PluginDir is the directory to lookup plugins.
	PluginDir string `hcl:"plugin_dir"`

	// LogLevel is the level of the logs to put out
	LogLevel string `hcl:"log_level"`

	// LogJson enables log output in a JSON format
	LogJson bool `hcl:"log_json"`

	// LogFile enables logging to a file
	LogFile string `hcl:"log_file"`

	// LogIncludeLocation dictates whether the logger includes file and line
	// information on each log line. This is useful for Nomad development and
	// debugging.
	LogIncludeLocation bool `hcl:"log_include_location"`

	// LogRotateDuration is the time period that logs should be rotated in
	LogRotateDuration string `hcl:"log_rotate_duration"`

	// LogRotateBytes is the max number of bytes that should be written to a file
	LogRotateBytes int `hcl:"log_rotate_bytes"`

	// LogRotateMaxFiles is the max number of log files to keep
	LogRotateMaxFiles int `hcl:"log_rotate_max_files"`

	// BindAddr is the address on which all of nomad's services will
	// be bound. If not specified, this defaults to 127.0.0.1.
	BindAddr string `hcl:"bind_addr"`

	// EnableDebug is used to enable debugging HTTP endpoints
	EnableDebug bool `hcl:"enable_debug"`

	// Ports is used to control the network ports we bind to.
	Ports *Ports `hcl:"ports"`

	// Addresses is used to override the network addresses we bind to.
	//
	// Use normalizedAddrs if you need the host+port to bind to.
	Addresses *Addresses `hcl:"addresses"`

	// normalizedAddr is set to the Address+Port by normalizeAddrs()
	normalizedAddrs *NormalizedAddrs

	// AdvertiseAddrs is used to control the addresses we advertise.
	AdvertiseAddrs *AdvertiseAddrs `hcl:"advertise"`

	// Client has our client related settings
	Client *ClientConfig `hcl:"client"`

	// Server has our server related settings
	Server *ServerConfig `hcl:"server"`

	// ACL has our acl related settings
	ACL *ACLConfig `hcl:"acl"`

	// Telemetry is used to configure sending telemetry
	Telemetry *Telemetry `hcl:"telemetry"`

	// LeaveOnInt is used to gracefully leave on the interrupt signal
	LeaveOnInt bool `hcl:"leave_on_interrupt"`

	// LeaveOnTerm is used to gracefully leave on the terminate signal
	LeaveOnTerm bool `hcl:"leave_on_terminate"`

	// EnableSyslog is used to enable sending logs to syslog
	EnableSyslog bool `hcl:"enable_syslog"`

	// SyslogFacility is used to control the syslog facility used.
	SyslogFacility string `hcl:"syslog_facility"`

	// DisableUpdateCheck is used to disable the periodic update
	// and security bulletin checking.
	DisableUpdateCheck *bool `hcl:"disable_update_check"`

	// DisableAnonymousSignature is used to disable setting the
	// anonymous signature when doing the update check and looking
	// for security bulletins
	DisableAnonymousSignature bool `hcl:"disable_anonymous_signature"`

	// Consuls is a slice derived from multiple `consul` blocks, here to support
	// features in Nomad Enterprise.
	Consuls []*config.ConsulConfig `hcl:"-"`

	// Vaults is a slice derived from multiple `vault` blocks, here to support
	// features in Nomad Enterprise.
	Vaults []*config.VaultConfig `hcl:"-"`

	// UI is used to configure the web UI
	UI *config.UIConfig `hcl:"ui"`

	// NomadConfig is used to override the default config.
	// This is largely used for testing purposes.
	NomadConfig *nomad.Config `hcl:"-" json:"-"`

	// ClientConfig is used to override the default config.
	// This is largely used for testing purposes.
	ClientConfig *client.Config `hcl:"-" json:"-"`

	// DevMode is set by the -dev CLI flag.
	DevMode bool `hcl:"-"`

	// Version information is set at compilation time
	Version *version.VersionInfo

	// List of config files that have been loaded (in order)
	Files []string `hcl:"-"`

	// TLSConfig provides TLS related configuration for the Nomad server and
	// client
	TLSConfig *config.TLSConfig `hcl:"tls"`

	// HTTPAPIResponseHeaders allows users to configure the Nomad http agent to
	// set arbitrary headers on API responses
	HTTPAPIResponseHeaders map[string]string `hcl:"http_api_response_headers"`

	// Sentinel holds sentinel related settings
	Sentinel *config.SentinelConfig `hcl:"sentinel"`

	// Autopilot contains the configuration for Autopilot behavior.
	Autopilot *config.AutopilotConfig `hcl:"autopilot"`

	// Plugins is the set of configured plugins
	Plugins []*config.PluginConfig `hcl:"plugin"`

	// Limits contains the configuration for timeouts.
	Limits config.Limits `hcl:"limits"`

	// Audit contains the configuration for audit logging.
	Audit *config.AuditConfig `hcl:"audit"`

	// Reporting is used to enable go census reporting
	Reporting *config.ReportingConfig `hcl:"reporting,block"`

	// KEKProviders are used to wrap the Nomad keyring
	KEKProviders []*structs.KEKProviderConfig `hcl:"keyring"`

	// ExtraKeysHCL is used by hcl to surface unexpected keys
	ExtraKeysHCL []string `hcl:",unusedKeys" json:"-"`
}

func (c *Config) defaultConsul() *config.ConsulConfig {
	for _, cfg := range c.Consuls {
		if cfg.Name == structs.ConsulDefaultCluster {
			return cfg
		}
	}
	return nil
}

func (c *Config) defaultVault() *config.VaultConfig {
	for _, cfg := range c.Vaults {
		if cfg.Name == structs.VaultDefaultCluster {
			return cfg
		}
	}
	return nil
}

// ClientConfig is configuration specific to the client mode
type ClientConfig struct {
	// Enabled controls if we are a client
	Enabled bool `hcl:"enabled"`

	// StateDir is the state directory
	StateDir string `hcl:"state_dir"`

	// AllocDir is the directory for storing allocation data
	AllocDir string `hcl:"alloc_dir"`

	// AllocMountsDir is the directory for storing mounts into allocation data
	AllocMountsDir string `hcl:"alloc_mounts_dir"`

	// Servers is a list of known server addresses. These are as "host:port"
	Servers []string `hcl:"servers"`

	// NodeClass is used to group the node by class
	NodeClass string `hcl:"node_class"`

	// NodePool defines the node pool in which the client is registered.
	//
	// If the node pool does not exist, it will be created automatically if the
	// node registers in the authoritative region. In non-authoritative
	// regions, the node is kept in the 'initializing' status until the node
	// pool is created and replicated.
	NodePool string `hcl:"node_pool"`

	// Options is used for configuration of nomad internals,
	// like fingerprinters and drivers. The format is:
	//
	//  namespace.option = value
	Options map[string]string `hcl:"options"`

	// Metadata associated with the node
	Meta map[string]string `hcl:"meta"`

	// A mapping of directories on the host OS to attempt to embed inside each
	// task's chroot.
	ChrootEnv map[string]string `hcl:"chroot_env"`

	// Interface to use for network fingerprinting
	NetworkInterface string `hcl:"network_interface"`

	// Sort the IP addresses by the preferred IP family. This is useful when
	// the interface has multiple IP addresses and the client should prefer
	// one over the other.
	PreferredAddressFamily structs.NodeNetworkAF `hcl:"preferred_address_family"`

	// NetworkSpeed is used to override any detected or default network link
	// speed.
	NetworkSpeed int `hcl:"network_speed"`

	// CpuCompute is used to override any detected or default total CPU compute.
	CpuCompute int `hcl:"cpu_total_compute"`

	// MemoryMB is used to override any detected or default total memory.
	MemoryMB int `hcl:"memory_total_mb"`

	// DiskTotalMB is used to override any detected or default total disk space.
	DiskTotalMB int `hcl:"disk_total_mb"`

	// DiskFreeMB is used to override any detected or default free disk space.
	DiskFreeMB int `hcl:"disk_free_mb"`

	// ReservableCores is used to override detected reservable cpu cores.
	ReservableCores string `hcl:"reservable_cores"`

	// MaxKillTimeout allows capping the user-specifiable KillTimeout.
	MaxKillTimeout string `hcl:"max_kill_timeout"`

	// ClientMaxPort is the upper range of the ports that the client uses for
	// communicating with plugin subsystems
	ClientMaxPort int `hcl:"client_max_port"`

	// ClientMinPort is the lower range of the ports that the client uses for
	// communicating with plugin subsystems
	ClientMinPort int `hcl:"client_min_port"`

	// MaxDynamicPort is the upper range of the dynamic ports that the client
	// uses for allocations
	MaxDynamicPort int `hcl:"max_dynamic_port"`

	// MinDynamicPort is the lower range of the dynamic ports that the client
	// uses for allocations
	MinDynamicPort int `hcl:"min_dynamic_port"`

	// Reserved is used to reserve resources from being used by Nomad. This can
	// be used to target a certain utilization or to prevent Nomad from using a
	// particular set of ports.
	Reserved *Resources `hcl:"reserved"`

	// GCInterval is the time interval at which the client triggers garbage
	// collection
	GCInterval    time.Duration
	GCIntervalHCL string `hcl:"gc_interval" json:"-"`

	// GCParallelDestroys is the number of parallel destroys the garbage
	// collector will allow.
	GCParallelDestroys int `hcl:"gc_parallel_destroys"`

	// GCDiskUsageThreshold is the disk usage threshold given as a percent
	// beyond which the Nomad client triggers GC of terminal allocations
	GCDiskUsageThreshold float64 `hcl:"gc_disk_usage_threshold"`

	// GCInodeUsageThreshold is the inode usage threshold beyond which the Nomad
	// client triggers GC of the terminal allocations
	GCInodeUsageThreshold float64 `hcl:"gc_inode_usage_threshold"`

	// GCMaxAllocs is the maximum number of allocations a node can have
	// before garbage collection is triggered.
	GCMaxAllocs int `hcl:"gc_max_allocs"`

	// NoHostUUID disables using the host's UUID and will force generation of a
	// random UUID.
	NoHostUUID *bool `hcl:"no_host_uuid"`

	// DisableRemoteExec disables remote exec targeting tasks on this client
	DisableRemoteExec bool `hcl:"disable_remote_exec"`

	// TemplateConfig includes configuration for template rendering
	TemplateConfig *client.ClientTemplateConfig `hcl:"template"`

	// ServerJoin contains information that is used to attempt to join servers
	ServerJoin *ServerJoin `hcl:"server_join"`

	// HostVolumes contains information about the volumes an operator has made
	// available to jobs running on this node.
	HostVolumes []*structs.ClientHostVolumeConfig `hcl:"host_volume"`

	// CNIPath is the path to search for CNI plugins, multiple paths can be
	// specified colon delimited
	CNIPath string `hcl:"cni_path"`

	// CNIConfigDir is the directory where CNI network configuration is located. The
	// client will use this path when fingerprinting CNI networks.
	CNIConfigDir string `hcl:"cni_config_dir"`

	// BridgeNetworkName is the name of the bridge to create when using the
	// bridge network mode
	BridgeNetworkName string `hcl:"bridge_network_name"`

	// BridgeNetworkSubnet is the subnet to allocate IPv4 addresses from when
	// creating allocations with bridge networking mode. This range is local to
	// the host
	BridgeNetworkSubnet string `hcl:"bridge_network_subnet"`

	// BridgeNetworkSubnetIPv6 is the subnet to allocate IPv6 addresses when
	// creating allocations with bridge networking mode. This range is local to
	// the host
	BridgeNetworkSubnetIPv6 string `hcl:"bridge_network_subnet_ipv6"`

	// BridgeNetworkHairpinMode is whether or not to enable hairpin mode on the
	// internal bridge network
	BridgeNetworkHairpinMode bool `hcl:"bridge_network_hairpin_mode"`

	// HostNetworks describes the different host networks available to the host
	// if the host uses multiple interfaces
	HostNetworks []*structs.ClientHostNetworkConfig `hcl:"host_network"`

	// BindWildcardDefaultHostNetwork toggles if when there are no host networks,
	// should the port mapping rules match the default network address (false) or
	// matching any destination address (true). Defaults to true
	BindWildcardDefaultHostNetwork bool `hcl:"bind_wildcard_default_host_network"`

	// CgroupParent sets the parent cgroup for subsystems managed by Nomad. If the cgroup
	// doest not exist Nomad will attempt to create it during startup. Defaults to '/nomad'
	CgroupParent string `hcl:"cgroup_parent"`

	// NomadServiceDiscovery is a boolean parameter which allows operators to
	// enable/disable to Nomad native service discovery feature on the client.
	// This parameter is exposed via the Nomad fingerprinter and used to ensure
	// correct scheduling decisions on allocations which require this.
	NomadServiceDiscovery *bool `hcl:"nomad_service_discovery"`

	// Artifact contains the configuration for artifacts.
	Artifact *config.ArtifactConfig `hcl:"artifact"`

	// Drain specifies whether to drain the client on shutdown; ignored in dev mode.
	Drain *config.DrainConfig `hcl:"drain_on_shutdown"`

	// Users is used to configure parameters around operating system users.
	Users *config.UsersConfig `hcl:"users"`

	// ExtraKeysHCL is used by hcl to surface unexpected keys
	ExtraKeysHCL []string `hcl:",unusedKeys" json:"-"`
}

func (c *ClientConfig) Copy() *ClientConfig {
	if c == nil {
		return c
	}

	nc := *c
	nc.Servers = slices.Clone(c.Servers)
	nc.Options = maps.Clone(c.Options)
	nc.Meta = maps.Clone(c.Meta)
	nc.ChrootEnv = maps.Clone(c.ChrootEnv)
	nc.Reserved = c.Reserved.Copy()
	nc.NoHostUUID = pointer.Copy(c.NoHostUUID)
	nc.TemplateConfig = c.TemplateConfig.Copy()
	nc.ServerJoin = c.ServerJoin.Copy()
	nc.HostVolumes = helper.CopySlice(c.HostVolumes)
	nc.HostNetworks = helper.CopySlice(c.HostNetworks)
	nc.NomadServiceDiscovery = pointer.Copy(c.NomadServiceDiscovery)
	nc.Artifact = c.Artifact.Copy()
	nc.Drain = c.Drain.Copy()
	nc.Users = c.Users.Copy()
	nc.ExtraKeysHCL = slices.Clone(c.ExtraKeysHCL)
	return &nc
}

// ACLConfig is configuration specific to the ACL system
type ACLConfig struct {
	// Enabled controls if we are enforce and manage ACLs
	Enabled bool `hcl:"enabled"`

	// TokenTTL controls how long we cache ACL tokens. This controls
	// how stale they can be when we are enforcing policies. Defaults
	// to "30s". Reducing this impacts performance by forcing more
	// frequent resolution.
	TokenTTL    time.Duration
	TokenTTLHCL string `hcl:"token_ttl" json:"-"`

	// PolicyTTL controls how long we cache ACL policies. This controls
	// how stale they can be when we are enforcing policies. Defaults
	// to "30s". Reducing this impacts performance by forcing more
	// frequent resolution.
	PolicyTTL    time.Duration
	PolicyTTLHCL string `hcl:"policy_ttl" json:"-"`

	// RoleTTL controls how long we cache ACL roles. This controls how stale
	// they can be when we are enforcing policies. Defaults to "30s".
	// Reducing this impacts performance by forcing more frequent resolution.
	RoleTTL    time.Duration
	RoleTTLHCL string `hcl:"role_ttl" json:"-"`

	// ReplicationToken is used by servers to replicate tokens and policies
	// from the authoritative region. This must be a valid management token
	// within the authoritative region.
	ReplicationToken string `hcl:"replication_token"`

	// TokenMinExpirationTTL is used to enforce the lowest acceptable value for
	// ACL token expiration. This is used by the Nomad servers to validate ACL
	// tokens with an expiration value set upon creation.
	TokenMinExpirationTTL    time.Duration
	TokenMinExpirationTTLHCL string `hcl:"token_min_expiration_ttl" json:"-"`

	// TokenMaxExpirationTTL is used to enforce the highest acceptable value
	// for ACL token expiration. This is used by the Nomad servers to validate
	// ACL tokens with an expiration value set upon creation.
	TokenMaxExpirationTTL    time.Duration
	TokenMaxExpirationTTLHCL string `hcl:"token_max_expiration_ttl" json:"-"`

	// ExtraKeysHCL is used by hcl to surface unexpected keys
	ExtraKeysHCL []string `hcl:",unusedKeys" json:"-"`
}

func (a *ACLConfig) Copy() *ACLConfig {
	if a == nil {
		return nil
	}

	na := *a
	na.ExtraKeysHCL = slices.Clone(a.ExtraKeysHCL)
	return &na
}

// ServerConfig is configuration specific to the server mode
type ServerConfig struct {
	// Enabled controls if we are a server
	Enabled bool `hcl:"enabled"`

	// AuthoritativeRegion is used to control which region is treated as
	// the source of truth for global tokens and ACL policies.
	AuthoritativeRegion string `hcl:"authoritative_region"`

	// BootstrapExpect tries to automatically bootstrap the Nomad cluster,
	// by withholding peers until enough servers join.
	BootstrapExpect int `hcl:"bootstrap_expect"`

	// DataDir is the directory to store our state in
	DataDir string `hcl:"data_dir"`

	// ProtocolVersion is the protocol version to speak. This must be between
	// ProtocolVersionMin and ProtocolVersionMax.
	//
	// Deprecated: This has never been used and will emit a warning if nonzero.
	ProtocolVersion int `hcl:"protocol_version" json:"-"`

	// RaftProtocol is the Raft protocol version to speak. This must be from [1-3].
	RaftProtocol int `hcl:"raft_protocol"`

	// RaftMultiplier scales the Raft timing parameters
	RaftMultiplier *int `hcl:"raft_multiplier"`

	// NumSchedulers is the number of scheduler thread that are run.
	// This can be as many as one per core, or zero to disable this server
	// from doing any scheduling work.
	NumSchedulers *int `hcl:"num_schedulers"`

	// EnabledSchedulers controls the set of sub-schedulers that are
	// enabled for this server to handle. This will restrict the evaluations
	// that the workers dequeue for processing.
	EnabledSchedulers []string `hcl:"enabled_schedulers"`

	// NodeGCThreshold controls how "old" a node must be to be collected by GC.
	// Age is not the only requirement for a node to be GCed but the threshold
	// can be used to filter by age.
	NodeGCThreshold string `hcl:"node_gc_threshold"`

	// JobGCInterval controls how often we dispatch a job to GC jobs that are
	// available for garbage collection.
	JobGCInterval string `hcl:"job_gc_interval"`

	// JobGCThreshold controls how "old" a job must be to be collected by GC.
	// Age is not the only requirement for a Job to be GCed but the threshold
	// can be used to filter by age.
	JobGCThreshold string `hcl:"job_gc_threshold"`

	// EvalGCThreshold controls how "old" an eval must be to be collected by GC.
	// Age is not the only requirement for a eval to be GCed but the threshold
	// can be used to filter by age. Please note that batch job evaluations are
	// controlled by 'BatchEvalGCThreshold' instead.
	EvalGCThreshold string `hcl:"eval_gc_threshold"`

	// BatchEvalGCThreshold controls how "old" an evaluation must be to be eligible
	// for GC if the eval belongs to a batch job.
	BatchEvalGCThreshold string `hcl:"batch_eval_gc_threshold"`

	// DeploymentGCThreshold controls how "old" a deployment must be to be
	// collected by GC. Age is not the only requirement for a deployment to be
	// GCed but the threshold can be used to filter by age.
	DeploymentGCThreshold string `hcl:"deployment_gc_threshold"`

	// CSIVolumeClaimGCInterval is how often we dispatch a job to GC
	// volume claims.
	CSIVolumeClaimGCInterval string `hcl:"csi_volume_claim_gc_interval"`

	// CSIVolumeClaimGCThreshold controls how "old" a CSI volume must be to
	// have its claims collected by GC.	Age is not the only requirement for
	// a volume to be GCed but the threshold can be used to filter by age.
	CSIVolumeClaimGCThreshold string `hcl:"csi_volume_claim_gc_threshold"`

	// CSIPluginGCThreshold controls how "old" a CSI plugin must be to be
	// collected by GC. Age is not the only requirement for a plugin to be
	// GCed but the threshold can be used to filter by age.
	CSIPluginGCThreshold string `hcl:"csi_plugin_gc_threshold"`

	// ACLTokenGCThreshold controls how "old" an expired ACL token must be to
	// be collected by GC.
	ACLTokenGCThreshold string `hcl:"acl_token_gc_threshold"`

	// RootKeyGCInterval is how often we dispatch a job to GC
	// encryption key metadata
	RootKeyGCInterval string `hcl:"root_key_gc_interval"`

	// RootKeyGCThreshold is how "old" encryption key metadata must be
	// to be eligible for GC.
	RootKeyGCThreshold string `hcl:"root_key_gc_threshold"`

	// RootKeyRotationThreshold is how "old" an encryption key must be
	// before it is automatically rotated on the next garbage
	// collection interval.
	RootKeyRotationThreshold string `hcl:"root_key_rotation_threshold"`

	// HeartbeatGrace is the grace period beyond the TTL to account for network,
	// processing delays and clock skew before marking a node as "down".
	HeartbeatGrace    time.Duration
	HeartbeatGraceHCL string `hcl:"heartbeat_grace" json:"-"`

	// MinHeartbeatTTL is the minimum time between heartbeats. This is used as
	// a floor to prevent excessive updates.
	MinHeartbeatTTL    time.Duration
	MinHeartbeatTTLHCL string `hcl:"min_heartbeat_ttl" json:"-"`

	// MaxHeartbeatsPerSecond is the maximum target rate of heartbeats
	// being processed per second. This allows the TTL to be increased
	// to meet the target rate.
	MaxHeartbeatsPerSecond float64 `hcl:"max_heartbeats_per_second"`

	// FailoverHeartbeatTTL is the TTL applied to heartbeats after
	// a new leader is elected, since we no longer know the status
	// of all the heartbeats.
	FailoverHeartbeatTTL    time.Duration
	FailoverHeartbeatTTLHCL string `hcl:"failover_heartbeat_ttl" json:"-"`

	// StartJoin is a list of addresses to attempt to join when the
	// agent starts. If Serf is unable to communicate with any of these
	// addresses, then the agent will error and exit.
	// Deprecated in Nomad 0.10
	StartJoin []string `hcl:"start_join"`

	// RetryJoin is a list of addresses to join with retry enabled.
	// Deprecated in Nomad 0.10
	RetryJoin []string `hcl:"retry_join"`

	// RetryMaxAttempts specifies the maximum number of times to retry joining a
	// host on startup. This is useful for cases where we know the node will be
	// online eventually.
	// Deprecated in Nomad 0.10
	RetryMaxAttempts int `hcl:"retry_max"`

	// RetryInterval specifies the amount of time to wait in between join
	// attempts on agent start. The minimum allowed value is 1 second and
	// the default is 30s.
	// Deprecated in Nomad 0.10
	RetryInterval    time.Duration
	RetryIntervalHCL string `hcl:"retry_interval" json:"-"`

	// RejoinAfterLeave controls our interaction with the cluster after leave.
	// When set to false (default), a leave causes Nomad to not rejoin
	// the cluster until an explicit join is received. If this is set to
	// true, we ignore the leave, and rejoin the cluster on start.
	RejoinAfterLeave bool `hcl:"rejoin_after_leave"`

	// (Enterprise-only) NonVotingServer is whether this server will act as a
	// non-voting member of the cluster to help provide read scalability.
	NonVotingServer bool `hcl:"non_voting_server"`

	// (Enterprise-only) RedundancyZone is the redundancy zone to use for this server.
	RedundancyZone string `hcl:"redundancy_zone"`

	// (Enterprise-only) UpgradeVersion is the custom upgrade version to use when
	// performing upgrade migrations.
	UpgradeVersion string `hcl:"upgrade_version"`

	// Encryption key to use for the Serf communication
	EncryptKey string `hcl:"encrypt" json:"-"`

	// ServerJoin contains information that is used to attempt to join servers
	ServerJoin *ServerJoin `hcl:"server_join"`

	// DefaultSchedulerConfig configures the initial scheduler config to be persisted in Raft.
	// Once the cluster is bootstrapped, and Raft persists the config (from here or through API),
	// This value is ignored.
	DefaultSchedulerConfig *structs.SchedulerConfiguration `hcl:"default_scheduler_config"`

	// PlanRejectionTracker configures the node plan rejection tracker that
	// detects potentially bad nodes.
	PlanRejectionTracker *PlanRejectionTracker `hcl:"plan_rejection_tracker"`

	// EnableEventBroker configures whether this server's state store
	// will generate events for its event stream.
	EnableEventBroker *bool `hcl:"enable_event_broker"`

	// EventBufferSize configure the amount of events to be held in memory.
	// If EnableEventBroker is set to true, the minimum allowable value
	// for the EventBufferSize is 1.
	EventBufferSize *int `hcl:"event_buffer_size"`

	// LicensePath is the path to search for an enterprise license.
	LicensePath string `hcl:"license_path"`

	// LicenseEnv is the full enterprise license.  If NOMAD_LICENSE
	// is set, LicenseEnv will be set to the value at startup.
	LicenseEnv string

	// licenseAdditionalPublicKeys is an internal-only field used to
	// setup test licenses.
	licenseAdditionalPublicKeys []string

	// ExtraKeysHCL is used by hcl to surface unexpected keys
	ExtraKeysHCL []string `hcl:",unusedKeys" json:"-"`

	// Search configures UI search features.
	Search *Search `hcl:"search"`

	// DeploymentQueryRateLimit is in queries per second and is used by the
	// DeploymentWatcher to throttle the amount of simultaneously deployments
	DeploymentQueryRateLimit float64 `hcl:"deploy_query_rate_limit"`

	// RaftBoltConfig configures boltdb as used by raft.
	RaftBoltConfig *RaftBoltConfig `hcl:"raft_boltdb"`

	// RaftSnapshotThreshold controls how many outstanding logs there must be
	// before we perform a snapshot. This is to prevent excessive snapshotting by
	// replaying a small set of logs instead. The value passed here is the initial
	// setting used. This can be tuned during operation with a hot reload.
	RaftSnapshotThreshold *int `hcl:"raft_snapshot_threshold"`

	// RaftSnapshotInterval controls how often we check if we should perform a
	// snapshot. We randomly stagger between this value and 2x this value to avoid
	// the entire cluster from performing a snapshot at once. The value passed
	// here is the initial setting used. This can be tuned during operation with a
	// hot reload.
	RaftSnapshotInterval *string `hcl:"raft_snapshot_interval"`

	// RaftTrailingLogs controls how many logs are left after a snapshot. This is
	// used so that we can quickly replay logs on a follower instead of being
	// forced to send an entire snapshot. The value passed here is the initial
	// setting used. This can be tuned during operation using a hot reload.
	RaftTrailingLogs *int `hcl:"raft_trailing_logs"`

	// JobDefaultPriority is the default Job priority if not specified.
	JobDefaultPriority *int `hcl:"job_default_priority"`

	// JobMaxPriority is an upper bound on the Job priority.
	JobMaxPriority *int `hcl:"job_max_priority"`

	// JobMaxSourceSize limits the maximum size of a jobs source hcl/json
	// before being discarded automatically. If unset, the maximum size defaults
	// to 1 MB. If the value is zero, no job sources will be stored.
	JobMaxSourceSize *string `hcl:"job_max_source_size"`

	// JobTrackedVersions is the number of historic job versions that are kept.
	JobTrackedVersions *int `hcl:"job_tracked_versions"`

	// OIDCIssuer if set enables OIDC Discovery and uses this value as the
	// issuer. Third parties such as AWS IAM OIDC Provider expect the issuer to
	// be a publically accessible HTTPS URL signed by a trusted well-known CA.
	OIDCIssuer string `hcl:"oidc_issuer"`
}

func (s *ServerConfig) Copy() *ServerConfig {
	if s == nil {
		return nil
	}

	ns := *s
	ns.RaftMultiplier = pointer.Copy(s.RaftMultiplier)
	ns.NumSchedulers = pointer.Copy(s.NumSchedulers)
	ns.EnabledSchedulers = slices.Clone(s.EnabledSchedulers)
	ns.StartJoin = slices.Clone(s.StartJoin)
	ns.RetryJoin = slices.Clone(s.RetryJoin)
	ns.ServerJoin = s.ServerJoin.Copy()
	ns.DefaultSchedulerConfig = s.DefaultSchedulerConfig.Copy()
	ns.PlanRejectionTracker = s.PlanRejectionTracker.Copy()
	ns.EnableEventBroker = pointer.Copy(s.EnableEventBroker)
	ns.EventBufferSize = pointer.Copy(s.EventBufferSize)
	ns.JobMaxSourceSize = pointer.Copy(s.JobMaxSourceSize)
	ns.licenseAdditionalPublicKeys = slices.Clone(s.licenseAdditionalPublicKeys)
	ns.ExtraKeysHCL = slices.Clone(s.ExtraKeysHCL)
	ns.Search = s.Search.Copy()
	ns.RaftBoltConfig = s.RaftBoltConfig.Copy()
	ns.RaftSnapshotInterval = pointer.Copy(s.RaftSnapshotInterval)
	ns.RaftSnapshotThreshold = pointer.Copy(s.RaftSnapshotThreshold)
	ns.RaftTrailingLogs = pointer.Copy(s.RaftTrailingLogs)
	ns.JobDefaultPriority = pointer.Copy(s.JobDefaultPriority)
	ns.JobMaxPriority = pointer.Copy(s.JobMaxPriority)
	ns.JobTrackedVersions = pointer.Copy(s.JobTrackedVersions)
	return &ns
}

// RaftBoltConfig is used in servers to configure parameters of the boltdb
// used for raft consensus.
type RaftBoltConfig struct {
	// NoFreelistSync toggles whether the underlying raft storage should sync its
	// freelist to disk within the bolt .db file. When disabled, IO performance
	// will be improved but at the expense of longer startup times.
	//
	// Default: false.
	NoFreelistSync bool `hcl:"no_freelist_sync"`
}

func (r *RaftBoltConfig) Copy() *RaftBoltConfig {
	if r == nil {
		return nil
	}

	nr := *r
	return &nr
}

// PlanRejectionTracker is used in servers to configure the plan rejection
// tracker.
type PlanRejectionTracker struct {
	// Enabled controls if the plan rejection tracker is active or not.
	Enabled *bool `hcl:"enabled"`

	// NodeThreshold is the number of times a node can have plan rejections
	// before it is marked as ineligible.
	NodeThreshold int `hcl:"node_threshold"`

	// NodeWindow is the time window used to track active plan rejections for
	// nodes.
	NodeWindow    time.Duration
	NodeWindowHCL string `hcl:"node_window" json:"-"`

	// ExtraKeysHCL is used by hcl to surface unexpected keys
	ExtraKeysHCL []string `hcl:",unusedKeys" json:"-"`
}

func (p *PlanRejectionTracker) Copy() *PlanRejectionTracker {
	if p == nil {
		return nil
	}

	np := *p
	np.Enabled = pointer.Copy(p.Enabled)
	np.ExtraKeysHCL = slices.Clone(p.ExtraKeysHCL)
	return &np
}

func (p *PlanRejectionTracker) Merge(b *PlanRejectionTracker) *PlanRejectionTracker {
	if p == nil {
		return b
	}

	result := *p

	if b == nil {
		return &result
	}

	if b.Enabled != nil {
		result.Enabled = b.Enabled
	}

	if b.NodeThreshold != 0 {
		result.NodeThreshold = b.NodeThreshold
	}

	if b.NodeWindow != 0 {
		result.NodeWindow = b.NodeWindow
	}
	if b.NodeWindowHCL != "" {
		result.NodeWindowHCL = b.NodeWindowHCL
	}
	return &result
}

// Search is used in servers to configure search API options.
type Search struct {
	// FuzzyEnabled toggles whether the FuzzySearch API is enabled. If not
	// enabled, requests to /v1/search/fuzzy will reply with a 404 response code.
	//
	// Default: enabled.
	FuzzyEnabled bool `hcl:"fuzzy_enabled"`

	// LimitQuery limits the number of objects searched in the FuzzySearch API.
	// The results are indicated as truncated if the limit is reached.
	//
	// Lowering this value can reduce resource consumption of Nomad server when
	// the FuzzySearch API is enabled.
	//
	// Default value: 20.
	LimitQuery int `hcl:"limit_query"`

	// LimitResults limits the number of results provided by the FuzzySearch API.
	// The results are indicated as truncate if the limit is reached.
	//
	// Lowering this value can reduce resource consumption of Nomad server per
	// fuzzy search request when the FuzzySearch API is enabled.
	//
	// Default value: 100.
	LimitResults int `hcl:"limit_results"`

	// MinTermLength is the minimum length of Text required before the FuzzySearch
	// API will return results.
	//
	// Increasing this value can avoid resource consumption on Nomad server by
	// reducing searches with less meaningful results.
	//
	// Default value: 2.
	MinTermLength int `hcl:"min_term_length"`
}

func (s *Search) Copy() *Search {
	if s == nil {
		return nil
	}

	ns := *s
	return &ns
}

// ServerJoin is used in both clients and servers to bootstrap connections to
// servers
type ServerJoin struct {
	// StartJoin is a list of addresses to attempt to join when the
	// agent starts. If Serf is unable to communicate with any of these
	// addresses, then the agent will error and exit.
	StartJoin []string `hcl:"start_join"`

	// RetryJoin is a list of addresses to join with retry enabled, or a single
	// value to find multiple servers using go-discover syntax.
	RetryJoin []string `hcl:"retry_join"`

	// RetryMaxAttempts specifies the maximum number of times to retry joining a
	// host on startup. This is useful for cases where we know the node will be
	// online eventually.
	RetryMaxAttempts int `hcl:"retry_max"`

	// RetryInterval specifies the amount of time to wait in between join
	// attempts on agent start. The minimum allowed value is 1 second and
	// the default is 30s.
	RetryInterval    time.Duration
	RetryIntervalHCL string `hcl:"retry_interval" json:"-"`

	// ExtraKeysHCL is used by hcl to surface unexpected keys
	ExtraKeysHCL []string `hcl:",unusedKeys" json:"-"`
}

func (s *ServerJoin) Copy() *ServerJoin {
	if s == nil {
		return nil
	}

	ns := *s
	ns.StartJoin = slices.Clone(s.StartJoin)
	ns.RetryJoin = slices.Clone(s.RetryJoin)
	ns.ExtraKeysHCL = slices.Clone(s.ExtraKeysHCL)
	return &ns
}

func (s *ServerJoin) Merge(b *ServerJoin) *ServerJoin {
	if s == nil {
		return b
	}

	result := *s

	if b == nil {
		return &result
	}

	if len(b.StartJoin) != 0 {
		result.StartJoin = b.StartJoin
	}
	if len(b.RetryJoin) != 0 {
		result.RetryJoin = b.RetryJoin
	}
	if b.RetryMaxAttempts != 0 {
		result.RetryMaxAttempts = b.RetryMaxAttempts
	}
	if b.RetryInterval != 0 {
		result.RetryInterval = b.RetryInterval
	}

	return &result
}

// EncryptBytes returns the encryption key configured.
func (s *ServerConfig) EncryptBytes() ([]byte, error) {
	return base64.StdEncoding.DecodeString(s.EncryptKey)
}

// Telemetry is the telemetry configuration for the server
type Telemetry struct {

	// InMemoryCollectionInterval configures the in-memory sink collection
	// interval. This sink is always configured and backs the JSON metrics API
	// endpoint. This option is particularly useful for debugging or
	// development.
	InMemoryCollectionInterval string        `hcl:"in_memory_collection_interval"`
	inMemoryCollectionInterval time.Duration `hcl:"-"`

	// InMemoryRetentionPeriod configures the in-memory sink retention period
	// This sink is always configured and backs the JSON metrics API endpoint.
	// This option is particularly useful for debugging or development.
	InMemoryRetentionPeriod string        `hcl:"in_memory_retention_period"`
	inMemoryRetentionPeriod time.Duration `hcl:"-"`

	StatsiteAddr                  string        `hcl:"statsite_address"`
	StatsdAddr                    string        `hcl:"statsd_address"`
	DataDogAddr                   string        `hcl:"datadog_address"`
	DataDogTags                   []string      `hcl:"datadog_tags"`
	PrometheusMetrics             bool          `hcl:"prometheus_metrics"`
	DisableHostname               bool          `hcl:"disable_hostname"`
	UseNodeName                   bool          `hcl:"use_node_name"`
	CollectionInterval            string        `hcl:"collection_interval"`
	collectionInterval            time.Duration `hcl:"-"`
	PublishAllocationMetrics      bool          `hcl:"publish_allocation_metrics"`
	PublishNodeMetrics            bool          `hcl:"publish_node_metrics"`
	IncludeAllocMetadataInMetrics bool          `hcl:"include_alloc_metadata_in_metrics"`
	AllowedMetadataKeysInMetrics  []string      `hcl:"allowed_metadata_keys_in_metrics"`

	// PrefixFilter allows for filtering out metrics from being collected
	PrefixFilter []string `hcl:"prefix_filter"`

	// FilterDefault controls whether to allow metrics that have not been specified
	// by the filter
	FilterDefault *bool `hcl:"filter_default"`

	// DisableDispatchedJobSummaryMetrics allows ignoring dispatched jobs when
	// publishing Job summary metrics. This is useful in environments that produce
	// high numbers of single count dispatch jobs as the metrics for each take up
	// a small memory overhead.
	DisableDispatchedJobSummaryMetrics bool `hcl:"disable_dispatched_job_summary_metrics"`

	// DisableQuotaUtilizationMetrics allows to disable publishing of quota
	// utilization metrics
	DisableQuotaUtilizationMetrics bool `hcl:"disable_quota_utilization_metrics"`

	// DisableRPCRateMetricsLabels drops the label for the identity of the
	// requester when publishing metrics on RPC rate on the server. This may be
	// useful to control metrics collection costs in environments where request
	// rate is well-controlled but cardinality of requesters is high.
	DisableRPCRateMetricsLabels bool `hcl:"disable_rpc_rate_metrics_labels"`

	// Circonus: see https://github.com/circonus-labs/circonus-gometrics
	// for more details on the various configuration options.
	// Valid configuration combinations:
	//    - CirconusAPIToken
	//      metric management enabled (search for existing check or create a new one)
	//    - CirconusSubmissionUrl
	//      metric management disabled (use check with specified submission_url,
	//      broker must be using a public SSL certificate)
	//    - CirconusAPIToken + CirconusCheckSubmissionURL
	//      metric management enabled (use check with specified submission_url)
	//    - CirconusAPIToken + CirconusCheckID
	//      metric management enabled (use check with specified id)

	// CirconusAPIToken is a valid API Token used to create/manage check. If provided,
	// metric management is enabled.
	// Default: none
	CirconusAPIToken string `hcl:"circonus_api_token"`
	// CirconusAPIApp is an app name associated with API token.
	// Default: "nomad"
	CirconusAPIApp string `hcl:"circonus_api_app"`
	// CirconusAPIURL is the base URL to use for contacting the Circonus API.
	// Default: "https://api.circonus.com/v2"
	CirconusAPIURL string `hcl:"circonus_api_url"`
	// CirconusSubmissionInterval is the interval at which metrics are submitted to Circonus.
	// Default: 10s
	CirconusSubmissionInterval string `hcl:"circonus_submission_interval"`
	// CirconusCheckSubmissionURL is the check.config.submission_url field from a
	// previously created HTTPTRAP check.
	// Default: none
	CirconusCheckSubmissionURL string `hcl:"circonus_submission_url"`
	// CirconusCheckID is the check id (not check bundle id) from a previously created
	// HTTPTRAP check. The numeric portion of the check._cid field.
	// Default: none
	CirconusCheckID string `hcl:"circonus_check_id"`
	// CirconusCheckForceMetricActivation will force enabling metrics, as they are encountered,
	// if the metric already exists and is NOT active. If check management is enabled, the default
	// behavior is to add new metrics as they are encountered. If the metric already exists in the
	// check, it will *NOT* be activated. This setting overrides that behavior.
	// Default: "false"
	CirconusCheckForceMetricActivation string `hcl:"circonus_check_force_metric_activation"`
	// CirconusCheckInstanceID serves to uniquely identify the metrics coming from this "instance".
	// It can be used to maintain metric continuity with transient or ephemeral instances as
	// they move around within an infrastructure.
	// Default: hostname:app
	CirconusCheckInstanceID string `hcl:"circonus_check_instance_id"`
	// CirconusCheckSearchTag is a special tag which, when coupled with the instance id, helps to
	// narrow down the search results when neither a Submission URL or Check ID is provided.
	// Default: service:app (e.g. service:nomad)
	CirconusCheckSearchTag string `hcl:"circonus_check_search_tag"`
	// CirconusCheckTags is a comma separated list of tags to apply to the check. Note that
	// the value of CirconusCheckSearchTag will always be added to the check.
	// Default: none
	CirconusCheckTags string `hcl:"circonus_check_tags"`
	// CirconusCheckDisplayName is the name for the check which will be displayed in the Circonus UI.
	// Default: value of CirconusCheckInstanceID
	CirconusCheckDisplayName string `hcl:"circonus_check_display_name"`
	// CirconusBrokerID is an explicit broker to use when creating a new check. The numeric portion
	// of broker._cid. If metric management is enabled and neither a Submission URL nor Check ID
	// is provided, an attempt will be made to search for an existing check using Instance ID and
	// Search Tag. If one is not found, a new HTTPTRAP check will be created.
	// Default: use Select Tag if provided, otherwise, a random Enterprise Broker associated
	// with the specified API token or the default Circonus Broker.
	// Default: none
	CirconusBrokerID string `hcl:"circonus_broker_id"`
	// CirconusBrokerSelectTag is a special tag which will be used to select a broker when
	// a Broker ID is not provided. The best use of this is to as a hint for which broker
	// should be used based on *where* this particular instance is running.
	// (e.g. a specific geo location or datacenter, dc:sfo)
	// Default: none
	CirconusBrokerSelectTag string `hcl:"circonus_broker_select_tag"`

	// ExtraKeysHCL is used by hcl to surface unexpected keys
	ExtraKeysHCL []string `hcl:",unusedKeys" json:"-"`
}

func (t *Telemetry) Copy() *Telemetry {
	if t == nil {
		return nil
	}

	nt := *t
	nt.DataDogTags = slices.Clone(t.DataDogTags)
	nt.PrefixFilter = slices.Clone(t.PrefixFilter)
	nt.FilterDefault = pointer.Copy(t.FilterDefault)
	nt.ExtraKeysHCL = slices.Clone(t.ExtraKeysHCL)
	return &nt
}

// PrefixFilters parses the PrefixFilter field and returns a list of allowed and blocked filters
func (t *Telemetry) PrefixFilters() (allowed, blocked []string, err error) {
	for _, rule := range t.PrefixFilter {
		if rule == "" {
			continue
		}
		switch rule[0] {
		case '+':
			allowed = append(allowed, rule[1:])
		case '-':
			blocked = append(blocked, rule[1:])
		default:
			return nil, nil, fmt.Errorf("Filter rule must begin with either '+' or '-': %q", rule)
		}
	}
	return allowed, blocked, nil
}

// Validate the telemetry configuration options. These are used by the agent,
// regardless of mode, so can live here rather than a structs package. It is
// safe to call, without checking whether the config object is nil first.
func (t *Telemetry) Validate() error {
	if t == nil {
		return nil
	}

	// Ensure we have durations that are greater than zero.
	if t.inMemoryCollectionInterval <= 0 {
		return errors.New("telemetry in-memory collection interval must be greater than zero")
	}
	if t.inMemoryRetentionPeriod <= 0 {
		return errors.New("telemetry in-memory retention period must be greater than zero")
	}

	// Ensure the in-memory durations do not conflict.
	if t.inMemoryCollectionInterval > t.inMemoryRetentionPeriod {
		return errors.New("telemetry in-memory collection interval cannot be greater than retention period")
	}

	return nil
}

// Ports encapsulates the various ports we bind to for network services. If any
// are not specified then the defaults are used instead.
type Ports struct {
	HTTP int `hcl:"http"`
	RPC  int `hcl:"rpc"`
	Serf int `hcl:"serf"`
	// ExtraKeysHCL is used by hcl to surface unexpected keys
	ExtraKeysHCL []string `hcl:",unusedKeys" json:"-"`
}

func (p *Ports) Copy() *Ports {
	if p == nil {
		return nil
	}

	np := *p
	np.ExtraKeysHCL = slices.Clone(p.ExtraKeysHCL)
	return &np
}

// Addresses encapsulates all of the addresses we bind to for various
// network services. Everything is optional and defaults to BindAddr.
type Addresses struct {
	HTTP string `hcl:"http"`
	RPC  string `hcl:"rpc"`
	Serf string `hcl:"serf"`
	// ExtraKeysHCL is used by hcl to surface unexpected keys
	ExtraKeysHCL []string `hcl:",unusedKeys" json:"-"`
}

func (a *Addresses) Copy() *Addresses {
	if a == nil {
		return nil
	}

	na := *a
	na.ExtraKeysHCL = slices.Clone(a.ExtraKeysHCL)
	return &na
}

// NormalizedAddrs is used to control the addresses we advertise out for
// different network services. All are optional and default to BindAddr and
// their default Port.
type NormalizedAddrs struct {
	HTTP []string
	RPC  string
	Serf string
}

func (n *NormalizedAddrs) Copy() *NormalizedAddrs {
	if n == nil {
		return nil
	}

	nn := *n
	nn.HTTP = slices.Clone(n.HTTP)
	return &nn
}

// AdvertiseAddrs is used to control the addresses we advertise out for
// different network services. All are optional and default to BindAddr and
// their default Port.
type AdvertiseAddrs struct {
	HTTP string `hcl:"http"`
	RPC  string `hcl:"rpc"`
	Serf string `hcl:"serf"`
	// ExtraKeysHCL is used by hcl to surface unexpected keys
	ExtraKeysHCL []string `hcl:",unusedKeys" json:"-"`
}

func (a *AdvertiseAddrs) Copy() *AdvertiseAddrs {
	if a == nil {
		return nil
	}

	na := *a
	na.ExtraKeysHCL = slices.Clone(a.ExtraKeysHCL)
	return &na
}

type Resources struct {
	CPU           int    `hcl:"cpu"`
	MemoryMB      int    `hcl:"memory"`
	DiskMB        int    `hcl:"disk"`
	ReservedPorts string `hcl:"reserved_ports"`
	Cores         string `hcl:"cores"`
	// ExtraKeysHCL is used by hcl to surface unexpected keys
	ExtraKeysHCL []string `hcl:",unusedKeys" json:"-"`
}

func (r *Resources) Copy() *Resources {
	if r == nil {
		return nil
	}

	nr := *r
	nr.ExtraKeysHCL = slices.Clone(r.ExtraKeysHCL)
	return &nr
}

// devModeConfig holds the config for the -dev and -dev-connect flags
type devModeConfig struct {
	// mode flags are set at the command line via -dev and -dev-connect
	defaultMode bool
	connectMode bool
	consulMode  bool
	vaultMode   bool

	bindAddr string
	iface    string
}

func (mode *devModeConfig) enabled() bool {
	return mode.defaultMode || mode.connectMode ||
		mode.consulMode || mode.vaultMode
}

func (mode *devModeConfig) validate() error {
	if mode.connectMode {
		if runtime.GOOS != "linux" {
			// strictly speaking -dev-connect only binds to the
			// non-localhost interface, but given its purpose
			// is to support a feature with network namespaces
			// we'll return an error here rather than let the agent
			// come up and fail unexpectedly to run jobs
			return fmt.Errorf("-dev-connect is only supported on linux.")
		}
		u, err := users.Current()
		if err != nil {
			return fmt.Errorf(
				"-dev-connect uses network namespaces and is only supported for root: %v", err)
		}
		if u.Uid != "0" {
			return fmt.Errorf(
				"-dev-connect uses network namespaces and is only supported for root.")
		}
		// Ensure Consul is on PATH
		if _, err := exec.LookPath("consul"); err != nil {
			return fmt.Errorf("-dev-connect requires a 'consul' binary in Nomad's $PATH")
		}
	}
	return nil
}

func (mode *devModeConfig) networkConfig() error {
	if runtime.GOOS == "windows" {
		mode.bindAddr = "127.0.0.1"
		mode.iface = "Loopback Pseudo-Interface 1"
		return nil
	}
	if runtime.GOOS == "darwin" {
		mode.bindAddr = "127.0.0.1"
		mode.iface = "lo0"
		return nil
	}
	if mode != nil && mode.connectMode {
		// if we hit either of the errors here we're in a weird situation
		// where syscalls to get the list of network interfaces are failing.
		// rather than throwing errors, we'll fall back to the default.
		ifAddrs, err := sockaddr.GetDefaultInterfaces()
		errMsg := "-dev=connect uses network namespaces: %v"
		if err != nil {
			return fmt.Errorf(errMsg, err)
		}
		if len(ifAddrs) < 1 {
			return fmt.Errorf(errMsg, "could not find public network interface")
		}
		iface := ifAddrs[0].Name
		mode.iface = iface
		mode.bindAddr = "0.0.0.0" // allows CLI to "just work"
		return nil
	}
	mode.bindAddr = "127.0.0.1"
	mode.iface = "lo"
	return nil
}

// DevConfig is a Config that is used for dev mode of Nomad.
func DevConfig(mode *devModeConfig) *Config {
	if mode == nil {
		mode = &devModeConfig{defaultMode: true}
		mode.networkConfig()
	}
	conf := DefaultConfig()
	conf.BindAddr = mode.bindAddr
	conf.LogLevel = "DEBUG"
	conf.Client.Enabled = true
	conf.Server.Enabled = true
	conf.DevMode = true
	conf.Server.BootstrapExpect = 1
	conf.EnableDebug = true
	conf.DisableAnonymousSignature = true
	conf.defaultConsul().AutoAdvertise = pointer.Of(true)
	conf.Client.NetworkInterface = mode.iface
	conf.Client.Options = map[string]string{
		"driver.raw_exec.enable": "true",
		"driver.docker.volumes":  "true",
	}
	conf.Client.GCInterval = 10 * time.Minute
	conf.Client.GCDiskUsageThreshold = 99
	conf.Client.GCInodeUsageThreshold = 99
	conf.Client.GCMaxAllocs = 50
	conf.Client.Options[fingerprint.TightenNetworkTimeoutsConfig] = "true"
	conf.Client.BindWildcardDefaultHostNetwork = true
	conf.Client.NomadServiceDiscovery = pointer.Of(true)
	conf.Client.ReservableCores = "" // inherit all the cores
	conf.Telemetry.PrometheusMetrics = true
	conf.Telemetry.PublishAllocationMetrics = true
	conf.Telemetry.PublishNodeMetrics = true
	conf.Telemetry.IncludeAllocMetadataInMetrics = true
	conf.Telemetry.AllowedMetadataKeysInMetrics = []string{}

	if mode.consulMode {
		conf.Consuls[0].ServiceIdentity = &config.WorkloadIdentityConfig{
			Audience: []string{"consul.io"},
			TTL:      pointer.Of(time.Hour),
		}
		conf.Consuls[0].TaskIdentity = &config.WorkloadIdentityConfig{
			Audience: []string{"consul.io"},
			TTL:      pointer.Of(time.Hour),
		}
	}

	if mode.vaultMode {
		conf.Vaults[0].Enabled = pointer.Of(true)
		conf.Vaults[0].Addr = "http://localhost:8200"
		conf.Vaults[0].DefaultIdentity = &config.WorkloadIdentityConfig{
			Audience: []string{"vault.io"},
			TTL:      pointer.Of(time.Hour),
		}
	}
	return conf
}

// DefaultConfig is the baseline configuration for Nomad.
func DefaultConfig() *Config {
	cfg := &Config{
		LogLevel:   "INFO",
		Region:     "global",
		Datacenter: "dc1",
		BindAddr:   "0.0.0.0",
		Ports: &Ports{
			HTTP: 4646,
			RPC:  4647,
			Serf: 4648,
		},
		Addresses:      &Addresses{},
		AdvertiseAddrs: &AdvertiseAddrs{},
		Consuls:        []*config.ConsulConfig{config.DefaultConsulConfig()},
		Vaults:         []*config.VaultConfig{config.DefaultVaultConfig()},
		UI:             config.DefaultUIConfig(),
		Client: &ClientConfig{
			Enabled:               false,
			NodePool:              structs.NodePoolDefault,
			MaxKillTimeout:        "30s",
			ClientMinPort:         14000,
			ClientMaxPort:         14512,
			MinDynamicPort:        20000,
			MaxDynamicPort:        32000,
			Reserved:              &Resources{},
			GCInterval:            1 * time.Minute,
			GCParallelDestroys:    2,
			GCDiskUsageThreshold:  80,
			GCInodeUsageThreshold: 70,
			GCMaxAllocs:           50,
			NoHostUUID:            pointer.Of(true),
			DisableRemoteExec:     false,
			ServerJoin: &ServerJoin{
				RetryJoin:        []string{},
				RetryInterval:    30 * time.Second,
				RetryMaxAttempts: 0,
			},
			TemplateConfig:                 client.DefaultTemplateConfig(),
			BindWildcardDefaultHostNetwork: true,
			CNIPath:                        "/opt/cni/bin",
			CNIConfigDir:                   "/opt/cni/config",
			NomadServiceDiscovery:          pointer.Of(true),
			Artifact:                       config.DefaultArtifactConfig(),
			Drain:                          nil,
			Users:                          config.DefaultUsersConfig(),
		},
		Server: &ServerConfig{
			Enabled:           false,
			EnableEventBroker: pointer.Of(true),
			EventBufferSize:   pointer.Of(100),
			RaftProtocol:      3,
			StartJoin:         []string{},
			PlanRejectionTracker: &PlanRejectionTracker{
				Enabled:       pointer.Of(false),
				NodeThreshold: 100,
				NodeWindow:    5 * time.Minute,
			},
			ServerJoin: &ServerJoin{
				RetryJoin:        []string{},
				RetryInterval:    30 * time.Second,
				RetryMaxAttempts: 0,
			},
			Search: &Search{
				FuzzyEnabled:  true,
				LimitQuery:    20,
				LimitResults:  100,
				MinTermLength: 2,
			},
			JobMaxSourceSize:   pointer.Of("1M"),
			JobTrackedVersions: pointer.Of(structs.JobDefaultTrackedVersions),
		},
		ACL: &ACLConfig{
			Enabled:   false,
			TokenTTL:  30 * time.Second,
			PolicyTTL: 30 * time.Second,
			RoleTTL:   30 * time.Second,
		},
		SyslogFacility: "LOCAL0",
		Telemetry: &Telemetry{
			InMemoryCollectionInterval: "10s",
			inMemoryCollectionInterval: 10 * time.Second,
			InMemoryRetentionPeriod:    "1m",
			inMemoryRetentionPeriod:    1 * time.Minute,
			CollectionInterval:         "1s",
			collectionInterval:         1 * time.Second,
		},
		TLSConfig:          &config.TLSConfig{},
		Sentinel:           &config.SentinelConfig{},
		Version:            version.GetVersion(),
		Autopilot:          config.DefaultAutopilotConfig(),
		Audit:              &config.AuditConfig{},
		DisableUpdateCheck: pointer.Of(false),
		Limits:             config.DefaultLimits(),
		Reporting:          config.DefaultReporting(),
		KEKProviders:       []*structs.KEKProviderConfig{},
	}

	return cfg
}

// Listener can be used to get a new listener using a custom bind address.
// If the bind provided address is empty, the BindAddr is used instead.
func (c *Config) Listener(proto, addr string, port int) (net.Listener, error) {
	if addr == "" {
		addr = c.BindAddr
	}

	// Do our own range check to avoid bugs in package net.
	//
	//   golang.org/issue/11715
	//   golang.org/issue/13447
	//
	// Both of the above bugs were fixed by golang.org/cl/12447 which will be
	// included in Go 1.6. The error returned below is the same as what Go 1.6
	// will return.
	if 0 > port || port > 65535 {
		return nil, &net.OpError{
			Op:  "listen",
			Net: proto,
			Err: &net.AddrError{Err: "invalid port", Addr: fmt.Sprint(port)},
		}
	}
	return net.Listen(proto, net.JoinHostPort(addr, strconv.Itoa(port)))
}

// Merge merges two configurations.
func (c *Config) Merge(b *Config) *Config {
	result := *c

	if b.Region != "" {
		result.Region = b.Region
	}
	if b.Datacenter != "" {
		result.Datacenter = b.Datacenter
	}
	if b.NodeName != "" {
		result.NodeName = b.NodeName
	}
	if b.DataDir != "" {
		result.DataDir = b.DataDir
	}
	if b.PluginDir != "" {
		result.PluginDir = b.PluginDir
	}
	if b.LogLevel != "" {
		result.LogLevel = b.LogLevel
	}
	if b.LogJson {
		result.LogJson = true
	}
	if b.LogFile != "" {
		result.LogFile = b.LogFile
	}
	if b.LogIncludeLocation {
		result.LogIncludeLocation = true
	}
	if b.LogRotateDuration != "" {
		result.LogRotateDuration = b.LogRotateDuration
	}
	if b.LogRotateBytes != 0 {
		result.LogRotateBytes = b.LogRotateBytes
	}
	if b.LogRotateMaxFiles != 0 {
		result.LogRotateMaxFiles = b.LogRotateMaxFiles
	}
	if b.BindAddr != "" {
		result.BindAddr = b.BindAddr
	}
	if b.EnableDebug {
		result.EnableDebug = true
	}
	if b.LeaveOnInt {
		result.LeaveOnInt = true
	}
	if b.LeaveOnTerm {
		result.LeaveOnTerm = true
	}
	if b.EnableSyslog {
		result.EnableSyslog = true
	}
	if b.SyslogFacility != "" {
		result.SyslogFacility = b.SyslogFacility
	}
	if b.DisableUpdateCheck != nil {
		result.DisableUpdateCheck = pointer.Of(*b.DisableUpdateCheck)
	}
	if b.DisableAnonymousSignature {
		result.DisableAnonymousSignature = true
	}

	// Apply the telemetry config
	if result.Telemetry == nil && b.Telemetry != nil {
		telemetry := *b.Telemetry
		result.Telemetry = &telemetry
	} else if b.Telemetry != nil {
		result.Telemetry = result.Telemetry.Merge(b.Telemetry)
	}

	// Apply the Reporting Config
	if result.Reporting == nil && b.Reporting != nil {
		result.Reporting = b.Reporting.Copy()
	} else if b.Reporting != nil {
		result.Reporting = result.Reporting.Merge(b.Reporting)
	}

	// Apply the TLS Config
	if result.TLSConfig == nil && b.TLSConfig != nil {
		result.TLSConfig = b.TLSConfig.Copy()
	} else if b.TLSConfig != nil {
		result.TLSConfig = result.TLSConfig.Merge(b.TLSConfig)
	}

	// Apply the client config
	if result.Client == nil && b.Client != nil {
		client := *b.Client
		result.Client = &client
	} else if b.Client != nil {
		result.Client = result.Client.Merge(b.Client)
	}

	// Apply the server config
	if result.Server == nil && b.Server != nil {
		server := *b.Server
		result.Server = &server
	} else if b.Server != nil {
		result.Server = result.Server.Merge(b.Server)
	}

	// Apply the acl config
	if result.ACL == nil && b.ACL != nil {
		server := *b.ACL
		result.ACL = &server
	} else if b.ACL != nil {
		result.ACL = result.ACL.Merge(b.ACL)
	}

	// Apply the Audit config
	if result.Audit == nil && b.Audit != nil {
		audit := *b.Audit
		result.Audit = &audit
	} else if b.ACL != nil {
		result.Audit = result.Audit.Merge(b.Audit)
	}

	// Apply the ports config
	if result.Ports == nil && b.Ports != nil {
		ports := *b.Ports
		result.Ports = &ports
	} else if b.Ports != nil {
		result.Ports = result.Ports.Merge(b.Ports)
	}

	// Apply the address config
	if result.Addresses == nil && b.Addresses != nil {
		addrs := *b.Addresses
		result.Addresses = &addrs
	} else if b.Addresses != nil {
		result.Addresses = result.Addresses.Merge(b.Addresses)
	}

	// Apply the advertise addrs config
	if result.AdvertiseAddrs == nil && b.AdvertiseAddrs != nil {
		advertise := *b.AdvertiseAddrs
		result.AdvertiseAddrs = &advertise
	} else if b.AdvertiseAddrs != nil {
		result.AdvertiseAddrs = result.AdvertiseAddrs.Merge(b.AdvertiseAddrs)
	}

	// Apply the Consul Configurations
	result.Consuls = mergeConsulConfigs(result.Consuls, b.Consuls)

	// Apply the Vault Configurations
	result.Vaults = mergeVaultConfigs(result.Vaults, b.Vaults)

	// Apply the UI Configuration
	if result.UI == nil && b.UI != nil {
		uiConfig := *b.UI
		result.UI = &uiConfig
	} else if b.UI != nil {
		result.UI = result.UI.Merge(b.UI)
	}

	// Apply the sentinel config
	if result.Sentinel == nil && b.Sentinel != nil {
		server := *b.Sentinel
		result.Sentinel = &server
	} else if b.Sentinel != nil {
		result.Sentinel = result.Sentinel.Merge(b.Sentinel)
	}

	if result.Autopilot == nil && b.Autopilot != nil {
		autopilot := *b.Autopilot
		result.Autopilot = &autopilot
	} else if b.Autopilot != nil {
		result.Autopilot = result.Autopilot.Merge(b.Autopilot)
	}

	if len(result.Plugins) == 0 && len(b.Plugins) != 0 {
		copy := make([]*config.PluginConfig, len(b.Plugins))
		for i, v := range b.Plugins {
			copy[i] = v.Copy()
		}
		result.Plugins = copy
	} else if len(b.Plugins) != 0 {
		result.Plugins = config.PluginConfigSetMerge(result.Plugins, b.Plugins)
	}

	// Merge config files lists
	result.Files = append(result.Files, b.Files...)

	// Add the http API response header map values
	if result.HTTPAPIResponseHeaders == nil {
		result.HTTPAPIResponseHeaders = make(map[string]string)
	}
	for k, v := range b.HTTPAPIResponseHeaders {
		result.HTTPAPIResponseHeaders[k] = v
	}

	result.Limits = c.Limits.Merge(b.Limits)

	result.KEKProviders = mergeKEKProviderConfigs(result.KEKProviders, b.KEKProviders)

	return &result
}

// mergeVaultConfigs takes two slices of VaultConfig and returns a slice
// containing the superset of all configurations, and with every configuration
// with the same name merged
func mergeVaultConfigs(left, right []*config.VaultConfig) []*config.VaultConfig {
	results := []*config.VaultConfig{}

	doMerge := func(dstConfigs, srcConfigs []*config.VaultConfig) []*config.VaultConfig {
		for _, src := range srcConfigs {
			if src.Name == "" {
				src.Name = "default"
			}
			var found bool
			for i, dst := range dstConfigs {
				if dst.Name == src.Name {
					dstConfigs[i] = dst.Merge(src)
					found = true
					break
				}
			}
			if !found {
				dstConfigs = append(dstConfigs, config.DefaultVaultConfig().Merge(src))
			}
		}
		return dstConfigs
	}

	results = doMerge(results, left)
	results = doMerge(results, right)
	return results
}

// mergeConsulConfigs takes two slices of ConsulConfig and returns a slice
// containing the superset of all configurations, and with every configuration
// with the same name merged
func mergeConsulConfigs(left, right []*config.ConsulConfig) []*config.ConsulConfig {
	if len(left) == 0 {
		return right
	}
	if len(right) == 0 {
		return left
	}
	results := []*config.ConsulConfig{}

	doMerge := func(dstConfigs, srcConfigs []*config.ConsulConfig) []*config.ConsulConfig {
		for _, src := range srcConfigs {
			if src.Name == "" {
				src.Name = "default"
			}
			var found bool
			for i, dst := range dstConfigs {
				if dst.Name == src.Name {
					dstConfigs[i] = dst.Merge(src)
					found = true
					break
				}
			}
			if !found {
				dstConfigs = append(dstConfigs, config.DefaultConsulConfig().Merge(src))
			}
		}
		return dstConfigs
	}

	results = doMerge(results, left)
	results = doMerge(results, right)
	return results
}

func mergeKEKProviderConfigs(left, right []*structs.KEKProviderConfig) []*structs.KEKProviderConfig {
	if len(left) == 0 {
		return right
	}
	if len(right) == 0 {
		return left
	}
	results := []*structs.KEKProviderConfig{}
	doMerge := func(dstConfigs, srcConfigs []*structs.KEKProviderConfig) []*structs.KEKProviderConfig {
		for _, src := range srcConfigs {
			var found bool
			for i, dst := range dstConfigs {
				if dst.Provider == src.Provider && dst.Name == src.Name {
					dstConfigs[i] = dst.Merge(src)
					found = true
					break
				}
			}
			if !found {
				dstConfigs = append(dstConfigs, src)
			}
		}
		return dstConfigs
	}

	results = doMerge(results, left)
	results = doMerge(results, right)
	sort.Slice(results, func(i, j int) bool {
		return results[i].ID() < results[j].ID()
	})

	return results
}

// Copy returns a deep copy safe for mutation.
func (c *Config) Copy() *Config {
	if c == nil {
		return nil
	}
	nc := *c

	nc.Ports = c.Ports.Copy()
	nc.Addresses = c.Addresses.Copy()
	nc.normalizedAddrs = c.normalizedAddrs.Copy()
	nc.AdvertiseAddrs = c.AdvertiseAddrs.Copy()
	nc.Client = c.Client.Copy()
	nc.Server = c.Server.Copy()
	nc.ACL = c.ACL.Copy()
	nc.Telemetry = c.Telemetry.Copy()
	nc.DisableUpdateCheck = pointer.Copy(c.DisableUpdateCheck)
	nc.Consuls = helper.CopySlice(c.Consuls)
	nc.Vaults = helper.CopySlice(c.Vaults)
	nc.UI = c.UI.Copy()

	nc.NomadConfig = c.NomadConfig.Copy()
	nc.ClientConfig = c.ClientConfig.Copy()

	nc.Version = c.Version.Copy()
	nc.Files = slices.Clone(c.Files)
	nc.TLSConfig = c.TLSConfig.Copy()
	nc.HTTPAPIResponseHeaders = maps.Clone(c.HTTPAPIResponseHeaders)
	nc.Sentinel = c.Sentinel.Copy()
	nc.Autopilot = c.Autopilot.Copy()
	nc.Plugins = helper.CopySlice(c.Plugins)
	nc.Limits = c.Limits.Copy()
	nc.Audit = c.Audit.Copy()
	nc.Reporting = c.Reporting.Copy()
	nc.KEKProviders = helper.CopySlice(c.KEKProviders)
	nc.ExtraKeysHCL = slices.Clone(c.ExtraKeysHCL)
	return &nc
}

// normalizeAddrs normalizes Addresses and AdvertiseAddrs to always be
// initialized and have reasonable defaults.
func (c *Config) normalizeAddrs() error {
	if c.BindAddr != "" {
		ipStr, err := listenerutil.ParseSingleIPTemplate(c.BindAddr)
		if err != nil {
			return fmt.Errorf("Bind address resolution failed: %v", err)
		}
		c.BindAddr = ipStr
	}

	httpAddrs, err := normalizeMultipleBind(c.Addresses.HTTP, c.BindAddr)
	if err != nil {
		return fmt.Errorf("Failed to parse HTTP address: %v", err)
	}
	c.Addresses.HTTP = strings.Join(httpAddrs, " ")

	addr, err := normalizeBind(c.Addresses.RPC, c.BindAddr)
	if err != nil {
		return fmt.Errorf("Failed to parse RPC address: %v", err)
	}
	c.Addresses.RPC = addr

	addr, err = normalizeBind(c.Addresses.Serf, c.BindAddr)
	if err != nil {
		return fmt.Errorf("Failed to parse Serf address: %v", err)
	}
	c.Addresses.Serf = addr

	c.normalizedAddrs = &NormalizedAddrs{
		HTTP: joinHostPorts(httpAddrs, strconv.Itoa(c.Ports.HTTP)),
		RPC:  net.JoinHostPort(c.Addresses.RPC, strconv.Itoa(c.Ports.RPC)),
		Serf: net.JoinHostPort(c.Addresses.Serf, strconv.Itoa(c.Ports.Serf)),
	}

	addr, err = normalizeAdvertise(c.AdvertiseAddrs.HTTP, httpAddrs[0], c.Ports.HTTP, c.DevMode)
	if err != nil {
		return fmt.Errorf("Failed to parse HTTP advertise address (%v, %v, %v, %v): %v", c.AdvertiseAddrs.HTTP, c.Addresses.HTTP, c.Ports.HTTP, c.DevMode, err)
	}
	c.AdvertiseAddrs.HTTP = addr

	addr, err = normalizeAdvertise(c.AdvertiseAddrs.RPC, c.Addresses.RPC, c.Ports.RPC, c.DevMode)
	if err != nil {
		return fmt.Errorf("Failed to parse RPC advertise address: %v", err)
	}
	c.AdvertiseAddrs.RPC = addr

	// Skip serf if server is disabled
	if c.Server != nil && c.Server.Enabled {
		addr, err = normalizeAdvertise(c.AdvertiseAddrs.Serf, c.Addresses.Serf, c.Ports.Serf, c.DevMode)
		if err != nil {
			return fmt.Errorf("Failed to parse Serf advertise address: %v", err)
		}
		c.AdvertiseAddrs.Serf = addr
	}

	// Skip network_interface evaluation if not a client
	if c.Client != nil && c.Client.Enabled && c.Client.NetworkInterface != "" {
		parsed, err := parseSingleInterfaceTemplate(c.Client.NetworkInterface)
		if err != nil {
			return fmt.Errorf("Failed to parse network-interface: %v", err)
		}

		c.Client.NetworkInterface = parsed
	}

	return nil
}

// parseSingleInterfaceTemplate parses a go-sockaddr template and returns an
// error if it doesn't result in a single value.
func parseSingleInterfaceTemplate(tpl string) (string, error) {
	out, err := template.Parse(tpl)
	if err != nil {
		// Typically something like:
		// unable to parse template "{{printfl \"en50\"}}": template: sockaddr.Parse:1: function "printfl" not defined
		return "", err
	}

	// Remove any extra empty space around the rendered result and check if the
	// result is also not empty if the user provided a template.
	out = strings.TrimSpace(out)
	if tpl != "" && out == "" {
		return "", fmt.Errorf("template %q evaluated to empty result", tpl)
	}

	// `template.Parse` returns a space-separated list of results, but on
	// Windows network interfaces are allowed to have spaces, so there is no
	// guaranteed separators that we can use to test if the template returned
	// multiple interfaces.
	// The test below checks if the template results to a single valid interface.
	_, err = net.InterfaceByName(out)
	if err != nil {
		return "", fmt.Errorf("invalid interface name %q", out)
	}

	return out, nil
}

// parseMultipleIPTemplate is used as a helper function to parse out a multiple IP
// addresses from a config parameter.
func parseMultipleIPTemplate(ipTmpl string) ([]string, error) {
	out, err := template.Parse(ipTmpl)
	if err != nil {
		return []string{}, fmt.Errorf("Unable to parse address template %q: %v", ipTmpl, err)
	}

	ips := strings.Split(out, " ")
	if len(ips) == 0 {
		return []string{}, errors.New("No addresses found, please configure one.")
	}

	return deduplicateAddrs(ips), nil
}

// normalizeBind returns a normalized bind address.
//
// If addr is set it is used, if not the default bind address is used.
func normalizeBind(addr, bind string) (string, error) {
	if addr == "" {
		return bind, nil
	}
	return listenerutil.ParseSingleIPTemplate(addr)
}

// normalizeMultipleBind returns normalized bind addresses.
//
// If addr is set it is used, if not the default bind address is used.
func normalizeMultipleBind(addr, bind string) ([]string, error) {
	if addr == "" {
		return []string{bind}, nil
	}
	return parseMultipleIPTemplate(addr)
}

// normalizeAdvertise returns a normalized advertise address.
//
// If addr is set, it is used and the default port is appended if no port is
// set.
//
// If addr is not set and bind is a valid address, the returned string is the
// bind+port.
//
// If addr is not set and bind is not a valid advertise address, the hostname
// is resolved and returned with the port.
//
// Loopback is only considered a valid advertise address in dev mode.
func normalizeAdvertise(addr string, bind string, defport int, dev bool) (string, error) {
	addr, err := listenerutil.ParseSingleIPTemplate(addr)
	if err != nil {
		return "", fmt.Errorf("Error parsing advertise address template: %v", err)
	}

	if addr != "" {
		// Default to using manually configured address
		_, _, err = net.SplitHostPort(addr)
		if err != nil {
			if !isMissingPort(err) && !isTooManyColons(err) {
				return "", fmt.Errorf("Error parsing advertise address %q: %v", addr, err)
			}

			// missing port, append the default
			return net.JoinHostPort(addr, strconv.Itoa(defport)), nil
		}

		return addr, nil
	}

	// Fallback to bind address first, and then try resolving the local hostname
	ips, err := net.LookupIP(bind)
	if err != nil {
		return "", fmt.Errorf("Error resolving bind address %q: %v", bind, err)
	}

	// Return the first non-localhost unicast address
	for _, ip := range ips {
		if ip.IsLinkLocalUnicast() || ip.IsGlobalUnicast() {
			return net.JoinHostPort(ip.String(), strconv.Itoa(defport)), nil
		}
		if ip.IsLoopback() {
			if dev {
				// loopback is fine for dev mode
				return net.JoinHostPort(ip.String(), strconv.Itoa(defport)), nil
			}
			return "", fmt.Errorf("Defaulting advertise to localhost is unsafe, please set advertise manually")
		}
	}

	// Bind is not localhost but not a valid advertise IP, use first private IP
	addr, err = listenerutil.ParseSingleIPTemplate("{{ GetPrivateIP }}")
	if err != nil {
		return "", fmt.Errorf("Unable to parse default advertise address: %v", err)
	}
	return net.JoinHostPort(addr, strconv.Itoa(defport)), nil
}

// isMissingPort returns true if an error is a "missing port" error from
// net.SplitHostPort.
func isMissingPort(err error) bool {
	// matches error const in net/ipsock.go
	const missingPort = "missing port in address"
	return err != nil && strings.Contains(err.Error(), missingPort)
}

// isTooManyColons returns true if an error is a "too many colons" error from
// net.SplitHostPort.
func isTooManyColons(err error) bool {
	// matches error const in net/ipsock.go
	const tooManyColons = "too many colons in address"
	return err != nil && strings.Contains(err.Error(), tooManyColons)
}

// Merge is used to merge two ACL configs together. The settings from the input always take precedence.
func (a *ACLConfig) Merge(b *ACLConfig) *ACLConfig {
	result := *a

	if b.Enabled {
		result.Enabled = true
	}
	if b.TokenTTL != 0 {
		result.TokenTTL = b.TokenTTL
	}
	if b.TokenTTLHCL != "" {
		result.TokenTTLHCL = b.TokenTTLHCL
	}
	if b.PolicyTTL != 0 {
		result.PolicyTTL = b.PolicyTTL
	}
	if b.PolicyTTLHCL != "" {
		result.PolicyTTLHCL = b.PolicyTTLHCL
	}
	if b.RoleTTL != 0 {
		result.RoleTTL = b.RoleTTL
	}
	if b.RoleTTLHCL != "" {
		result.RoleTTLHCL = b.RoleTTLHCL
	}
	if b.TokenMinExpirationTTL != 0 {
		result.TokenMinExpirationTTL = b.TokenMinExpirationTTL
	}
	if b.TokenMinExpirationTTLHCL != "" {
		result.TokenMinExpirationTTLHCL = b.TokenMinExpirationTTLHCL
	}
	if b.TokenMaxExpirationTTL != 0 {
		result.TokenMaxExpirationTTL = b.TokenMaxExpirationTTL
	}
	if b.TokenMaxExpirationTTLHCL != "" {
		result.TokenMaxExpirationTTLHCL = b.TokenMaxExpirationTTLHCL
	}
	if b.ReplicationToken != "" {
		result.ReplicationToken = b.ReplicationToken
	}
	return &result
}

// Merge is used to merge two server configs together
func (s *ServerConfig) Merge(b *ServerConfig) *ServerConfig {
	result := *s

	if b.Enabled {
		result.Enabled = true
	}
	if b.AuthoritativeRegion != "" {
		result.AuthoritativeRegion = b.AuthoritativeRegion
	}
	if b.BootstrapExpect > 0 {
		result.BootstrapExpect = b.BootstrapExpect
	}
	if b.DataDir != "" {
		result.DataDir = b.DataDir
	}
	if b.ProtocolVersion != 0 {
		result.ProtocolVersion = b.ProtocolVersion
	}
	if b.RaftProtocol != 0 {
		result.RaftProtocol = b.RaftProtocol
	}
	if b.RaftMultiplier != nil {
		c := *b.RaftMultiplier
		result.RaftMultiplier = &c
	}
	if b.NumSchedulers != nil {
		result.NumSchedulers = pointer.Of(*b.NumSchedulers)
	}
	if b.NodeGCThreshold != "" {
		result.NodeGCThreshold = b.NodeGCThreshold
	}
	if b.JobGCInterval != "" {
		result.JobGCInterval = b.JobGCInterval
	}
	if b.JobGCThreshold != "" {
		result.JobGCThreshold = b.JobGCThreshold
	}
	if b.JobDefaultPriority != nil {
		result.JobDefaultPriority = pointer.Of(*b.JobDefaultPriority)
	}
	if b.JobMaxPriority != nil {
		result.JobMaxPriority = pointer.Of(*b.JobMaxPriority)
	}
	if b.EvalGCThreshold != "" {
		result.EvalGCThreshold = b.EvalGCThreshold
	}
	if b.BatchEvalGCThreshold != "" {
		result.BatchEvalGCThreshold = b.BatchEvalGCThreshold
	}
	if b.DeploymentGCThreshold != "" {
		result.DeploymentGCThreshold = b.DeploymentGCThreshold
	}
	if b.CSIVolumeClaimGCInterval != "" {
		result.CSIVolumeClaimGCInterval = b.CSIVolumeClaimGCInterval
	}
	if b.CSIVolumeClaimGCThreshold != "" {
		result.CSIVolumeClaimGCThreshold = b.CSIVolumeClaimGCThreshold
	}
	if b.CSIPluginGCThreshold != "" {
		result.CSIPluginGCThreshold = b.CSIPluginGCThreshold
	}
	if b.ACLTokenGCThreshold != "" {
		result.ACLTokenGCThreshold = b.ACLTokenGCThreshold
	}
	if b.RootKeyGCInterval != "" {
		result.RootKeyGCInterval = b.RootKeyGCInterval
	}
	if b.RootKeyGCThreshold != "" {
		result.RootKeyGCThreshold = b.RootKeyGCThreshold
	}
	if b.RootKeyRotationThreshold != "" {
		result.RootKeyRotationThreshold = b.RootKeyRotationThreshold
	}
	if b.HeartbeatGrace != 0 {
		result.HeartbeatGrace = b.HeartbeatGrace
	}
	if b.HeartbeatGraceHCL != "" {
		result.HeartbeatGraceHCL = b.HeartbeatGraceHCL
	}
	if b.MinHeartbeatTTL != 0 {
		result.MinHeartbeatTTL = b.MinHeartbeatTTL
	}
	if b.MinHeartbeatTTLHCL != "" {
		result.MinHeartbeatTTLHCL = b.MinHeartbeatTTLHCL
	}
	if b.MaxHeartbeatsPerSecond != 0.0 {
		result.MaxHeartbeatsPerSecond = b.MaxHeartbeatsPerSecond
	}
	if b.FailoverHeartbeatTTL != 0 {
		result.FailoverHeartbeatTTL = b.FailoverHeartbeatTTL
	}
	if b.FailoverHeartbeatTTLHCL != "" {
		result.FailoverHeartbeatTTLHCL = b.FailoverHeartbeatTTLHCL
	}
	if b.RetryMaxAttempts != 0 {
		result.RetryMaxAttempts = b.RetryMaxAttempts
	}
	if b.RetryInterval != 0 {
		result.RetryInterval = b.RetryInterval
	}
	if b.RetryIntervalHCL != "" {
		result.RetryIntervalHCL = b.RetryIntervalHCL
	}
	if b.RejoinAfterLeave {
		result.RejoinAfterLeave = true
	}
	if b.NonVotingServer {
		result.NonVotingServer = true
	}
	if b.RedundancyZone != "" {
		result.RedundancyZone = b.RedundancyZone
	}
	if b.UpgradeVersion != "" {
		result.UpgradeVersion = b.UpgradeVersion
	}
	if b.EncryptKey != "" {
		result.EncryptKey = b.EncryptKey
	}
	if b.ServerJoin != nil {
		result.ServerJoin = result.ServerJoin.Merge(b.ServerJoin)
	}
	if b.LicensePath != "" {
		result.LicensePath = b.LicensePath
	}

	if b.EnableEventBroker != nil {
		result.EnableEventBroker = b.EnableEventBroker
	}

	if b.EventBufferSize != nil {
		result.EventBufferSize = b.EventBufferSize
	}

	result.JobMaxSourceSize = pointer.Merge(s.JobMaxSourceSize, b.JobMaxSourceSize)

	if b.PlanRejectionTracker != nil {
		result.PlanRejectionTracker = result.PlanRejectionTracker.Merge(b.PlanRejectionTracker)
	}

	if b.DefaultSchedulerConfig != nil {
		c := *b.DefaultSchedulerConfig
		result.DefaultSchedulerConfig = &c
	}

	if b.DeploymentQueryRateLimit != 0 {
		result.DeploymentQueryRateLimit = b.DeploymentQueryRateLimit
	}

	if b.Search != nil {
		result.Search = &Search{FuzzyEnabled: b.Search.FuzzyEnabled}
		if b.Search.LimitQuery > 0 {
			result.Search.LimitQuery = b.Search.LimitQuery
		}
		if b.Search.LimitResults > 0 {
			result.Search.LimitResults = b.Search.LimitResults
		}
		if b.Search.MinTermLength > 0 {
			result.Search.MinTermLength = b.Search.MinTermLength
		}
	}

	if b.RaftBoltConfig != nil {
		result.RaftBoltConfig = &RaftBoltConfig{
			NoFreelistSync: b.RaftBoltConfig.NoFreelistSync,
		}
	}

	if b.RaftSnapshotThreshold != nil {
		result.RaftSnapshotThreshold = pointer.Of(*b.RaftSnapshotThreshold)
	}

	if b.RaftSnapshotInterval != nil {
		result.RaftSnapshotInterval = pointer.Of(*b.RaftSnapshotInterval)
	}

	if b.RaftTrailingLogs != nil {
		result.RaftTrailingLogs = pointer.Of(*b.RaftTrailingLogs)
	}

	if b.JobTrackedVersions != nil {
		result.JobTrackedVersions = b.JobTrackedVersions
	}

	if b.OIDCIssuer != "" {
		result.OIDCIssuer = b.OIDCIssuer
	}

	// Add the schedulers
	result.EnabledSchedulers = append(result.EnabledSchedulers, b.EnabledSchedulers...)

	// Copy the start join addresses
	result.StartJoin = make([]string, 0, len(s.StartJoin)+len(b.StartJoin))
	result.StartJoin = append(result.StartJoin, s.StartJoin...)
	result.StartJoin = append(result.StartJoin, b.StartJoin...)

	// Copy the retry join addresses
	result.RetryJoin = make([]string, 0, len(s.RetryJoin)+len(b.RetryJoin))
	result.RetryJoin = append(result.RetryJoin, s.RetryJoin...)
	result.RetryJoin = append(result.RetryJoin, b.RetryJoin...)

	return &result
}

// Merge is used to merge two client configs together
func (a *ClientConfig) Merge(b *ClientConfig) *ClientConfig {
	result := *a

	if b.Enabled {
		result.Enabled = true
	}
	if b.StateDir != "" {
		result.StateDir = b.StateDir
	}
	if b.AllocDir != "" {
		result.AllocDir = b.AllocDir
	}
	if b.AllocMountsDir != "" {
		result.AllocMountsDir = b.AllocMountsDir
	}
	if b.NodeClass != "" {
		result.NodeClass = b.NodeClass
	}
	if b.NodePool != "" {
		result.NodePool = b.NodePool
	}
	if b.NetworkInterface != "" {
		result.NetworkInterface = b.NetworkInterface
	}

	if b.PreferredAddressFamily != "" {
		result.PreferredAddressFamily = b.PreferredAddressFamily
	}

	if b.NetworkSpeed != 0 {
		result.NetworkSpeed = b.NetworkSpeed
	}
	if b.CpuCompute != 0 {
		result.CpuCompute = b.CpuCompute
	}
	if b.MemoryMB != 0 {
		result.MemoryMB = b.MemoryMB
	}
	if b.DiskTotalMB != 0 {
		result.DiskTotalMB = b.DiskTotalMB
	}
	if b.DiskFreeMB != 0 {
		result.DiskFreeMB = b.DiskFreeMB
	}
	if b.MaxKillTimeout != "" {
		result.MaxKillTimeout = b.MaxKillTimeout
	}
	if b.ClientMaxPort != 0 {
		result.ClientMaxPort = b.ClientMaxPort
	}
	if b.ClientMinPort != 0 {
		result.ClientMinPort = b.ClientMinPort
	}
	if b.MaxDynamicPort != 0 {
		result.MaxDynamicPort = b.MaxDynamicPort
	}
	if b.MinDynamicPort != 0 {
		result.MinDynamicPort = b.MinDynamicPort
	}
	if result.Reserved == nil && b.Reserved != nil {
		reserved := *b.Reserved
		result.Reserved = &reserved
	} else if b.Reserved != nil {
		result.Reserved = result.Reserved.Merge(b.Reserved)
	}
	if b.ReservableCores != "" {
		result.ReservableCores = b.ReservableCores
	}
	if b.GCInterval != 0 {
		result.GCInterval = b.GCInterval
	}
	if b.GCIntervalHCL != "" {
		result.GCIntervalHCL = b.GCIntervalHCL
	}
	if b.GCParallelDestroys != 0 {
		result.GCParallelDestroys = b.GCParallelDestroys
	}
	if b.GCDiskUsageThreshold != 0 {
		result.GCDiskUsageThreshold = b.GCDiskUsageThreshold
	}
	if b.GCInodeUsageThreshold != 0 {
		result.GCInodeUsageThreshold = b.GCInodeUsageThreshold
	}
	if b.GCMaxAllocs != 0 {
		result.GCMaxAllocs = b.GCMaxAllocs
	}
	// NoHostUUID defaults to true, merge if false
	if b.NoHostUUID != nil {
		result.NoHostUUID = b.NoHostUUID
	}

	if b.DisableRemoteExec {
		result.DisableRemoteExec = b.DisableRemoteExec
	}

	if b.TemplateConfig != nil {
		result.TemplateConfig = result.TemplateConfig.Merge(b.TemplateConfig)
	}

	// Add the servers
	result.Servers = append(result.Servers, b.Servers...)

	// Add the options map values
	if result.Options == nil {
		result.Options = make(map[string]string)
	}
	for k, v := range b.Options {
		result.Options[k] = v
	}

	// Add the meta map values
	if result.Meta == nil {
		result.Meta = make(map[string]string)
	}
	for k, v := range b.Meta {
		result.Meta[k] = v
	}

	// Add the chroot_env map values
	if result.ChrootEnv == nil {
		result.ChrootEnv = make(map[string]string)
	}
	for k, v := range b.ChrootEnv {
		result.ChrootEnv[k] = v
	}

	if b.ServerJoin != nil {
		result.ServerJoin = result.ServerJoin.Merge(b.ServerJoin)
	}

	if len(a.HostVolumes) == 0 && len(b.HostVolumes) != 0 {
		result.HostVolumes = structs.CopySliceClientHostVolumeConfig(b.HostVolumes)
	} else if len(b.HostVolumes) != 0 {
		result.HostVolumes = structs.HostVolumeSliceMerge(a.HostVolumes, b.HostVolumes)
	}

	if b.CNIPath != "" {
		result.CNIPath = b.CNIPath
	}
	if b.CNIConfigDir != "" {
		result.CNIConfigDir = b.CNIConfigDir
	}
	if b.BridgeNetworkName != "" {
		result.BridgeNetworkName = b.BridgeNetworkName
	}
	if b.BridgeNetworkSubnet != "" {
		result.BridgeNetworkSubnet = b.BridgeNetworkSubnet
	}
	if b.BridgeNetworkSubnetIPv6 != "" {
		result.BridgeNetworkSubnetIPv6 = b.BridgeNetworkSubnetIPv6
	}
	if b.BridgeNetworkHairpinMode {
		result.BridgeNetworkHairpinMode = true
	}

	result.HostNetworks = a.HostNetworks

	if len(b.HostNetworks) != 0 {
		result.HostNetworks = append(result.HostNetworks, b.HostNetworks...)
	}

	if b.BindWildcardDefaultHostNetwork {
		result.BindWildcardDefaultHostNetwork = true
	}

	// This value is a pointer, therefore if it is not nil the user has
	// supplied an override value.
	if b.NomadServiceDiscovery != nil {
		result.NomadServiceDiscovery = b.NomadServiceDiscovery
	}

	if b.CgroupParent != "" {
		result.CgroupParent = b.CgroupParent
	}

	result.Artifact = a.Artifact.Merge(b.Artifact)
	result.Drain = a.Drain.Merge(b.Drain)
	result.Users = a.Users.Merge(b.Users)

	return &result
}

// Merge is used to merge two telemetry configs together
func (t *Telemetry) Merge(b *Telemetry) *Telemetry {
	result := *t

	if b.InMemoryCollectionInterval != "" {
		result.InMemoryCollectionInterval = b.InMemoryCollectionInterval
	}
	if b.inMemoryCollectionInterval != 0 {
		result.inMemoryCollectionInterval = b.inMemoryCollectionInterval
	}
	if b.InMemoryRetentionPeriod != "" {
		result.InMemoryRetentionPeriod = b.InMemoryRetentionPeriod
	}
	if b.inMemoryRetentionPeriod != 0 {
		result.inMemoryRetentionPeriod = b.inMemoryRetentionPeriod
	}
	if b.StatsiteAddr != "" {
		result.StatsiteAddr = b.StatsiteAddr
	}
	if b.StatsdAddr != "" {
		result.StatsdAddr = b.StatsdAddr
	}
	if b.DataDogAddr != "" {
		result.DataDogAddr = b.DataDogAddr
	}
	if b.DataDogTags != nil {
		result.DataDogTags = b.DataDogTags
	}
	if b.PrometheusMetrics {
		result.PrometheusMetrics = b.PrometheusMetrics
	}
	if b.DisableHostname {
		result.DisableHostname = true
	}

	if b.UseNodeName {
		result.UseNodeName = true
	}
	if b.CollectionInterval != "" {
		result.CollectionInterval = b.CollectionInterval
	}
	if b.collectionInterval != 0 {
		result.collectionInterval = b.collectionInterval
	}
	if b.PublishNodeMetrics {
		result.PublishNodeMetrics = true
	}
	if b.PublishAllocationMetrics {
		result.PublishAllocationMetrics = true
	}
	if b.IncludeAllocMetadataInMetrics {
		result.IncludeAllocMetadataInMetrics = true
	}
	result.AllowedMetadataKeysInMetrics = append(result.AllowedMetadataKeysInMetrics, b.AllowedMetadataKeysInMetrics...)
	if b.CirconusAPIToken != "" {
		result.CirconusAPIToken = b.CirconusAPIToken
	}
	if b.CirconusAPIApp != "" {
		result.CirconusAPIApp = b.CirconusAPIApp
	}
	if b.CirconusAPIURL != "" {
		result.CirconusAPIURL = b.CirconusAPIURL
	}
	if b.CirconusCheckSubmissionURL != "" {
		result.CirconusCheckSubmissionURL = b.CirconusCheckSubmissionURL
	}
	if b.CirconusSubmissionInterval != "" {
		result.CirconusSubmissionInterval = b.CirconusSubmissionInterval
	}
	if b.CirconusCheckID != "" {
		result.CirconusCheckID = b.CirconusCheckID
	}
	if b.CirconusCheckForceMetricActivation != "" {
		result.CirconusCheckForceMetricActivation = b.CirconusCheckForceMetricActivation
	}
	if b.CirconusCheckInstanceID != "" {
		result.CirconusCheckInstanceID = b.CirconusCheckInstanceID
	}
	if b.CirconusCheckSearchTag != "" {
		result.CirconusCheckSearchTag = b.CirconusCheckSearchTag
	}
	if b.CirconusCheckTags != "" {
		result.CirconusCheckTags = b.CirconusCheckTags
	}
	if b.CirconusCheckDisplayName != "" {
		result.CirconusCheckDisplayName = b.CirconusCheckDisplayName
	}
	if b.CirconusBrokerID != "" {
		result.CirconusBrokerID = b.CirconusBrokerID
	}
	if b.CirconusBrokerSelectTag != "" {
		result.CirconusBrokerSelectTag = b.CirconusBrokerSelectTag
	}

	if b.PrefixFilter != nil {
		result.PrefixFilter = b.PrefixFilter
	}

	if b.FilterDefault != nil {
		result.FilterDefault = b.FilterDefault
	}

	if b.DisableDispatchedJobSummaryMetrics {
		result.DisableDispatchedJobSummaryMetrics = b.DisableDispatchedJobSummaryMetrics
	}
	if b.DisableQuotaUtilizationMetrics {
		result.DisableQuotaUtilizationMetrics = b.DisableQuotaUtilizationMetrics
	}
	if b.DisableRPCRateMetricsLabels {
		result.DisableRPCRateMetricsLabels = b.DisableRPCRateMetricsLabels
	}

	return &result
}

// Merge is used to merge two port configurations.
func (a *Ports) Merge(b *Ports) *Ports {
	result := *a

	if b.HTTP != 0 {
		result.HTTP = b.HTTP
	}
	if b.RPC != 0 {
		result.RPC = b.RPC
	}
	if b.Serf != 0 {
		result.Serf = b.Serf
	}
	return &result
}

// Merge is used to merge two address configs together.
func (a *Addresses) Merge(b *Addresses) *Addresses {
	result := *a

	if b.HTTP != "" {
		result.HTTP = b.HTTP
	}
	if b.RPC != "" {
		result.RPC = b.RPC
	}
	if b.Serf != "" {
		result.Serf = b.Serf
	}
	return &result
}

// Merge merges two advertise addrs configs together.
func (a *AdvertiseAddrs) Merge(b *AdvertiseAddrs) *AdvertiseAddrs {
	result := *a

	if b.RPC != "" {
		result.RPC = b.RPC
	}
	if b.Serf != "" {
		result.Serf = b.Serf
	}
	if b.HTTP != "" {
		result.HTTP = b.HTTP
	}
	return &result
}

func (r *Resources) Merge(b *Resources) *Resources {
	result := *r
	if b.CPU != 0 {
		result.CPU = b.CPU
	}
	if b.MemoryMB != 0 {
		result.MemoryMB = b.MemoryMB
	}
	if b.DiskMB != 0 {
		result.DiskMB = b.DiskMB
	}
	if b.ReservedPorts != "" {
		result.ReservedPorts = b.ReservedPorts
	}
	if b.Cores != "" {
		result.Cores = b.Cores
	}
	return &result
}

// LoadConfig loads the configuration at the given path, regardless if its a file or
// directory. Called for each -config to build up the runtime config value. Do not apply any
// default values, defaults should be added once in DefaultConfig
func LoadConfig(path string) (*Config, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		return LoadConfigDir(path)
	}

	cleaned := filepath.Clean(path)
	config, err := ParseConfigFile(cleaned)
	if err != nil {
		return nil, fmt.Errorf("Error loading %s: %s", cleaned, err)
	}

	config.Files = append(config.Files, cleaned)
	return config, nil
}

// LoadConfigDir loads all the configurations in the given directory
// in alphabetical order.
func LoadConfigDir(dir string) (*Config, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf(
			"configuration path must be a directory: %s", dir)
	}

	var files []string
	err = nil
	for err != io.EOF {
		var fis []os.FileInfo
		fis, err = f.Readdir(128)
		if err != nil && err != io.EOF {
			return nil, err
		}

		for _, fi := range fis {
			// Ignore directories
			if fi.IsDir() {
				continue
			}

			// Only care about files that are valid to load.
			name := fi.Name()
			skip := true
			if strings.HasSuffix(name, ".hcl") {
				skip = false
			} else if strings.HasSuffix(name, ".json") {
				skip = false
			}
			if skip || isTemporaryFile(name) {
				continue
			}

			path := filepath.Join(dir, name)
			files = append(files, path)
		}
	}

	// Fast-path if we have no files
	if len(files) == 0 {
		return &Config{}, nil
	}

	sort.Strings(files)

	var result *Config
	for _, f := range files {
		config, err := ParseConfigFile(f)
		if err != nil {
			return nil, fmt.Errorf("Error loading %s: %s", f, err)
		}
		config.Files = append(config.Files, f)

		if result == nil {
			result = config
		} else {
			result = result.Merge(config)
		}
	}

	return result, nil
}

// joinHostPorts joins every addr in addrs with the specified port
func joinHostPorts(addrs []string, port string) []string {
	localAddrs := make([]string, len(addrs))
	for i, k := range addrs {
		localAddrs[i] = net.JoinHostPort(k, port)

	}

	return localAddrs
}

// isTemporaryFile returns true or false depending on whether the
// provided file name is a temporary file for the following editors:
// emacs or vim.
func isTemporaryFile(name string) bool {
	return strings.HasSuffix(name, "~") || // vim
		strings.HasPrefix(name, ".#") || // emacs
		(strings.HasPrefix(name, "#") && strings.HasSuffix(name, "#")) // emacs
}

func deduplicateAddrs(addrs []string) []string {
	keys := make(map[string]bool)
	list := []string{}

	for _, entry := range addrs {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
