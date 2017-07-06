package config

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/tlsutil"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

var (
	// DefaultEnvBlacklist is the default set of environment variables that are
	// filtered when passing the environment variables of the host to a task.
	DefaultEnvBlacklist = strings.Join([]string{
		"CONSUL_TOKEN",
		"VAULT_TOKEN",
		"ATLAS_TOKEN",
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN",
		"GOOGLE_APPLICATION_CREDENTIALS",
	}, ",")

	// DefaulUserBlacklist is the default set of users that tasks are not
	// allowed to run as when using a driver in "user.checked_drivers"
	DefaultUserBlacklist = strings.Join([]string{
		"root",
		"Administrator",
	}, ",")

	// DefaultUserCheckedDrivers is the set of drivers we apply the user
	// blacklist onto. For virtualized drivers it often doesn't make sense to
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

	// StateDir is where we store our state
	StateDir string

	// AllocDir is where we store data for allocations
	AllocDir string

	// LogOutput is the destination for logs
	LogOutput io.Writer

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

	// GloballyReservedPorts are ports that are reserved across all network
	// devices and IPs.
	GloballyReservedPorts []int

	// A mapping of directories on the host OS to attempt to embed inside each
	// task's chroot.
	ChrootEnv map[string]string

	// Options provides arbitrary key-value configuration for nomad internals,
	// like fingerprinters and drivers. The format is:
	//
	//	namespace.option = value
	Options map[string]string

	// Version is the version of the Nomad client
	Version string

	// Revision is the commit number of the Nomad client
	Revision string

	// ConsulConfig is this Agent's Consul configuration
	ConsulConfig *config.ConsulConfig

	// VaultConfig is this Agent's Vault configuration
	VaultConfig *config.VaultConfig

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
	TLSConfig *config.TLSConfig

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
}

func (c *Config) Copy() *Config {
	nc := new(Config)
	*nc = *c
	nc.Node = nc.Node.Copy()
	nc.Servers = helper.CopySliceString(nc.Servers)
	nc.Options = helper.CopyMapStringString(nc.Options)
	nc.GloballyReservedPorts = helper.CopySliceInt(c.GloballyReservedPorts)
	nc.ConsulConfig = c.ConsulConfig.Copy()
	nc.VaultConfig = c.VaultConfig.Copy()
	return nc
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		VaultConfig:             config.DefaultVaultConfig(),
		ConsulConfig:            config.DefaultConsulConfig(),
		LogOutput:               os.Stderr,
		Region:                  "global",
		StatsCollectionInterval: 1 * time.Second,
		TLSConfig:               &config.TLSConfig{},
		LogLevel:                "DEBUG",
		GCInterval:              1 * time.Minute,
		GCParallelDestroys:      2,
		GCDiskUsageThreshold:    80,
		GCInodeUsageThreshold:   70,
		GCMaxAllocs:             50,
		NoHostUUID:              true,
	}
}

// Read returns the specified configuration value or "".
func (c *Config) Read(id string) string {
	return c.Options[id]
}

// ReadDefault returns the specified configuration value, or the specified
// default value if none is set.
func (c *Config) ReadDefault(id string, defaultValue string) string {
	val, ok := c.Options[id]
	if !ok {
		return defaultValue
	}
	return val
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

// ReadStringListToMap tries to parse the specified option as a comma separated list.
// If there is an error in parsing, an empty list is returned.
func (c *Config) ReadStringListToMap(key string) map[string]struct{} {
	s := strings.TrimSpace(c.Read(key))
	list := make(map[string]struct{})
	if s != "" {
		for _, e := range strings.Split(s, ",") {
			trimmed := strings.TrimSpace(e)
			list[trimmed] = struct{}{}
		}
	}
	return list
}

// ReadStringListToMap tries to parse the specified option as a comma separated list.
// If there is an error in parsing, an empty list is returned.
func (c *Config) ReadStringListToMapDefault(key, defaultValue string) map[string]struct{} {
	val, ok := c.Options[key]
	if !ok {
		val = defaultValue
	}

	list := make(map[string]struct{})
	if val != "" {
		for _, e := range strings.Split(val, ",") {
			trimmed := strings.TrimSpace(e)
			list[trimmed] = struct{}{}
		}
	}
	return list
}

// TLSConfig returns a TLSUtil Config based on the client configuration
func (c *Config) TLSConfiguration() *tlsutil.Config {
	tlsConf := &tlsutil.Config{
		VerifyIncoming:       true,
		VerifyOutgoing:       true,
		VerifyServerHostname: c.TLSConfig.VerifyServerHostname,
		CAFile:               c.TLSConfig.CAFile,
		CertFile:             c.TLSConfig.CertFile,
		KeyFile:              c.TLSConfig.KeyFile,
	}
	return tlsConf
}
