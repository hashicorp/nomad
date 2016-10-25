package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/consul-template/signals"
	"github.com/hashicorp/consul-template/watch"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl"
	"github.com/mitchellh/mapstructure"
)

// The pattern to split the config template syntax on
var configTemplateRe = regexp.MustCompile("([a-zA-Z]:)?([^:]+)")

const (
	// DefaultFilePerms are the default file permissions for templates rendered
	// onto disk when a specific file permission has not already been specified.
	DefaultFilePerms = 0644

	// DefaultDedupPrefix is the default prefix used for de-duplication mode
	DefaultDedupPrefix = "consul-template/dedup/"

	// DefaultCommandTimeout is the amount of time to wait for a command to return.
	DefaultCommandTimeout = 30 * time.Second

	// DefaultReloadSignal is the default signal for reload.
	DefaultReloadSignal = syscall.SIGHUP

	// DefaultDumpSignal is the default signal for a core dump.
	DefaultDumpSignal = syscall.SIGQUIT

	// DefaultKillSignal is the default signal for termination.
	DefaultKillSignal = syscall.SIGINT
)

// Config is used to configure Consul Template
type Config struct {
	// Path is the path to this configuration file on disk. This value is not
	// read from disk by rather dynamically populated by the code so the Config
	// has a reference to the path to the file on disk that created it.
	Path string `mapstructure:"-"`

	// Consul is the location of the Consul instance to query (may be an IP
	// address or FQDN) with port.
	Consul string `mapstructure:"consul"`

	// Token is the Consul API token.
	Token string `mapstructure:"token"`

	// ReloadSignal is the signal to listen for a reload event.
	ReloadSignal os.Signal `mapstructure:"reload_signal"`

	// DumpSignal is the signal to listen for a core dump event.
	DumpSignal os.Signal `mapstructure:"dump_signal"`

	// KillSignal is the signal to listen for a graceful terminate event.
	KillSignal os.Signal `mapstructure:"kill_signal"`

	// Auth is the HTTP basic authentication for communicating with Consul.
	Auth *AuthConfig `mapstructure:"auth"`

	// Vault is the configuration for connecting to a vault server.
	Vault *VaultConfig `mapstructure:"vault"`

	// SSL indicates we should use a secure connection while talking to
	// Consul. This requires Consul to be configured to serve HTTPS.
	SSL *SSLConfig `mapstructure:"ssl"`

	// Syslog is the configuration for syslog.
	Syslog *SyslogConfig `mapstructure:"syslog"`

	// Exec is the configuration for exec/supervise mode.
	Exec *ExecConfig `mapstructure:"exec"`

	// MaxStale is the maximum amount of time for staleness from Consul as given
	// by LastContact. If supplied, Consul Template will query all servers instead
	// of just the leader.
	MaxStale time.Duration `mapstructure:"max_stale"`

	// ConfigTemplates is a slice of the ConfigTemplate objects in the config.
	ConfigTemplates []*ConfigTemplate `mapstructure:"template"`

	// Retry is the duration of time to wait between Consul failures.
	Retry time.Duration `mapstructure:"retry"`

	// Wait is the quiescence timers.
	Wait *watch.Wait `mapstructure:"wait"`

	// PidFile is the path on disk where a PID file should be written containing
	// this processes PID.
	PidFile string `mapstructure:"pid_file"`

	// LogLevel is the level with which to log for this config.
	LogLevel string `mapstructure:"log_level"`

	// Deduplicate is used to configure the dedup settings
	Deduplicate *DeduplicateConfig `mapstructure:"deduplicate"`

	// setKeys is the list of config keys that were set by the user.
	setKeys map[string]struct{}
}

