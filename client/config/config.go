// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/consul-template/config"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/command/agent/host"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/bufconndialer"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/structs"
	structsc "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/version"
)

var (
	// DefaultEnvDenylist is the default set of environment variables that are
	// filtered when passing the environment variables of the host to a task.
	DefaultEnvDenylist = strings.Join(host.DefaultEnvDenyList, ",")

	// DefaultUserDenylist is the default set of users that tasks are not
	// allowed to run as when using a driver in "user.checked_drivers"
	DefaultUserDenylist = strings.Join([]string{
		"root",
		"Administrator",
	}, ",")

	// DefaultUserCheckedDrivers is the set of drivers we apply the user
	// denylist onto. For virtualized drivers it often doesn't make sense to
	// make this stipulation so by default they are ignored.
	DefaultUserCheckedDrivers = strings.Join([]string{
		"exec",
		"qemu",
		"java",
	}, ",")

	// DefaultChrootEnv is a mapping of directories on the host OS to attempt to embed inside each
	// task's chroot.
	DefaultChrootEnv = map[string]string{
		"/bin":            "/bin",
		"/etc":            "/etc",
		"/lib":            "/lib",
		"/lib32":          "/lib32",
		"/lib64":          "/lib64",
		"/run/resolvconf": "/run/resolvconf",
		"/sbin":           "/sbin",
		"/usr":            "/usr",

		// embed systemd-resolved paths for systemd-resolved paths:
		// /etc/resolv.conf is a symlink to /run/systemd/resolve/stub-resolv.conf in such systems.
		// In non-systemd systems, this mount is a no-op and the path is ignored if not present.
		"/run/systemd/resolve": "/run/systemd/resolve",
	}

	DefaultTemplateMaxStale = 87600 * time.Hour

	DefaultTemplateFunctionDenylist = []string{"plugin", "writeToFile"}
)

// RPCHandler can be provided to the Client if there is a local server
// to avoid going over the network. If not provided, the Client will
// maintain a connection pool to the servers
type RPCHandler interface {
	RPC(method string, args interface{}, reply interface{}) error
}

