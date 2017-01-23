package config

import (
	"fmt"
	"time"

	"github.com/hashicorp/vault/api"
)

const (
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

	// UnwrapToken unwraps the provided Vault token as a wrapped token.
	UnwrapToken *bool `mapstructure:"unwrap_token"`
}

// DefaultVaultConfig returns a configuration that is populated with the
// default values.
func DefaultVaultConfig() *VaultConfig {
	v := &VaultConfig{
		Address:     stringFromEnv(api.EnvVaultAddress),
		RenewToken:  boolFromEnv("VAULT_RENEW_TOKEN"),
		UnwrapToken: boolFromEnv("VAULT_UNWRAP_TOKEN"),
		Retry:       DefaultRetryConfig(),
		SSL: &SSLConfig{
			CaCert:     stringFromEnv(api.EnvVaultCACert),
			CaPath:     stringFromEnv(api.EnvVaultCAPath),
			Cert:       stringFromEnv(api.EnvVaultClientCert),
			Key:        stringFromEnv(api.EnvVaultClientKey),
			ServerName: stringFromEnv(api.EnvVaultTLSServerName),
			Verify:     antiboolFromEnv(api.EnvVaultInsecure),
		},
		Token: stringFromEnv("VAULT_TOKEN"),
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

	o.RenewToken = c.RenewToken

	if c.Retry != nil {
		o.Retry = c.Retry.Copy()
	}

	if c.SSL != nil {
		o.SSL = c.SSL.Copy()
	}

	o.Token = c.Token

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

	if o.UnwrapToken != nil {
		r.UnwrapToken = o.UnwrapToken
	}

	return r
}

// Finalize ensures there no nil pointers.
func (c *VaultConfig) Finalize() {
	if c.Enabled == nil {
		c.Enabled = Bool(StringPresent(c.Address))
	}

	if c.Address == nil {
		c.Address = String("")
	}

	if c.RenewToken == nil {
		c.RenewToken = Bool(DefaultVaultRenewToken)
	}

	if c.Retry == nil {
		c.Retry = DefaultRetryConfig()
	}
	c.Retry.Finalize()

	if c.SSL == nil {
		c.SSL = DefaultSSLConfig()
	}
	c.SSL.Finalize()

	if c.Token == nil {
		c.Token = String("")
	}

	if c.UnwrapToken == nil {
		c.UnwrapToken = Bool(DefaultVaultUnwrapToken)
	}
}

// GoString defines the printable version of this struct.
func (c *VaultConfig) GoString() string {
	if c == nil {
		return "(*VaultConfig)(nil)"
	}

	return fmt.Sprintf("&VaultConfig{"+
		"Enabled:%s, "+
		"Address:%s, "+
		"Token:%t, "+
		"UnwrapToken:%s, "+
		"RenewToken:%s, "+
		"Retry:%#v, "+
		"SSL:%#v"+
		"}",
		BoolGoString(c.Enabled),
		StringGoString(c.Address),
		StringPresent(c.Token),
		BoolGoString(c.UnwrapToken),
		BoolGoString(c.RenewToken),
		c.Retry,
		c.SSL,
	)
}