// Copy returns a deep copy of the current configuration. This is useful because
// the nested data structures may be shared.
func (c *Config) Copy() *Config {
	config := new(Config)
	config.Path = c.Path
	config.Consul = c.Consul
	config.Token = c.Token
	config.ReloadSignal = c.ReloadSignal
	config.DumpSignal = c.DumpSignal
	config.KillSignal = c.KillSignal

	if c.Auth != nil {
		config.Auth = &AuthConfig{
			Enabled:  c.Auth.Enabled,
			Username: c.Auth.Username,
			Password: c.Auth.Password,
		}
	}

	if c.Vault != nil {
		config.Vault = &VaultConfig{
			Address:     c.Vault.Address,
			Token:       c.Vault.Token,
			UnwrapToken: c.Vault.UnwrapToken,
			RenewToken:  c.Vault.RenewToken,
		}

		if c.Vault.SSL != nil {
			config.Vault.SSL = &SSLConfig{
				Enabled:    c.Vault.SSL.Enabled,
				Verify:     c.Vault.SSL.Verify,
				Cert:       c.Vault.SSL.Cert,
				Key:        c.Vault.SSL.Key,
				CaCert:     c.Vault.SSL.CaCert,
				CaPath:     c.Vault.SSL.CaPath,
				ServerName: c.Vault.SSL.ServerName,
			}
		}
	}

	if c.SSL != nil {
		config.SSL = &SSLConfig{
			Enabled:    c.SSL.Enabled,
			Verify:     c.SSL.Verify,
			Cert:       c.SSL.Cert,
			Key:        c.SSL.Key,
			CaCert:     c.SSL.CaCert,
			CaPath:     c.SSL.CaPath,
			ServerName: c.SSL.ServerName,
		}
	}

	if c.Syslog != nil {
		config.Syslog = &SyslogConfig{
			Enabled:  c.Syslog.Enabled,
			Facility: c.Syslog.Facility,
		}
	}

	if c.Exec != nil {
		config.Exec = &ExecConfig{
			Command:      c.Exec.Command,
			Splay:        c.Exec.Splay,
			ReloadSignal: c.Exec.ReloadSignal,
			KillSignal:   c.Exec.KillSignal,
			KillTimeout:  c.Exec.KillTimeout,
		}
	}

	config.MaxStale = c.MaxStale

	config.ConfigTemplates = make([]*ConfigTemplate, len(c.ConfigTemplates))
	for i, t := range c.ConfigTemplates {
		config.ConfigTemplates[i] = &ConfigTemplate{
			Source:           t.Source,
			Destination:      t.Destination,
			EmbeddedTemplate: t.EmbeddedTemplate,
			Command:          t.Command,
			CommandTimeout:   t.CommandTimeout,
			Perms:            t.Perms,
			Backup:           t.Backup,
			LeftDelim:        t.LeftDelim,
			RightDelim:       t.RightDelim,
			Wait:             t.Wait,
		}
	}

	config.Retry = c.Retry

	if c.Wait != nil {
		config.Wait = &watch.Wait{
			Min: c.Wait.Min,
			Max: c.Wait.Max,
		}
	}

	config.PidFile = c.PidFile
	config.LogLevel = c.LogLevel

	if c.Deduplicate != nil {
		config.Deduplicate = &DeduplicateConfig{
			Enabled: c.Deduplicate.Enabled,
			Prefix:  c.Deduplicate.Prefix,
			TTL:     c.Deduplicate.TTL,
		}
	}

	config.setKeys = c.setKeys

	return config
}