// Config is used to parameterize and configure the behavior of the client
type Config struct {
	// DevMode controls if we are in a development mode which
	// avoids persistent storage.
	DevMode bool

	// EnableDebug is used to enable debugging RPC endpoints
	// in the absence of ACLs
	EnableDebug bool

	// StateDir is where we store our state
	StateDir string

	// AllocDir is where we store data for allocations
	AllocDir string

	// Logger provides a logger to the client
	Logger log.InterceptLogger

	// Region is the clients region
	Region string

	// Network interface to be used in network fingerprinting
	NetworkInterface string

	// Network speed is the default speed of network interfaces if they can not
	// be determined dynamically.
	NetworkSpeed int

	// CpuCompute is the default total CPU compute if they can not be determined
	// dynamically. It should be given as Cores * MHz (2 Cores * 2 Ghz = 4000)
	CpuCompute int

	// MemoryMB is the default node total memory in megabytes if it cannot be
	// determined dynamically.
	MemoryMB int

	// DiskTotalMB is the default node total disk space in megabytes if it cannot be
	// determined dynamically.
	DiskTotalMB int

	// DiskFreeMB is the default node free disk space in megabytes if it cannot be
	// determined dynamically.
	DiskFreeMB int

	// MaxKillTimeout allows capping the user-specifiable KillTimeout. If the
	// task's KillTimeout is greater than the MaxKillTimeout, MaxKillTimeout is
	// used.
	MaxKillTimeout time.Duration

	// Servers is a list of known server addresses. These are as "host:port"
	Servers []string

	// RPCHandler can be provided to avoid network traffic if the
	// server is running locally.
	RPCHandler RPCHandler

	// Node provides the base node
	Node *structs.Node

	// ClientMaxPort is the upper range of the ports that the client uses for
	// communicating with plugin subsystems over loopback
	ClientMaxPort uint

	// ClientMinPort is the lower range of the ports that the client uses for
	// communicating with plugin subsystems over loopback
	ClientMinPort uint

	// MaxDynamicPort is the largest dynamic port generated
	MaxDynamicPort int

	// MinDynamicPort is the smallest dynamic port generated
	MinDynamicPort int

	// A mapping of directories on the host OS to attempt to embed inside each
	// task's chroot.
	ChrootEnv map[string]string

	// Options provides arbitrary key-value configuration for nomad internals,
	// like fingerprinters and drivers. The format is:
	//
	//	namespace.option = value
	Options map[string]string

	// Version is the version of the Nomad client
	Version *version.VersionInfo

	// ConsulConfig is this Agent's default Consul configuration
	ConsulConfig *structsc.ConsulConfig

	// ConsulConfigs is a map of Consul configurations, here to support features
	// in Nomad Enterprise. The default Consul config pointer above will be
	// found in this map under the name "default"
	ConsulConfigs map[string]*structsc.ConsulConfig

	// VaultConfig is this Agent's default Vault configuration
	VaultConfig *structsc.VaultConfig

	// VaultConfigs is a map of Vault configurations, here to support features
	// in Nomad Enterprise. The default Vault config pointer above will be found
	// in this map under the name "default"
	VaultConfigs map[string]*structsc.VaultConfig

	// StatsCollectionInterval is the interval at which the Nomad client
	// collects resource usage stats
	StatsCollectionInterval time.Duration

	// PublishNodeMetrics determines whether nomad is going to publish node
	// level metrics to remote Telemetry sinks
	PublishNodeMetrics bool

	// PublishAllocationMetrics determines whether nomad is going to publish
	// allocation metrics to remote Telemetry sinks
	PublishAllocationMetrics bool

	// TLSConfig holds various TLS related configurations
	TLSConfig *structsc.TLSConfig

	// GCInterval is the time interval at which the client triggers garbage
	// collection
	GCInterval time.Duration

	// GCParallelDestroys is the number of parallel destroys the garbage
	// collector will allow.
	GCParallelDestroys int

	// GCDiskUsageThreshold is the disk usage threshold given as a percent
	// beyond which the Nomad client triggers GC of terminal allocations
	GCDiskUsageThreshold float64

	// GCInodeUsageThreshold is the inode usage threshold given as a percent
	// beyond which the Nomad client triggers GC of the terminal allocations
	GCInodeUsageThreshold float64

	// GCMaxAllocs is the maximum number of allocations a node can have
	// before garbage collection is triggered.
	GCMaxAllocs int

	// NoHostUUID disables using the host's UUID and will force generation of a
	// random UUID.
	NoHostUUID bool

	// ACLEnabled controls if ACL enforcement and management is enabled.
	ACLEnabled bool

	// ACLTokenTTL is how long we cache token values for
	ACLTokenTTL time.Duration

	// ACLPolicyTTL is how long we cache policy values for
	ACLPolicyTTL time.Duration

	// ACLRoleTTL is how long we cache ACL role value for within each Nomad
	// client.
	ACLRoleTTL time.Duration

	// DisableRemoteExec disables remote exec targeting tasks on this client
	DisableRemoteExec bool

	// TemplateConfig includes configuration for template rendering
	TemplateConfig *ClientTemplateConfig

	// RPCHoldTimeout is how long an RPC can be "held" before it is errored.
	// This is used to paper over a loss of leadership by instead holding RPCs,
	// so that the caller experiences a slow response rather than an error.
	// This period is meant to be long enough for a leader election to take
	// place, and a small jitter is applied to avoid a thundering herd.
	RPCHoldTimeout time.Duration

	// PluginLoader is used to load plugins.
	PluginLoader loader.PluginCatalog

	// PluginSingletonLoader is a plugin loader that will returns singleton
	// instances of the plugins.
	PluginSingletonLoader loader.PluginCatalog

	// StateDBFactory is used to override stateDB implementations,
	StateDBFactory state.NewStateDBFunc

	AllocRunnerFactory AllocRunnerFactory

	// CNIPath is the path used to search for CNI plugins. Multiple paths can
	// be specified with colon delimited
	CNIPath string

	// CNIConfigDir is the directory where CNI network configuration is located. The
	// client will use this path when fingerprinting CNI networks.
	CNIConfigDir string

	// CNIInterfacePrefix is the prefix to use when creating CNI network interfaces. This
	// defaults to 'eth', therefore the first interface created by CNI inside the alloc
	// network will be 'eth0'.
	CNIInterfacePrefix string

	// BridgeNetworkName is the name to use for the bridge created in bridge
	// networking mode. This defaults to 'nomad' if not set
	BridgeNetworkName string

	// BridgeNetworkHairpinMode is whether or not to enable hairpin mode on the
	// internal bridge network
	BridgeNetworkHairpinMode bool

	// BridgeNetworkAllocSubnet is the IP subnet to use for address allocation
	// for allocations in bridge networking mode. Subnet must be in CIDR
	// notation
	BridgeNetworkAllocSubnet string

	// HostVolumes is a map of the configured host volumes by name.
	HostVolumes map[string]*structs.ClientHostVolumeConfig

	// HostNetworks is a map of the conigured host networks by name.
	HostNetworks map[string]*structs.ClientHostNetworkConfig

	// BindWildcardDefaultHostNetwork toggles if the default host network should accept all
	// destinations (true) or only filter on the IP of the default host network (false) when
	// port mapping. This allows Nomad clients with no defined host networks to accept and
	// port forward traffic only matching on the destination port. An example use of this
	// is when a network loadbalancer is utilizing direct server return and the destination
	// address of incomming packets does not match the IP address of the host interface.
	//
	// This configuration is only considered if no host networks are defined.
	BindWildcardDefaultHostNetwork bool

	// CgroupParent is the parent cgroup Nomad should use when managing any cgroup subsystems.
	// Currently this only includes the 'cpuset' cgroup subsystem.
	CgroupParent string

	// ReservableCores if set overrides the set of reservable cores reported in fingerprinting.
	ReservableCores []numalib.CoreID

	// NomadServiceDiscovery determines whether the Nomad native service
	// discovery client functionality is enabled.
	NomadServiceDiscovery bool

	// TemplateDialer is our custom HTTP dialer for consul-template. This is
	// used for template functions which require access to the Nomad API.
	TemplateDialer *bufconndialer.BufConnWrapper

	// APIListenerRegistrar allows the client to register listeners created at
	// runtime (eg the Task API) with the agent's HTTP server. Since the agent
	// creates the HTTP *after* the client starts, we have to use this shim to
	// pass listeners back to the agent.
	// This is the same design as the bufconndialer but for the
	// http.Serve(listener) API instead of the net.Dial API.
	APIListenerRegistrar APIListenerRegistrar

	// Artifact configuration from the agent's config file.
	Artifact *ArtifactConfig

	// Drain configuration from the agent's config file.
	Drain *DrainConfig

	// ExtraAllocHooks are run with other allocation hooks, mainly for testing.
	ExtraAllocHooks []interfaces.RunnerHook
}

