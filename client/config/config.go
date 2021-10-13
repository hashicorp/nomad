package config

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/nomad/client/lib/cgutil"
	"github.com/hashicorp/nomad/command/agent/host"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
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

	// A mapping of directories on the host OS to attempt to embed inside each
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

	// LogOutput is the destination for logs
	LogOutput io.Writer

	// Logger provides a logger to thhe client
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

	// ConsulConfig is this Agent's Consul configuration
	ConsulConfig *structsc.ConsulConfig

	// VaultConfig is this Agent's Vault configuration
	VaultConfig *structsc.VaultConfig

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

	// LogLevel is the level of the logs to putout
	LogLevel string

	// NoHostUUID disables using the host's UUID and will force generation of a
	// random UUID.
	NoHostUUID bool

	// ACLEnabled controls if ACL enforcement and management is enabled.
	ACLEnabled bool

	// ACLTokenTTL is how long we cache token values for
	ACLTokenTTL time.Duration

	// ACLPolicyTTL is how long we cache policy values for
	ACLPolicyTTL time.Duration

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
	ReservableCores []uint16
}

type ClientTemplateConfig struct {
	FunctionDenylist []string
	DisableSandbox   bool
}

func (c *ClientTemplateConfig) Copy() *ClientTemplateConfig {
	if c == nil {
		return nil
	}

	nc := new(ClientTemplateConfig)
	*nc = *c
	nc.FunctionDenylist = helper.CopySliceString(nc.FunctionDenylist)
	return nc
}

func (c *Config) Copy() *Config {
	nc := new(Config)
	*nc = *c
	nc.Node = nc.Node.Copy()
	nc.Servers = helper.CopySliceString(nc.Servers)
	nc.Options = helper.CopyMapStringString(nc.Options)
	nc.HostVolumes = structs.CopyMapStringClientHostVolumeConfig(nc.HostVolumes)
	nc.ConsulConfig = c.ConsulConfig.Copy()
	nc.VaultConfig = c.VaultConfig.Copy()
	nc.TemplateConfig = c.TemplateConfig.Copy()
	if c.ReservableCores != nil {
		nc.ReservableCores = make([]uint16, len(c.ReservableCores))
		copy(nc.ReservableCores, c.ReservableCores)
	}
	return nc
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Version:                 version.GetVersion(),
		VaultConfig:             structsc.DefaultVaultConfig(),
		ConsulConfig:            structsc.DefaultConsulConfig(),
		LogOutput:               os.Stderr,
		Region:                  "global",
		StatsCollectionInterval: 1 * time.Second,
		TLSConfig:               &structsc.TLSConfig{},
		LogLevel:                "DEBUG",
		GCInterval:              1 * time.Minute,
		GCParallelDestroys:      2,
		GCDiskUsageThreshold:    80,
		GCInodeUsageThreshold:   70,
		GCMaxAllocs:             50,
		NoHostUUID:              true,
		DisableRemoteExec:       false,
		TemplateConfig: &ClientTemplateConfig{
			FunctionDenylist: []string{"plugin"},
			DisableSandbox:   false,
		},
		RPCHoldTimeout:     5 * time.Second,
		CNIPath:            "/opt/cni/bin",
		CNIConfigDir:       "/opt/cni/config",
		CNIInterfacePrefix: "eth",
		HostNetworks:       map[string]*structs.ClientHostNetworkConfig{},
		CgroupParent:       cgutil.DefaultCgroupParent,
		MaxDynamicPort:     structs.DefaultMinDynamicPort,
		MinDynamicPort:     structs.DefaultMaxDynamicPort,
	}
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