// Merge merges the values in config into this config object. Values in the
// config object overwrite the values in c.
func (c *Config) Merge(config *Config) {
	if config.WasSet("path") {
		c.Path = config.Path
	}

	if config.WasSet("consul") {
		c.Consul = config.Consul
	}

	if config.WasSet("token") {
		c.Token = config.Token
	}

	if config.WasSet("reload_signal") {
		c.ReloadSignal = config.ReloadSignal
	}

	if config.WasSet("dump_signal") {
		c.DumpSignal = config.DumpSignal
	}

	if config.WasSet("kill_signal") {
		c.KillSignal = config.KillSignal
	}

	if config.WasSet("vault") {
		if c.Vault == nil {
			c.Vault = &VaultConfig{}
		}
		if config.WasSet("vault.address") {
			c.Vault.Address = config.Vault.Address
		}
		if config.WasSet("vault.token") {
			c.Vault.Token = config.Vault.Token
		}
		if config.WasSet("vault.unwrap_token") {
			c.Vault.UnwrapToken = config.Vault.UnwrapToken
		}
		if config.WasSet("vault.renew_token") {
			c.Vault.RenewToken = config.Vault.RenewToken
		}
		if config.WasSet("vault.ssl") {
			if c.Vault.SSL == nil {
				c.Vault.SSL = &SSLConfig{}
			}
			if config.WasSet("vault.ssl.verify") {
				c.Vault.SSL.Verify = config.Vault.SSL.Verify
				c.Vault.SSL.Enabled = true
			}
			if config.WasSet("vault.ssl.cert") {
				c.Vault.SSL.Cert = config.Vault.SSL.Cert
				c.Vault.SSL.Enabled = true
			}
			if config.WasSet("vault.ssl.key") {
				c.Vault.SSL.Key = config.Vault.SSL.Key
				c.Vault.SSL.Enabled = true
			}
			if config.WasSet("vault.ssl.ca_cert") {
				c.Vault.SSL.CaCert = config.Vault.SSL.CaCert
				c.Vault.SSL.Enabled = true
			}
			if config.WasSet("vault.ssl.ca_path") {
				c.Vault.SSL.CaPath = config.Vault.SSL.CaPath
				c.Vault.SSL.Enabled = true
			}
			if config.WasSet("vault.ssl.enabled") {
				c.Vault.SSL.Enabled = config.Vault.SSL.Enabled
			}
			if config.WasSet("vault.ssl.server_name") {
				c.Vault.SSL.ServerName = config.Vault.SSL.ServerName
			}
		}
	}

	if config.WasSet("auth") {
		if c.Auth == nil {
			c.Auth = &AuthConfig{}
		}
		if config.WasSet("auth.username") {
			c.Auth.Username = config.Auth.Username
			c.Auth.Enabled = true
		}
		if config.WasSet("auth.password") {
			c.Auth.Password = config.Auth.Password
			c.Auth.Enabled = true
		}
		if config.WasSet("auth.enabled") {
			c.Auth.Enabled = config.Auth.Enabled
		}
	}

	if config.WasSet("ssl") {
		if c.SSL == nil {
			c.SSL = &SSLConfig{}
		}
		if config.WasSet("ssl.verify") {
			c.SSL.Verify = config.SSL.Verify
			c.SSL.Enabled = true
		}
		if config.WasSet("ssl.cert") {
			c.SSL.Cert = config.SSL.Cert
			c.SSL.Enabled = true
		}
		if config.WasSet("ssl.key") {
			c.SSL.Key = config.SSL.Key
			c.SSL.Enabled = true
		}
		if config.WasSet("ssl.ca_cert") {
			c.SSL.CaCert = config.SSL.CaCert
			c.SSL.Enabled = true
		}
		if config.WasSet("ssl.ca_path") {
			c.SSL.CaPath = config.SSL.CaPath
			c.SSL.Enabled = true
		}
		if config.WasSet("ssl.enabled") {
			c.SSL.Enabled = config.SSL.Enabled
		}
		if config.WasSet("ssl.server_name") {
			c.SSL.ServerName = config.SSL.ServerName
		}
	}

	if config.WasSet("syslog") {
		if c.Syslog == nil {
			c.Syslog = &SyslogConfig{}
		}
		if config.WasSet("syslog.facility") {
			c.Syslog.Facility = config.Syslog.Facility
			c.Syslog.Enabled = true
		}
		if config.WasSet("syslog.enabled") {
			c.Syslog.Enabled = config.Syslog.Enabled
		}
	}

	if config.WasSet("exec") {
		if c.Exec == nil {
			c.Exec = &ExecConfig{}
		}
		if config.WasSet("exec.command") {
			c.Exec.Command = config.Exec.Command
		}
		if config.WasSet("exec.splay") {
			c.Exec.Splay = config.Exec.Splay
		}
		if config.WasSet("exec.reload_signal") {
			c.Exec.ReloadSignal = config.Exec.ReloadSignal
		}
		if config.WasSet("exec.kill_signal") {
			c.Exec.KillSignal = config.Exec.KillSignal
		}
		if config.WasSet("exec.kill_timeout") {
			c.Exec.KillTimeout = config.Exec.KillTimeout
		}
	}

	if config.WasSet("max_stale") {
		c.MaxStale = config.MaxStale
	}

	if len(config.ConfigTemplates) > 0 {
		if c.ConfigTemplates == nil {
			c.ConfigTemplates = make([]*ConfigTemplate, 0, 1)
		}
		for _, template := range config.ConfigTemplates {
			c.ConfigTemplates = append(c.ConfigTemplates, &ConfigTemplate{
				Source:           template.Source,
				Destination:      template.Destination,
				EmbeddedTemplate: template.EmbeddedTemplate,
				Command:          template.Command,
				CommandTimeout:   template.CommandTimeout,
				Perms:            template.Perms,
				Backup:           template.Backup,
				LeftDelim:        template.LeftDelim,
				RightDelim:       template.RightDelim,
				Wait:             template.Wait,
			})
		}
	}

	if config.WasSet("retry") {
		c.Retry = config.Retry
	}

	if config.WasSet("wait") {
		c.Wait = &watch.Wait{
			Min: config.Wait.Min,
			Max: config.Wait.Max,
		}
	}

	if config.WasSet("pid_file") {
		c.PidFile = config.PidFile
	}

	if config.WasSet("log_level") {
		c.LogLevel = config.LogLevel
	}

	if config.WasSet("deduplicate") {
		if c.Deduplicate == nil {
			c.Deduplicate = &DeduplicateConfig{}
		}
		if config.WasSet("deduplicate.enabled") {
			c.Deduplicate.Enabled = config.Deduplicate.Enabled
		}
		if config.WasSet("deduplicate.prefix") {
			c.Deduplicate.Prefix = config.Deduplicate.Prefix
		}
	}

	if c.setKeys == nil {
		c.setKeys = make(map[string]struct{})
	}
	for k := range config.setKeys {
		if _, ok := c.setKeys[k]; !ok {
			c.setKeys[k] = struct{}{}
		}
	}
}