type APIListenerRegistrar interface {
	// Serve the HTTP API on the provided listener.
	//
	// The context is because Serve may be called before the HTTP server has been
	// initialized. If the context is canceled before the HTTP server is
	// initialized, the context's error will be returned.
	Serve(context.Context, net.Listener) error
}

// ClientTemplateConfig is configuration on the client specific to template
// rendering
type ClientTemplateConfig struct {
	// FunctionDenylist disables functions in consul-template that
	// are unsafe because they expose information from the client host.
	FunctionDenylist []string `hcl:"function_denylist"`

	// Deprecated: COMPAT(1.0) consul-template uses inclusive language from
	// v0.25.0 - function_blacklist is kept for compatibility
	FunctionBlacklist []string `hcl:"function_blacklist"`

	// DisableSandbox allows templates to access arbitrary files on the
	// client host. By default templates can access files only within
	// the task directory.
	DisableSandbox bool `hcl:"disable_file_sandbox"`

	// This is the maximum interval to allow "stale" data. By default, only the
	// Consul leader will respond to queries; any requests to a follower will
	// forward to the leader. In large clusters with many requests, this is not as
	// scalable, so this option allows any follower to respond to a query, so long
	// as the last-replicated data is within these bounds. Higher values result in
	// less cluster load, but are more likely to have outdated data.
	// NOTE: Since Consul Template uses a pointer, this field uses a pointer which
	// is inconsistent with how Nomad typically works. This decision was made to
	// maintain parity with the external subsystem, not to establish a new standard.
	MaxStale    *time.Duration `hcl:"-"`
	MaxStaleHCL string         `hcl:"max_stale,optional"`

	// BlockQueryWaitTime is amount of time in seconds to do a blocking query for.
	// Many endpoints in Consul support a feature known as "blocking queries".
	// A blocking query is used to wait for a potential change using long polling.
	// NOTE: Since Consul Template uses a pointer, this field uses a pointer which
	// is inconsistent with how Nomad typically works. This decision was made to
	// maintain parity with the external subsystem, not to establish a new standard.
	BlockQueryWaitTime    *time.Duration `hcl:"-"`
	BlockQueryWaitTimeHCL string         `hcl:"block_query_wait,optional"`

	// Wait is the quiescence timers; it defines the minimum and maximum amount of
	// time to wait for the Consul cluster to reach a consistent state before rendering a
	// template. This is useful to enable in systems where Consul is experiencing
	// a lot of flapping because it will reduce the number of times a template is rendered.
	Wait *WaitConfig `hcl:"wait,optional" json:"-"`

	// WaitBounds allows operators to define boundaries on individual template wait
	// configuration overrides. If set, this ensures that if a job author specifies
	// a wait configuration with values the cluster operator does not allow, the
	// cluster operator's boundary will be applied rather than the job author's
	// out of bounds configuration.
	WaitBounds *WaitConfig `hcl:"wait_bounds,optional" json:"-"`

	// This controls the retry behavior when an error is returned from Consul.
	// Consul Template is highly fault tolerant, meaning it does not exit in the
	// face of failure. Instead, it uses exponential back-off and retry functions
	// to wait for the cluster to become available, as is customary in distributed
	// systems.
	ConsulRetry *RetryConfig `hcl:"consul_retry,optional"`

	// This controls the retry behavior when an error is returned from Vault.
	// Consul Template is highly fault tolerant, meaning it does not exit in the
	// face of failure. Instead, it uses exponential back-off and retry functions
	// to wait for the cluster to become available, as is customary in distributed
	// systems.
	VaultRetry *RetryConfig `hcl:"vault_retry,optional"`

	// This controls the retry behavior when an error is returned from Nomad.
	// Consul Template is highly fault tolerant, meaning it does not exit in the
	// face of failure. Instead, it uses exponential back-off and retry functions
	// to wait for the cluster to become available, as is customary in distributed
	// systems.
	NomadRetry *RetryConfig `hcl:"nomad_retry,optional"`
}

