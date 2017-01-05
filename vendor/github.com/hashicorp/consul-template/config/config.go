package config

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/consul-template/signals"
	"github.com/hashicorp/hcl"
	"github.com/mitchellh/mapstructure"

	"github.com/pkg/errors"
)

const (
	// DefaultLogLevel is the default logging level.
	DefaultLogLevel = "WARN"

	// DefaultMaxStale is the default staleness permitted. This enables stale
	// queries by default for performance reasons.
	DefaultMaxStale = 2 * time.Second

	// DefaultReloadSignal is the default signal for reload.
	DefaultReloadSignal = syscall.SIGHUP

	// DefaultRetry is the default amount of time to sleep before retrying.
	DefaultRetry = 5 * time.Second

	// DefaultKillSignal is the default signal for termination.
	DefaultKillSignal = syscall.SIGINT
)

// Config is used to configure Consul Template
type Config struct {
	// Auth is the HTTP basic authentication for communicating with Consul.
	Auth *AuthConfig `mapstructure:"auth"`

	// Consul is the location of the Consul instance to query (may be an IP
	// address or FQDN) with port.
	Consul *string `mapstructure:"consul"`

	// Dedup is used to configure the dedup settings
	Dedup *DedupConfig `mapstructure:"deduplicate"`

	// Exec is the configuration for exec/supervise mode.
	Exec *ExecConfig `mapstructure:"exec"`

	// KillSignal is the signal to listen for a graceful terminate event.
	KillSignal *os.Signal `mapstructure:"kill_signal"`

	// LogLevel is the level with which to log for this config.
	LogLevel *string `mapstructure:"log_level"`

	// MaxStale is the maximum amount of time for staleness from Consul as given
	// by LastContact. If supplied, Consul Template will query all servers instead
	// of just the leader.
	MaxStale *time.Duration `mapstructure:"max_stale"`

	// PidFile is the path on disk where a PID file should be written containing
	// this processes PID.
	PidFile *string `mapstructure:"pid_file"`

	// ReloadSignal is the signal to listen for a reload event.
	ReloadSignal *os.Signal `mapstructure:"reload_signal"`

	// Retry is the duration of time to wait between Consul failures.
	Retry *time.Duration `mapstructure:"retry"`

	// SSL indicates we should use a secure connection while talking to
	// Consul. This requires Consul to be configured to serve HTTPS.
	SSL *SSLConfig `mapstructure:"ssl"`

	// Syslog is the configuration for syslog.
	Syslog *SyslogConfig `mapstructure:"syslog"`

	// Templates is the list of templates.
	Templates *TemplateConfigs `mapstructure:"template"`

	// Token is the Consul API token.
	Token *string `mapstructure:"token"`

	// Vault is the configuration for connecting to a vault server.
	Vault *VaultConfig `mapstructure:"vault"`

	// Wait is the quiescence timers.
	Wait *WaitConfig `mapstructure:"wait"`
}

// Copy returns a deep copy of the current configuration. This is useful because
// the nested data structures may be shared.
func (c *Config) Copy() *Config {
	var o Config

	if c.Auth != nil {
		o.Auth = c.Auth.Copy()
	}

	o.Consul = c.Consul

	if c.Dedup != nil {
		o.Dedup = c.Dedup.Copy()
	}

	if c.Exec != nil {
		o.Exec = c.Exec.Copy()
	}

	o.KillSignal = c.KillSignal

	o.LogLevel = c.LogLevel

	o.MaxStale = c.MaxStale

	o.PidFile = c.PidFile

	o.ReloadSignal = c.ReloadSignal

	o.Retry = c.Retry

	if c.SSL != nil {
		o.SSL = c.SSL.Copy()
	}

	if c.Syslog != nil {
		o.Syslog = c.Syslog.Copy()
	}

	if c.Templates != nil {
		o.Templates = c.Templates.Copy()
	}

	o.Token = c.Token

	if c.Vault != nil {
		o.Vault = c.Vault.Copy()
	}

	if c.Wait != nil {
		o.Wait = c.Wait.Copy()
	}

	return &o
}

// Merge merges the values in config into this config object. Values in the
// config object overwrite the values in c.
func (c *Config) Merge(o *Config) *Config {
	if c == nil {
		if o == nil {
			return nil
		}
		return o.Copy()
	}

	if o == nil {
		return c.Copy()
	}

	r := c.Copy()

	if o.Auth != nil {
		r.Auth = r.Auth.Merge(o.Auth)
	}

	if o.Consul != nil {
		r.Consul = o.Consul
	}

	if o.Dedup != nil {
		r.Dedup = r.Dedup.Merge(o.Dedup)
	}

	if o.Exec != nil {
		r.Exec = r.Exec.Merge(o.Exec)
	}

	if o.KillSignal != nil {
		r.KillSignal = o.KillSignal
	}

	if o.LogLevel != nil {
		r.LogLevel = o.LogLevel
	}

	if o.MaxStale != nil {
		r.MaxStale = o.MaxStale
	}

	if o.PidFile != nil {
		r.PidFile = o.PidFile
	}

	if o.ReloadSignal != nil {
		r.ReloadSignal = o.ReloadSignal
	}

	if o.Retry != nil {
		r.Retry = o.Retry
	}

	if o.SSL != nil {
		r.SSL = r.SSL.Merge(o.SSL)
	}

	if o.Syslog != nil {
		r.Syslog = r.Syslog.Merge(o.Syslog)
	}

	if o.Templates != nil {
		r.Templates = r.Templates.Merge(o.Templates)
	}

	if o.Token != nil {
		r.Token = o.Token
	}

	if o.Vault != nil {
		r.Vault = r.Vault.Merge(o.Vault)
	}

	if o.Wait != nil {
		r.Wait = r.Wait.Merge(o.Wait)
	}

	return r
}

