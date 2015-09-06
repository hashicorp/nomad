package agent

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl"
	hclobj "github.com/hashicorp/hcl/hcl"
	client "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad"
)

// Config is the configuration for the Nomad agent.
type Config struct {
	// Region is the region this agent is in. Defaults to region1.
	Region string `hcl:"region"`

	// Datacenter is the datacenter this agent is in. Defaults to dc1
	Datacenter string `hcl:"datacenter"`

	// NodeName is the name we register as. Defaults to hostname.
	NodeName string `hcl:"name"`

	// DataDir is the directory to store our state in
	DataDir string `hcl:"data_dir"`

	// LogLevel is the level of the logs to putout
	LogLevel string `hcl:"log_level"`

	// HttpAddr is used to control the address and port we bind to.
	// If not specified, 127.0.0.1:4646 is used.
	HttpAddr string `hcl:"http_addr"`

	// EnableDebug is used to enable debugging HTTP endpoints
	EnableDebug bool `hcl:"enable_debug"`

	// Client has our client related settings
	Client *ClientConfig `hcl:"client"`

	// Server has our server related settings
	Server *ServerConfig `hcl:"server"`

	Telemetry *Telemetry `hcl:"telemetry"`

	LeaveOnInt     bool
	LeaveOnTerm    bool
	EnableSyslog   bool
	SyslogFacility string

	DisableUpdateCheck        bool
	DisableAnonymousSignature bool

	Revision          string
	Version           string
	VersionPrerelease string

	DevMode bool `hcl:"-"`

	// NomadConfig is used to override the default config.
	// This is largly used for testing purposes.
	NomadConfig *nomad.Config `hcl:"-" json:"-"`

	// ClientConfig is used to override the default config.
	// This is largly used for testing purposes.
	ClientConfig *client.Config `hcl:"-" json:"-"`
}

type ClientConfig struct {
	// Enabled controls if we are a client
	Enabled bool `hcl:"enabled"`

	// StateDir is the state directory
	StateDir string `hcl:"state_dir"`

	// AllocDir is the directory for storing allocation data
	AllocDir string `hcl:"alloc_dir"`

	// Servers is a list of known server addresses. These are as "host:port"
	Servers []string `hcl:"servers"`

	// NodeID is the unique node identifier to use. A UUID is used
	// if not provided, and stored in the data directory
	NodeID string `hcl:"node_id"`

	// NodeClass is used to group the node by class
	NodeClass string `hcl:"node_class"`

	// Metadata associated with the node
	Meta map[string]string `hcl:"meta"`
}

type ServerConfig struct {
	// Enabled controls if we are a server
	Enabled bool `hcl:"enabled"`

	// Bootstrap is used to bring up the first Consul server, and
	// permits that node to elect itself leader
	Bootstrap bool `hcl:"bootstrap"`

	// BootstrapExpect tries to automatically bootstrap the Consul cluster,
	// by witholding peers until enough servers join.
	BootstrapExpect int `hcl:"bootstrap_expect"`

	// DataDir is the directory to store our state in
	DataDir string `hcl:"data_dir"`

	// ProtocolVersion is the protocol version to speak. This must be between
	// ProtocolVersionMin and ProtocolVersionMax.
	ProtocolVersion int `hcl:"protocol_version"`

	// AdvertiseAddr is the address we use for advertising our Serf,
	// and Consul RPC IP. If not specified, bind address is used.
	AdvertiseAddr string `mapstructure:"advertise_addr"`

	// BindAddr is used to control the address we bind to.
	// If not specified, the first private IP we find is used.
	// This controls the address we use for cluster facing
	// services (Gossip, Server RPC)
	BindAddr string `hcl:"bind_addr"`

	// NumSchedulers is the number of scheduler thread that are run.
	// This can be as many as one per core, or zero to disable this server
	// from doing any scheduling work.
	NumSchedulers int `hcl:"num_schedulers"`

	// EnabledSchedulers controls the set of sub-schedulers that are
	// enabled for this server to handle. This will restrict the evaluations
	// that the workers dequeue for processing.
	EnabledSchedulers []string `hcl:"enabled_schedulers"`
}

// Telemetry is the telemetry configuration for the server
type Telemetry struct {
	StatsiteAddr    string `hcl:"statsite_address"`
	StatsdAddr      string `hcl:"statsd_address"`
	DisableHostname bool   `hcl:"disable_hostname"`
}

// DevConfig is a Config that is used for dev mode of Nomad.
func DevConfig() *Config {
	conf := DefaultConfig()
	conf.LogLevel = "DEBUG"
	conf.Client.Enabled = true
	conf.Server.Enabled = true
	conf.DevMode = true
	conf.EnableDebug = true
	conf.DisableAnonymousSignature = true
	return conf
	return &Config{
		LogLevel:                  "DEBUG",
		DevMode:                   true,
		EnableDebug:               true,
		DisableAnonymousSignature: true,
	}
}

// DefaultConfig is a the baseline configuration for Nomad
func DefaultConfig() *Config {
	return &Config{
		LogLevel:   "INFO",
		Region:     "region1",
		Datacenter: "dc1",
		HttpAddr:   "127.0.0.1:4646",
		Client: &ClientConfig{
			Enabled: false,
		},
		Server: &ServerConfig{
			Enabled: false,
		},
	}
}

// Merge merges two configurations.
func (c *Config) Merge(c2 *Config) *Config {
	result := new(Config)

	result.Telemetry = c.Telemetry
	if c2.Telemetry != nil {
		result.Telemetry = c2.Telemetry
	}
	return result
}

// LoadConfig loads the configuration at the given path, regardless if
// its a file or directory.
func LoadConfig(path string) (*Config, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		return LoadConfigDir(path)
	} else {
		return LoadConfigFile(path)
	}
}

// LoadConfigFile loads the configuration from the given file.
func LoadConfigFile(path string) (*Config, error) {
	// Read the file
	d, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Parse!
	obj, err := hcl.Parse(string(d))
	if err != nil {
		return nil, err
	}

	// Start building the result
	var result Config
	if err := hcl.DecodeObject(&result, obj); err != nil {
		return nil, err
	}

	return &result, nil
}

func getString(o *hclobj.Object) string {
	if o == nil || o.Type != hclobj.ValueTypeString {
		return ""
	}

	return o.Value.(string)
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
			"configuration path must be a directory: %s",
			dir)
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

	var result *Config
	for _, f := range files {
		config, err := LoadConfigFile(f)
		if err != nil {
			return nil, fmt.Errorf("Error loading %s: %s", f, err)
		}

		if result == nil {
			result = config
		} else {
			result = result.Merge(config)
		}
	}

	return result, nil
}

// isTemporaryFile returns true or false depending on whether the
// provided file name is a temporary file for the following editors:
// emacs or vim.
func isTemporaryFile(name string) bool {
	return strings.HasSuffix(name, "~") || // vim
		strings.HasPrefix(name, ".#") || // emacs
		(strings.HasPrefix(name, "#") && strings.HasSuffix(name, "#")) // emacs
}