// Copy returns a deep copy of a ClientTemplateConfig
func (c *ClientTemplateConfig) Copy() *ClientTemplateConfig {
	if c == nil {
		return nil
	}

	nc := new(ClientTemplateConfig)
	*nc = *c

	if len(c.FunctionDenylist) > 0 {
		nc.FunctionDenylist = slices.Clone(nc.FunctionDenylist)
	} else if c.FunctionDenylist != nil {
		// Explicitly no functions denied (which is different than nil)
		nc.FunctionDenylist = []string{}
	}

	if c.BlockQueryWaitTime != nil {
		nc.BlockQueryWaitTime = &*c.BlockQueryWaitTime
	}

	if c.MaxStale != nil {
		nc.MaxStale = &*c.MaxStale
	}

	if c.Wait != nil {
		nc.Wait = c.Wait.Copy()
	}

	if c.ConsulRetry != nil {
		nc.ConsulRetry = c.ConsulRetry.Copy()
	}

	if c.VaultRetry != nil {
		nc.VaultRetry = c.VaultRetry.Copy()
	}

	if c.NomadRetry != nil {
		nc.NomadRetry = c.NomadRetry.Copy()
	}

	return nc
}

func (c *ClientTemplateConfig) IsEmpty() bool {
	if c == nil {
		return true
	}

	return !c.DisableSandbox &&
		c.FunctionDenylist == nil &&
		c.FunctionBlacklist == nil &&
		c.BlockQueryWaitTime == nil &&
		c.BlockQueryWaitTimeHCL == "" &&
		c.MaxStale == nil &&
		c.MaxStaleHCL == "" &&
		c.Wait.IsEmpty() &&
		c.ConsulRetry.IsEmpty() &&
		c.VaultRetry.IsEmpty() &&
		c.NomadRetry.IsEmpty()
}