// WasSet determines if the given key was set in the config (as opposed to just
// having the default value).
func (c *Config) WasSet(key string) bool {
	if _, ok := c.setKeys[key]; ok {
		return true
	}
	return false
}

// Set is a helper function for marking a key as set.
func (c *Config) Set(key string) {
	if c.setKeys == nil {
		c.setKeys = make(map[string]struct{})
	}
	if _, ok := c.setKeys[key]; !ok {
		c.setKeys[key] = struct{}{}
	}
}

// Parse parses the given string contents as a config
func Parse(s string) (*Config, error) {
	var errs *multierror.Error

	// Parse the file (could be HCL or JSON)
	var shadow interface{}
	if err := hcl.Decode(&shadow, s); err != nil {
		return nil, fmt.Errorf("error decoding config: %s", err)
	}

	// Convert to a map and flatten the keys we want to flatten
	parsed, ok := shadow.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("error converting config")
	}
	flattenKeys(parsed, []string{
		"auth",
		"ssl",
		"syslog",
		"exec",
		"vault",
		"deduplicate",
	})

	// Deprecations
	if vault, ok := parsed["vault"].(map[string]interface{}); ok {
		if val, ok := vault["renew"]; ok {
			log.Println(`[WARN] vault.renew has been renamed to vault.renew_token. ` +
				`Update your configuration files and change "renew" to "renew_token".`)
			vault["renew_token"] = val
			delete(vault, "renew")
		}
	}

	// Create a new, empty config
	config := new(Config)

	// Use mapstructure to populate the basic config fields
	metadata := new(mapstructure.Metadata)
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			StringToFileModeFunc(),
			signals.StringToSignalFunc(),
			watch.StringToWaitDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
			mapstructure.StringToTimeDurationHookFunc(),
		),
		ErrorUnused: true,
		Metadata:    metadata,
		Result:      config,
	})
	if err != nil {
		errs = multierror.Append(errs, err)
		return nil, errs.ErrorOrNil()
	}
	if err := decoder.Decode(parsed); err != nil {
		errs = multierror.Append(errs, err)
		return nil, errs.ErrorOrNil()
	}

	// Explicitly check for the nil signal and set the value back to nil
	if config.ReloadSignal == signals.SIGNIL {
		config.ReloadSignal = nil
	}
	if config.DumpSignal == signals.SIGNIL {
		config.DumpSignal = nil
	}
	if config.KillSignal == signals.SIGNIL {
		config.KillSignal = nil
	}
	if config.Exec != nil {
		if config.Exec.ReloadSignal == signals.SIGNIL {
			config.Exec.ReloadSignal = nil
		}
		if config.Exec.KillSignal == signals.SIGNIL {
			config.Exec.KillSignal = nil
		}
	}

	// Setup default values for templates
	for _, t := range config.ConfigTemplates {
		// Ensure there's a default value for the template's file permissions
		if t.Perms == 0000 {
			t.Perms = DefaultFilePerms
		}

		// Ensure we have a default command timeout
		if t.CommandTimeout == 0 {
			t.CommandTimeout = DefaultCommandTimeout
		}

		// Set up a default zero wait, which disables it for this
		// template.
		if t.Wait == nil {
			t.Wait = &watch.Wait{}
		}
	}

	// Update the list of set keys
	if config.setKeys == nil {
		config.setKeys = make(map[string]struct{})
	}
	for _, key := range metadata.Keys {
		if _, ok := config.setKeys[key]; !ok {
			config.setKeys[key] = struct{}{}
		}
	}
	config.setKeys["path"] = struct{}{}

	d := DefaultConfig()
	d.Merge(config)
	config = d

	return config, errs.ErrorOrNil()
}