// Parse parses the given string contents as a config
func Parse(s string) (*Config, error) {
	var shadow interface{}
	if err := hcl.Decode(&shadow, s); err != nil {
		return nil, errors.Wrap(err, "error decoding config")
	}

	// Convert to a map and flatten the keys we want to flatten
	parsed, ok := shadow.(map[string]interface{})
	if !ok {
		return nil, errors.New("error converting config")
	}

	flattenKeys(parsed, []string{
		"auth",
		"deduplicate",
		"env",
		"exec",
		"exec.env",
		"ssl",
		"syslog",
		"vault",
		"vault.ssl",
		"wait",
	})

	// FlattenFlatten keys belonging to the templates. We cannot do this above
	// because it is an array of tmeplates.
	if templates, ok := parsed["template"].([]map[string]interface{}); ok {
		for _, template := range templates {
			flattenKeys(template, []string{
				"env",
				"exec",
				"exec.env",
				"wait",
			})
		}
	}

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
	var c Config

	// Use mapstructure to populate the basic config fields
	var md mapstructure.Metadata
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			StringToFileModeFunc(),
			signals.StringToSignalFunc(),
			StringToWaitDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
			mapstructure.StringToTimeDurationHookFunc(),
		),
		ErrorUnused: true,
		Metadata:    &md,
		Result:      &c,
	})
	if err != nil {
		return nil, errors.Wrap(err, "mapstructure decoder creation failed")
	}
	if err := decoder.Decode(parsed); err != nil {
		return nil, errors.Wrap(err, "mapstructure decode failed")
	}

	return &c, nil
}

// Must returns a config object that must compile. If there are any errors, this
// function will panic. This is most useful in testing or constants.
func Must(s string) *Config {
	c, err := Parse(s)
	if err != nil {
		log.Fatal(err)
	}
	return c
}

// TestConfig returuns a default, finalized config, with the provided
// configuration taking precedence.
func TestConfig(c *Config) *Config {
	d := DefaultConfig().Merge(c)
	d.Finalize()
	return d
}

// FromFile reads the configuration file at the given path and returns a new
// Config struct with the data populated.
func FromFile(path string) (*Config, error) {
	c, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("from file %s", path))
	}
	return Parse(string(c))
}

// FromPath iterates and merges all configuration files in a given
// directory, returning the resulting config.
func FromPath(path string) (*Config, error) {
	// Ensure the given filepath exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, errors.Wrap(err, "missing file/folder"+path)
	}

	// Check if a file was given or a path to a directory
	stat, err := os.Stat(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed stating file "+path)
	}

	// Recursively parse directories, single load files
	if stat.Mode().IsDir() {
		// Ensure the given filepath has at least one config file
		_, err := ioutil.ReadDir(path)
		if err != nil {
			return nil, errors.Wrap(err, "failed listing dir "+path)
		}

		// Create a blank config to merge off of
		var c *Config

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
			c = c.Merge(newConfig)

			return nil
		})

		if err != nil {
			return nil, errors.Wrap(err, "walk error")
		}

		return c, nil
	} else if stat.Mode().IsRegular() {
		return FromFile(path)
	}

	return nil, fmt.Errorf("unknown filetype: %q", stat.Mode().String())
}

// GoString defines the printable version of this struct.
func (c *Config) GoString() string {
	if c == nil {
		return "(*Config)(nil)"
	}

	return fmt.Sprintf("&Config{"+
		"Auth:%#v, "+
		"Consul:%s, "+
		"Dedup:%#v, "+
		"Exec:%#v, "+
		"KillSignal:%s, "+
		"LogLevel:%s, "+
		"MaxStale:%s, "+
		"PidFile:%s, "+
		"ReloadSignal:%s, "+
		"Retry:%s, "+
		"SSL:%#v, "+
		"Syslog:%#v, "+
		"Templates:%#v, "+
		"Token:%s, "+
		"Vault:%#v, "+
		"Wait:%#v"+
		"}",
		c.Auth,
		StringGoString(c.Consul),
		c.Dedup,
		c.Exec,
		SignalGoString(c.KillSignal),
		StringGoString(c.LogLevel),
		TimeDurationGoString(c.MaxStale),
		StringGoString(c.PidFile),
		SignalGoString(c.ReloadSignal),
		TimeDurationGoString(c.Retry),
		c.SSL,
		c.Syslog,
		c.Templates,
		StringGoString(c.Token),
		c.Vault,
		c.Wait,
	)
}