// WaitConfig is mirrored from templateconfig.WaitConfig because we need to handle
// the HCL conversion which happens in agent.ParseConfigFile
// NOTE: Since Consul Template requires pointers, this type uses pointers to fields
// which is inconsistent with how Nomad typically works. This decision was made
// to maintain parity with the external subsystem, not to establish a new standard.
type WaitConfig struct {
	Min    *time.Duration `hcl:"-"`
	MinHCL string         `hcl:"min,optional" json:"-"`
	Max    *time.Duration `hcl:"-"`
	MaxHCL string         `hcl:"max,optional" json:"-"`
}

// Copy returns a deep copy of the receiver.
func (wc *WaitConfig) Copy() *WaitConfig {
	if wc == nil {
		return nil
	}

	nwc := new(WaitConfig)

	if wc.Min != nil {
		nwc.Min = &*wc.Min
	}

	if wc.Max != nil {
		nwc.Max = &*wc.Max
	}

	return wc
}

// Equal returns the result of reflect.DeepEqual
func (wc *WaitConfig) Equal(other *WaitConfig) bool {
	return reflect.DeepEqual(wc, other)
}

// IsEmpty returns true if the receiver only contains an instance with no fields set.
func (wc *WaitConfig) IsEmpty() bool {
	if wc == nil {
		return true
	}
	return wc.Equal(&WaitConfig{})
}

// Validate returns an error  if the receiver is nil or empty or if Min is greater
// than Max the user specified Max.
func (wc *WaitConfig) Validate() error {
	// If the config is nil or empty return false so that it is never assigned.
	if wc == nil || wc.IsEmpty() {
		return errors.New("wait config is nil or empty")
	}

	// If min is nil, return
	if wc.Min == nil {
		return nil
	}

	// If min isn't nil, make sure Max is less than Min.
	if wc.Max != nil {
		if *wc.Min > *wc.Max {
			return fmt.Errorf("wait config min %d is greater than max %d", *wc.Min, *wc.Max)
		}
	}

	// Otherwise, return nil. Consul Template will set a Max based off of Min.
	return nil
}

// Merge merges two WaitConfigs. The passed instance always takes precedence.
func (wc *WaitConfig) Merge(b *WaitConfig) *WaitConfig {
	if wc == nil {
		return b
	}

	result := *wc
	if b == nil {
		return &result
	}

	if b.Min != nil {
		result.Min = &*b.Min
	}

	if b.MinHCL != "" {
		result.MinHCL = b.MinHCL
	}

	if b.Max != nil {
		result.Max = &*b.Max
	}

	if b.MaxHCL != "" {
		result.MaxHCL = b.MaxHCL
	}

	return &result
}

// ToConsulTemplate converts a client WaitConfig instance to a consul-template WaitConfig
func (wc *WaitConfig) ToConsulTemplate() (*config.WaitConfig, error) {
	if wc.IsEmpty() {
		return nil, errors.New("wait config is empty")
	}

	if err := wc.Validate(); err != nil {
		return nil, err
	}

	result := &config.WaitConfig{Enabled: pointer.Of(true)}

	if wc.Min != nil {
		result.Min = wc.Min
	}

	if wc.Max != nil {
		result.Max = wc.Max
	}

	return result, nil
}

