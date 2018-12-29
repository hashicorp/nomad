package config

import (
	"fmt"
	"time"

	"github.com/hashicorp/vault/api"
)

const (
	// DefaultVaultGrace is the default grace period before which to read a new
	// secret from Vault. If a lease is due to expire in 15 seconds, Consul
	// Template will read a new secret at that time minus this value.
	DefaultVaultGrace = 15 * time.Second

	// DefaultVaultRenewToken is the default value for if the Vault token should
	// be renewed.
	DefaultVaultRenewToken = true

	// DefaultVaultUnwrapToken is the default value for if the Vault token should
	// be unwrapped.
	DefaultVaultUnwrapToken = false

	// DefaultVaultRetryBase is the default value for the base time to use for
	// exponential backoff.
	DefaultVaultRetryBase = 250 * time.Millisecond

	// DefaultVaultRetryMaxAttempts is the default maximum number of attempts to
	// retry before quitting.
	DefaultVaultRetryMaxAttempts = 5
)

// VaultConfig is the configuration for connecting to a vault server.
type VaultConfig struct {
	// Address is the URI to the Vault server.
	Address *string `mapstructure:"address"`

	// Enabled controls whether the Vault integration is active.
	Enabled *bool `mapstructure:"enabled"`

	// Grace is the amount of time before a lease is about to expire to force a
	// new secret to be read.
	Grace *time.Duration `mapstructure:"grace"`

	// RenewToken renews the Vault token.
	RenewToken *bool `mapstructure:"renew_token"`

	// Retry is the configuration for specifying how to behave on failure.
	Retry *RetryConfig `mapstructure:"retry"`

	// SSL indicates we should use a secure connection while talking to Vault.
	SSL *SSLConfig `mapstructure:"ssl"`

	// Token is the Vault token to communicate with for requests. It may be
	// a wrapped token or a real token. This can also be set via the VAULT_TOKEN
	// environment variable.
	Token *string `mapstructure:"token" json:"-"`

	// Transport configures the low-level network connection details.
	Transport *TransportConfig `mapstructure:"transport"`

	// UnwrapToken unwraps the provided Vault token as a wrapped token.
	UnwrapToken *bool `mapstructure:"unwrap_token"`
}

// DefaultVaultConfig returns a configuration that is populated with the
// default values.
func DefaultVaultConfig() *VaultConfig {
	v := &VaultConfig{
		Retry:     DefaultRetryConfig(),
		SSL:       DefaultSSLConfig(),
		Transport: DefaultTransportConfig(),
	}

	// Force SSL when communicating with Vault.
	v.SSL.Enabled = Bool(true)

	return v
}

// Copy returns a deep copy of this configuration.
func (c *VaultConfig) Copy() *VaultConfig {
	if c == nil {
		return nil
	}

	var o VaultConfig
	o.Address = c.Address

	o.Enabled = c.Enabled

	o.Grace = c.Grace

	o.RenewToken = c.RenewToken

	if c.Retry != nil {
		o.Retry = c.Retry.Copy()
	}

	if c.SSL != nil {
		o.SSL = c.SSL.Copy()
	}

	o.Token = c.Token

	if c.Transport != nil {
		o.Transport = c.Transport.Copy()
	}

	o.UnwrapToken = c.UnwrapToken

	return &o
}

// Merge combines all values in this configuration with the values in the other
// configuration, with values in the other configuration taking precedence.
// Maps and slices are merged, most other values are overwritten. Complex
// structs define their own merge functionality.
func (c *VaultConfig) Merge(o *VaultConfig) *VaultConfig {
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

	if o.Address != nil {
		r.Address = o.Address
	}

	if o.Enabled != nil {
		r.Enabled = o.Enabled
	}

	if o.Grace != nil {
		r.Grace = o.Grace
	}

	if o.RenewToken != nil {
		r.RenewToken = o.RenewToken
	}

	if o.Retry != nil {
		r.Retry = r.Retry.Merge(o.Retry)
	}

	if o.SSL != nil {
		r.SSL = r.SSL.Merge(o.SSL)
	}

	if o.Token != nil {
		r.Token = o.Token
	}

	if o.Transport != nil {
		r.Transport = r.Transport.Merge(o.Transport)
	}

	if o.UnwrapToken != nil {
		r.UnwrapToken = o.UnwrapToken
	}

	return r
}

// Finalize ensures there no nil pointers.
func (c *VaultConfig) Finalize() {
	if c.Address == nil {
		c.Address = stringFromEnv([]string{
			api.EnvVaultAddress,
		}, "")
	}

	if c.Grace == nil {
		c.Grace = TimeDuration(DefaultVaultGrace)
	}

	if c.RenewToken == nil {
		c.RenewToken = boolFromEnv([]string{
			"VAULT_RENEW_TOKEN",
		}, DefaultVaultRenewToken)
	}

	if c.Retry == nil {
		c.Retry = DefaultRetryConfig()
	}
	c.Retry.Finalize()

	// Vault has custom SSL settings
	if c.SSL == nil {
		c.SSL = DefaultSSLConfig()
	}
	if c.SSL.Enabled == nil {
		c.SSL.Enabled = Bool(true)
	}
	if c.SSL.CaCert == nil {
		c.SSL.CaCert = stringFromEnv([]string{api.EnvVaultCACert}, "")
	}
	if c.SSL.CaPath == nil {
		c.SSL.CaPath = stringFromEnv([]string{api.EnvVaultCAPath}, "")
	}
	if c.SSL.Cert == nil {
		c.SSL.Cert = stringFromEnv([]string{api.EnvVaultClientCert}, "")
	}
	if c.SSL.Key == nil {
		c.SSL.Key = stringFromEnv([]string{api.EnvVaultClientKey}, "")
	}
	if c.SSL.ServerName == nil {
		c.SSL.ServerName = stringFromEnv([]string{api.EnvVaultTLSServerName}, "")
	}
	if c.SSL.Verify == nil {
		c.SSL.Verify = antiboolFromEnv([]string{api.EnvVaultInsecure}, true)
	}
	c.SSL.Finalize()

	if c.Token == nil {
		c.Token = stringFromEnv([]string{
			"VAULT_TOKEN",
		}, "")

		if StringVal(c.Token) == "" {
			if homePath != "" {
				c.Token = stringFromFile([]string{
					homePath + "/.vault-token",
				}, "")
			}
		}
	}

	if c.Transport == nil {
		c.Transport = DefaultTransportConfig()
	}
	c.Transport.Finalize()

	if c.UnwrapToken == nil {
		c.UnwrapToken = boolFromEnv([]string{
			"VAULT_UNWRAP_TOKEN",
		}, DefaultVaultUnwrapToken)
	}

	if c.Enabled == nil {
		c.Enabled = Bool(StringPresent(c.Address))
	}
}

// GoString defines the printable version of this struct.
func (c *VaultConfig) GoString() string {
	if c == nil {
		return "(*VaultConfig)(nil)"
	}

	return fmt.Sprintf("&VaultConfig{"+
		"Address:%s, "+
		"Enabled:%s, "+
		"Grace:%s, "+
		"RenewToken:%s, "+
		"Retry:%#v, "+
		"SSL:%#v, "+
		"Token:%t, "+
		"Transport:%#v, "+
		"UnwrapToken:%s"+
		"}",
		StringGoString(c.Address),
		TimeDurationGoString(c.Grace),
		BoolGoString(c.Enabled),
		BoolGoString(c.RenewToken),
		c.Retry,
		c.SSL,
		StringPresent(c.Token),
		c.Transport,
		BoolGoString(c.UnwrapToken),
	)
}