// Must returns a config object that must compile. If there are any errors, this
// function will panic. This is most useful in testing or constants.
func Must(s string) *Config {
	c, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return c
}

// FromFile reads the configuration file at the given path and returns a new
// Config struct with the data populated.
func FromFile(path string) (*Config, error) {
	c, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config at %q: %s", path, err)
	}

	return Parse(string(c))
}

// FromPath iterates and merges all configuration files in a given
// directory, returning the resulting config.
func FromPath(path string) (*Config, error) {
	// Ensure the given filepath exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("config: missing file/folder: %s", path)
	}

	// Check if a file was given or a path to a directory
	stat, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("config: error stating file: %s", err)
	}

	// Recursively parse directories, single load files
	if stat.Mode().IsDir() {
		// Ensure the given filepath has at least one config file
		_, err := ioutil.ReadDir(path)
		if err != nil {
			return nil, fmt.Errorf("config: error listing directory: %s", err)
		}

		// Create a blank config to merge off of
		config := DefaultConfig()

		// Potential bug: Walk does not follow symlinks!
		err = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
			// If WalkFunc had an error, just return it
			if err != nil {
				return err
			}

			// Do nothing for directories
			if info.IsDir() {
				return nil
			}

			// Parse and merge the config
			newConfig, err := FromFile(path)
			if err != nil {
				return err
			}
			config.Merge(newConfig)

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("config: walk error: %s", err)
		}

		return config, nil
	} else if stat.Mode().IsRegular() {
		return FromFile(path)
	}

	return nil, fmt.Errorf("config: unknown filetype: %q", stat.Mode().String())
}