// RetryConfig is mirrored from templateconfig.WaitConfig because we need to handle
// the HCL indirection to support mapping in agent.ParseConfigFile.
// NOTE: Since Consul Template requires pointers, this type uses pointers to fields
// which is inconsistent with how Nomad typically works. However, since zero in
// Attempts and MaxBackoff have special meaning, it is necessary to know if the
// value was actually set rather than if it defaulted to 0. The rest of the fields
// use pointers to maintain parity with the external subystem, not to establish
// a new standard.
type RetryConfig struct {
	// Attempts is the total number of maximum attempts to retry before letting
	// the error fall through.
	// 0 means unlimited.
	Attempts *int `hcl:"attempts,optional"`
	// Backoff is the base of the exponential backoff. This number will be
	// multiplied by the next power of 2 on each iteration.
	Backoff    *time.Duration `hcl:"-"`
	BackoffHCL string         `hcl:"backoff,optional" json:"-"`
	// MaxBackoff is an upper limit to the sleep time between retries
	// A MaxBackoff of 0 means there is no limit to the exponential growth of the backoff.
	MaxBackoff    *time.Duration `hcl:"-"`
	MaxBackoffHCL string         `hcl:"max_backoff,optional" json:"-"`
}

func (rc *RetryConfig) Copy() *RetryConfig {
	if rc == nil {
		return nil
	}

	nrc := new(RetryConfig)
	*nrc = *rc

	// Now copy pointer values
	if rc.Attempts != nil {
		nrc.Attempts = &*rc.Attempts
	}
	if rc.Backoff != nil {
		nrc.Backoff = &*rc.Backoff
	}
	if rc.MaxBackoff != nil {
		nrc.MaxBackoff = &*rc.MaxBackoff
	}

	return nrc
}

// Equal returns the result of reflect.DeepEqual
func (rc *RetryConfig) Equal(other *RetryConfig) bool {
	return reflect.DeepEqual(rc, other)
}

// IsEmpty returns true if the receiver only contains an instance with no fields set.
func (rc *RetryConfig) IsEmpty() bool {
	if rc == nil {
		return true
	}

	return rc.Equal(&RetryConfig{})
}

// Validate returns an error if the receiver is nil or empty, or if Backoff
// is greater than  MaxBackoff.
func (rc *RetryConfig) Validate() error {
	// If the config is nil or empty return false so that it is never assigned.
	if rc == nil || rc.IsEmpty() {
		return errors.New("retry config is nil or empty")
	}

	// If Backoff not set, no need to validate
	if rc.Backoff == nil {
		return nil
	}

	// MaxBackoff nil will end up defaulted to 1 minutes. We should validate that
	// the user supplied backoff does not exceed that.
	if rc.MaxBackoff == nil && *rc.Backoff > config.DefaultRetryMaxBackoff {
		return fmt.Errorf("retry config backoff %d is greater than default max_backoff %d", *rc.Backoff, config.DefaultRetryMaxBackoff)
	}

	// MaxBackoff == 0 means backoff is unbounded. No need to validate.
	if rc.MaxBackoff != nil && *rc.MaxBackoff == 0 {
		return nil
	}

	if rc.MaxBackoff != nil && *rc.Backoff > *rc.MaxBackoff {
		return fmt.Errorf("retry config backoff %d is greater than max_backoff %d", *rc.Backoff, *rc.MaxBackoff)
	}

	return nil
}

// Merge merges two RetryConfigs. The passed instance always takes precedence.
func (rc *RetryConfig) Merge(b *RetryConfig) *RetryConfig {
	if rc == nil {
		return b
	}

	result := *rc
	if b == nil {
		return &result
	}

	if b.Attempts != nil {
		result.Attempts = &*b.Attempts
	}

	if b.Backoff != nil {
		result.Backoff = &*b.Backoff
	}

	if b.BackoffHCL != "" {
		result.BackoffHCL = b.BackoffHCL
	}

	if b.MaxBackoff != nil {
		result.MaxBackoff = &*b.MaxBackoff
	}

	if b.MaxBackoffHCL != "" {
		result.MaxBackoffHCL = b.MaxBackoffHCL
	}

	return &result
}