// DefaultConfig returns the default configuration struct. Certain environment
// variables may be set which control the values for the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Auth:         DefaultAuthConfig(),
		Consul:       stringFromEnv("CONSUL_HTTP_ADDR"),
		Dedup:        DefaultDedupConfig(),
		Exec:         DefaultExecConfig(),
		KillSignal:   Signal(DefaultKillSignal),
		LogLevel:     stringFromEnv("CT_LOG", "CONSUL_TEMPLATE_LOG"),
		MaxStale:     TimeDuration(DefaultMaxStale),
		PidFile:      String(""),
		ReloadSignal: Signal(DefaultReloadSignal),
		Retry:        TimeDuration(DefaultRetry),
		SSL:          DefaultSSLConfig(),
		Syslog:       DefaultSyslogConfig(),
		Templates:    DefaultTemplateConfigs(),
		Token:        stringFromEnv("CONSUL_TOKEN", "CONSUL_HTTP_TOKEN"),
		Vault:        DefaultVaultConfig(),
		Wait:         DefaultWaitConfig(),
	}
}

// Finalize ensures all configuration options have the default values, so it
// is safe to dereference the pointers later down the line. It also
// intelligently tries to activate stanzas that should be "enabled" because
// data was given, but the user did not explicitly add "Enabled: true" to the
// configuration.
func (c *Config) Finalize() {
	if c.Auth == nil {
		c.Auth = DefaultAuthConfig()
	}
	c.Auth.Finalize()

	if c.Consul == nil {
		c.Consul = String("")
	}

	if c.Dedup == nil {
		c.Dedup = DefaultDedupConfig()
	}
	c.Dedup.Finalize()

	if c.Exec == nil {
		c.Exec = DefaultExecConfig()
	}
	c.Exec.Finalize()

	if c.KillSignal == nil {
		c.KillSignal = Signal(DefaultKillSignal)
	}

	if c.LogLevel == nil {
		c.LogLevel = String(DefaultLogLevel)
	}

	if c.MaxStale == nil {
		c.MaxStale = TimeDuration(DefaultMaxStale)
	}

	if c.PidFile == nil {
		c.PidFile = String("")
	}

	if c.ReloadSignal == nil {
		c.ReloadSignal = Signal(DefaultReloadSignal)
	}

	if c.Retry == nil {
		c.Retry = TimeDuration(DefaultRetry)
	}

	if c.SSL == nil {
		c.SSL = DefaultSSLConfig()
	}
	c.SSL.Finalize()

	if c.Syslog == nil {
		c.Syslog = DefaultSyslogConfig()
	}
	c.Syslog.Finalize()

	if c.Templates == nil {
		c.Templates = DefaultTemplateConfigs()
	}
	c.Templates.Finalize()

	if c.Token == nil {
		c.Token = String("")
	}

	if c.Vault == nil {
		c.Vault = DefaultVaultConfig()
	}
	c.Vault.Finalize()

	if c.Wait == nil {
		c.Wait = DefaultWaitConfig()
	}
	c.Wait.Finalize()
}

func stringFromEnv(list ...string) *string {
	for _, s := range list {
		if v := os.Getenv(s); v != "" {
			return String(strings.TrimSpace(v))
		}
	}
	return nil
}

func antiboolFromEnv(s string) *bool {
	if b := boolFromEnv(s); b != nil {
		return Bool(!*b)
	}
	return nil
}

func boolFromEnv(s string) *bool {
	if v := os.Getenv(s); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return Bool(b)
		}
	}
	return nil
}

// flattenKeys is a function that takes a map[string]interface{} and recursively
// flattens any keys that are a []map[string]interface{} where the key is in the
// given list of keys.
func flattenKeys(m map[string]interface{}, keys []string) {
	keyMap := make(map[string]struct{})
	for _, key := range keys {
		keyMap[key] = struct{}{}
	}

	var flatten func(map[string]interface{}, string)
	flatten = func(m map[string]interface{}, parent string) {
		for k, v := range m {
			// Calculate the map key, since it could include a parent.
			mapKey := k
			if parent != "" {
				mapKey = parent + "." + k
			}

			if _, ok := keyMap[mapKey]; !ok {
				continue
			}

			switch typed := v.(type) {
			case []map[string]interface{}:
				if len(typed) > 0 {
					last := typed[len(typed)-1]
					flatten(last, mapKey)
					m[k] = last
				} else {
					m[k] = nil
				}
			case map[string]interface{}:
				flatten(typed, mapKey)
				m[k] = typed
			default:
				m[k] = v
			}
		}
	}

	flatten(m, "")
}