// DefaultConfig returns the default configuration struct.
func DefaultConfig() *Config {
	logLevel := os.Getenv("CONSUL_TEMPLATE_LOG")
	if logLevel == "" {
		logLevel = "WARN"
	}

	config := &Config{
		Vault: &VaultConfig{
			RenewToken: true,
			SSL: &SSLConfig{
				Enabled: true,
				Verify:  true,
			},
		},
		Auth: &AuthConfig{
			Enabled: false,
		},
		ReloadSignal: DefaultReloadSignal,
		DumpSignal:   DefaultDumpSignal,
		KillSignal:   DefaultKillSignal,
		SSL: &SSLConfig{
			Enabled: false,
			Verify:  true,
		},
		Syslog: &SyslogConfig{
			Enabled:  false,
			Facility: "LOCAL0",
		},
		Deduplicate: &DeduplicateConfig{
			Enabled: false,
			Prefix:  DefaultDedupPrefix,
			TTL:     15 * time.Second,
		},
		Exec: &ExecConfig{
			KillSignal:  syscall.SIGTERM,
			KillTimeout: 30 * time.Second,
		},
		ConfigTemplates: make([]*ConfigTemplate, 0),
		Retry:           5 * time.Second,
		MaxStale:        1 * time.Second,
		Wait:            &watch.Wait{},
		LogLevel:        logLevel,
		setKeys:         make(map[string]struct{}),
	}

	if v := os.Getenv("CONSUL_HTTP_ADDR"); v != "" {
		config.Consul = v
	}

	if v := os.Getenv("CONSUL_TOKEN"); v != "" {
		config.Token = v
	}

	if v := os.Getenv("VAULT_ADDR"); v != "" {
		config.Vault.Address = v
	}

	if v := os.Getenv("VAULT_TOKEN"); v != "" {
		config.Vault.Token = v
	}

	if v := os.Getenv("VAULT_UNWRAP_TOKEN"); v != "" {
		config.Vault.UnwrapToken = true
	}

	if v := os.Getenv("VAULT_CAPATH"); v != "" {
		config.Vault.SSL.Cert = v
	}

	if v := os.Getenv("VAULT_CACERT"); v != "" {
		config.Vault.SSL.CaCert = v
	}

	if v := os.Getenv("VAULT_SKIP_VERIFY"); v != "" {
		config.Vault.SSL.Verify = false
	}

	if v := os.Getenv("VAULT_TLS_SERVER_NAME"); v != "" {
		config.Vault.SSL.ServerName = v
	}

	return config
}

// AuthConfig is the HTTP basic authentication data.
type AuthConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// String is the string representation of this authentication. If authentication
// is not enabled, this returns the empty string. The username and password will
// be separated by a colon.
func (a *AuthConfig) String() string {
	if !a.Enabled {
		return ""
	}

	if a.Password != "" {
		return fmt.Sprintf("%s:%s", a.Username, a.Password)
	}

	return a.Username
}

// ExecConfig is used to configure the application when it runs in
// exec/supervise mode.
type ExecConfig struct {
	// Command is the command to execute and watch as a child process.
	Command string `mapstructure:"command"`

	// Splay is the maximum amount of time to wait to kill the process.
	Splay time.Duration `mapstructure:"splay"`

	// ReloadSignal is the signal to send to the child process when a template
	// changes. This tells the child process that templates have
	ReloadSignal os.Signal `mapstructure:"reload_signal"`

	// KillSignal is the signal to send to the command to kill it gracefully. The
	// default value is "SIGTERM".
	KillSignal os.Signal `mapstructure:"kill_signal"`

	// KillTimeout is the amount of time to give the process to cleanup before
	// hard-killing it.
	KillTimeout time.Duration `mapstructure:"kill_timeout"`
}