// ToConsulTemplate converts a client RetryConfig instance to a consul-template RetryConfig
func (rc *RetryConfig) ToConsulTemplate() (*config.RetryConfig, error) {
	if err := rc.Validate(); err != nil {
		return nil, err
	}

	result := &config.RetryConfig{Enabled: pointer.Of(true)}

	if rc.Attempts != nil {
		result.Attempts = rc.Attempts
	}

	if rc.Backoff != nil {
		result.Backoff = rc.Backoff
	}

	if rc.MaxBackoff != nil {
		result.MaxBackoff = &*rc.MaxBackoff
	}

	return result, nil
}

func (c *Config) Copy() *Config {
	if c == nil {
		return nil
	}

	nc := *c
	nc.Node = nc.Node.Copy()
	nc.Servers = slices.Clone(nc.Servers)
	nc.Options = maps.Clone(nc.Options)
	nc.HostVolumes = structs.CopyMapStringClientHostVolumeConfig(nc.HostVolumes)
	nc.ConsulConfig = c.ConsulConfig.Copy()
	nc.ConsulConfigs = helper.DeepCopyMap(c.ConsulConfigs)
	nc.VaultConfig = c.VaultConfig.Copy()
	nc.VaultConfigs = helper.DeepCopyMap(c.VaultConfigs)
	nc.TemplateConfig = c.TemplateConfig.Copy()
	nc.ReservableCores = slices.Clone(c.ReservableCores)
	nc.Artifact = c.Artifact.Copy()
	return &nc
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	cfg := &Config{
		Version:                 version.GetVersion(),
		VaultConfig:             structsc.DefaultVaultConfig(),
		ConsulConfig:            structsc.DefaultConsulConfig(),
		Region:                  "global",
		StatsCollectionInterval: 1 * time.Second,
		TLSConfig:               &structsc.TLSConfig{},
		GCInterval:              1 * time.Minute,
		GCParallelDestroys:      2,
		GCDiskUsageThreshold:    80,
		GCInodeUsageThreshold:   70,
		GCMaxAllocs:             50,
		NoHostUUID:              true,
		DisableRemoteExec:       false,
		TemplateConfig: &ClientTemplateConfig{
			FunctionDenylist:   DefaultTemplateFunctionDenylist,
			DisableSandbox:     false,
			BlockQueryWaitTime: pointer.Of(5 * time.Minute),         // match Consul default
			MaxStale:           pointer.Of(DefaultTemplateMaxStale), // match Consul default
			Wait: &WaitConfig{
				Min: pointer.Of(5 * time.Second),
				Max: pointer.Of(4 * time.Minute),
			},
			ConsulRetry: &RetryConfig{
				Attempts: pointer.Of(0), // unlimited
			},
			VaultRetry: &RetryConfig{
				Attempts: pointer.Of(0), // unlimited
			},
			NomadRetry: &RetryConfig{
				Attempts: pointer.Of(0), // unlimited
			},
		},
		RPCHoldTimeout:     5 * time.Second,
		CNIPath:            "/opt/cni/bin",
		CNIConfigDir:       "/opt/cni/config",
		CNIInterfacePrefix: "eth",
		HostNetworks:       map[string]*structs.ClientHostNetworkConfig{},
		CgroupParent:       "nomad.slice", // SETH todo
		MaxDynamicPort:     structs.DefaultMinDynamicPort,
		MinDynamicPort:     structs.DefaultMaxDynamicPort,
	}

	cfg.ConsulConfigs = map[string]*structsc.ConsulConfig{
		"default": cfg.ConsulConfig}
	cfg.VaultConfigs = map[string]*structsc.VaultConfig{
		"default": cfg.VaultConfig}

	return cfg
}

// Read returns the specified configuration value or "".
func (c *Config) Read(id string) string {
	return c.Options[id]
}

// ReadDefault returns the specified configuration value, or the specified
// default value if none is set.
func (c *Config) ReadDefault(id string, defaultValue string) string {
	return c.ReadAlternativeDefault([]string{id}, defaultValue)
}

// ReadAlternativeDefault returns the specified configuration value, or the
// specified value if none is set.
func (c *Config) ReadAlternativeDefault(ids []string, defaultValue string) string {
	for _, id := range ids {
		val, ok := c.Options[id]
		if ok {
			return val
		}
	}

	return defaultValue
}

// ReadBool parses the specified option as a boolean.
func (c *Config) ReadBool(id string) (bool, error) {
	val, ok := c.Options[id]
	if !ok {
		return false, fmt.Errorf("Specified config is missing from options")
	}
	bval, err := strconv.ParseBool(val)
	if err != nil {
		return false, fmt.Errorf("Failed to parse %s as bool: %s", val, err)
	}
	return bval, nil
}

// ReadBoolDefault tries to parse the specified option as a boolean. If there is
// an error in parsing, the default option is returned.
func (c *Config) ReadBoolDefault(id string, defaultValue bool) bool {
	val, err := c.ReadBool(id)
	if err != nil {
		return defaultValue
	}
	return val
}

// ReadInt parses the specified option as a int.
func (c *Config) ReadInt(id string) (int, error) {
	val, ok := c.Options[id]
	if !ok {
		return 0, fmt.Errorf("Specified config is missing from options")
	}
	ival, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("Failed to parse %s as int: %s", val, err)
	}
	return ival, nil
}

// ReadIntDefault tries to parse the specified option as a int. If there is
// an error in parsing, the default option is returned.
func (c *Config) ReadIntDefault(id string, defaultValue int) int {
	val, err := c.ReadInt(id)
	if err != nil {
		return defaultValue
	}
	return val
}

// ReadDuration parses the specified option as a duration.
func (c *Config) ReadDuration(id string) (time.Duration, error) {
	val, ok := c.Options[id]
	if !ok {
		return time.Duration(0), fmt.Errorf("Specified config is missing from options")
	}
	dval, err := time.ParseDuration(val)
	if err != nil {
		return time.Duration(0), fmt.Errorf("Failed to parse %s as time duration: %s", val, err)
	}
	return dval, nil
}

// ReadDurationDefault tries to parse the specified option as a duration. If there is
// an error in parsing, the default option is returned.
func (c *Config) ReadDurationDefault(id string, defaultValue time.Duration) time.Duration {
	val, err := c.ReadDuration(id)
	if err != nil {
		return defaultValue
	}
	return val
}

// ReadStringListToMap tries to parse the specified option(s) as a comma separated list.
// If there is an error in parsing, an empty list is returned.
func (c *Config) ReadStringListToMap(keys ...string) map[string]struct{} {
	val := c.ReadAlternativeDefault(keys, "")

	return splitValue(val)
}

// ReadStringListToMapDefault tries to parse the specified option as a comma
// separated list. If there is an error in parsing, an empty list is returned.
func (c *Config) ReadStringListToMapDefault(key, defaultValue string) map[string]struct{} {
	return c.ReadStringListAlternativeToMapDefault([]string{key}, defaultValue)
}

// ReadStringListAlternativeToMapDefault tries to parse the specified options as a comma sparated list.
// If there is an error in parsing, an empty list is returned.
func (c *Config) ReadStringListAlternativeToMapDefault(keys []string, defaultValue string) map[string]struct{} {
	val := c.ReadAlternativeDefault(keys, defaultValue)

	return splitValue(val)
}

// splitValue parses the value as a comma separated list.
func splitValue(val string) map[string]struct{} {
	list := make(map[string]struct{})
	if val != "" {
		for _, e := range strings.Split(val, ",") {
			trimmed := strings.TrimSpace(e)
			list[trimmed] = struct{}{}
		}
	}
	return list
}

// NomadPluginConfig produces the NomadConfig struct which is sent to Nomad plugins
func (c *Config) NomadPluginConfig() *base.AgentConfig {
	return &base.AgentConfig{
		Driver: &base.ClientDriverConfig{
			ClientMinPort: c.ClientMinPort,
			ClientMaxPort: c.ClientMaxPort,
		},
	}
}