// DeduplicateConfig is used to enable the de-duplication mode, which depends
// on electing a leader per-template and watching of a key. This is used
// to reduce the cost of many instances of CT running the same template.
type DeduplicateConfig struct {
	// Controls if deduplication mode is enabled
	Enabled bool `mapstructure:"enabled"`

	// Controls the KV prefix used. Defaults to defaultDedupPrefix
	Prefix string `mapstructure:"prefix"`

	// TTL is the Session TTL used for lock acquisition, defaults to 15 seconds.
	TTL time.Duration `mapstructure:"ttl"`
}

// SSLConfig is the configuration for SSL.
type SSLConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Verify     bool   `mapstructure:"verify"`
	Cert       string `mapstructure:"cert"`
	Key        string `mapstructure:"key"`
	CaCert     string `mapstructure:"ca_cert"`
	CaPath     string `mapstructure:"ca_path"`
	ServerName string `mapstructure:"server_name"`
}

// SyslogConfig is the configuration for syslog.
type SyslogConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Facility string `mapstructure:"facility"`
}

// ConfigTemplate is the representation of an input template, output location,
// and optional command to execute when rendered
type ConfigTemplate struct {
	Source           string        `mapstructure:"source"`
	Destination      string        `mapstructure:"destination"`
	EmbeddedTemplate string        `mapstructure:"contents"`
	Command          string        `mapstructure:"command"`
	CommandTimeout   time.Duration `mapstructure:"command_timeout"`
	Perms            os.FileMode   `mapstructure:"perms"`
	Backup           bool          `mapstructure:"backup"`
	LeftDelim        string        `mapstructure:"left_delimiter"`
	RightDelim       string        `mapstructure:"right_delimiter"`
	Wait             *watch.Wait   `mapstructure:"wait"`
}

// VaultConfig is the configuration for connecting to a vault server.
type VaultConfig struct {
	Address     string `mapstructure:"address"`
	Token       string `mapstructure:"token" json:"-"`
	UnwrapToken bool   `mapstructure:"unwrap_token"`
	RenewToken  bool   `mapstructure:"renew_token"`

	// SSL indicates we should use a secure connection while talking to Vault.
	SSL *SSLConfig `mapstructure:"ssl"`
}

// ParseConfigTemplate parses a string into a ConfigTemplate struct
func ParseConfigTemplate(s string) (*ConfigTemplate, error) {
	if len(strings.TrimSpace(s)) < 1 {
		return nil, errors.New("cannot specify empty template declaration")
	}

	var source, destination, command string
	parts := configTemplateRe.FindAllString(s, -1)

	switch len(parts) {
	case 1:
		source = parts[0]
	case 2:
		source, destination = parts[0], parts[1]
	case 3:
		source, destination, command = parts[0], parts[1], parts[2]
	default:
		return nil, errors.New("invalid template declaration format")
	}

	return &ConfigTemplate{
		Source:         source,
		Destination:    destination,
		Command:        command,
		CommandTimeout: DefaultCommandTimeout,
		Perms:          DefaultFilePerms,
		Wait:           &watch.Wait{},
	}, nil
}

// flattenKeys is a function that takes a map[string]interface{} and recursively
// flattens any keys that are a []map[string]interface{} where the key is in the
// given list of keys.
func flattenKeys(m map[string]interface{}, keys []string) {
	keyMap := make(map[string]struct{})
	for _, key := range keys {
		keyMap[key] = struct{}{}
	}

	var flatten func(map[string]interface{})
	flatten = func(m map[string]interface{}) {
		for k, v := range m {
			if _, ok := keyMap[k]; !ok {
				continue
			}

			switch typed := v.(type) {
			case []map[string]interface{}:
				if len(typed) > 0 {
					last := typed[len(typed)-1]
					flatten(last)
					m[k] = last
				} else {
					m[k] = nil
				}
			case map[string]interface{}:
				flatten(typed)
				m[k] = typed
			default:
				m[k] = v
			}
		}
	}

	flatten(m)
}
