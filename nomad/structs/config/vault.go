package config

import (
	"time"

	vault "github.com/hashicorp/vault/api"
)

const (
	// DefaultVaultConnectRetryIntv is the retry interval between trying to
	// connect to Vault
	DefaultVaultConnectRetryIntv = 30 * time.Second
)

// VaultConfig contains the configuration information necessary to
// communicate with Vault in order to:
//
// - Renew Vault tokens/leases.
//
// - Pass a token for the Nomad Server to derive sub-tokens.
//
// - Create child tokens with policy subsets of the Server's token.
type VaultConfig struct {

	// Enabled enables or disables Vault support.
	Enabled *bool `mapstructure:"enabled"`

	// Token is the Vault token given to Nomad such that it can
	// derive child tokens. Nomad will renew this token at half its lease
	// lifetime.
	Token string `mapstructure:"token"`

	// AllowUnauthenticated allows users to submit jobs requiring Vault tokens
	// without providing a Vault token proving they have access to these
	// policies.
	AllowUnauthenticated *bool `mapstructure:"allow_unauthenticated"`

	// TaskTokenTTL is the TTL of the tokens created by Nomad Servers and used
	// by the client.  There should be a minimum time value such that the client
	// does not have to renew with Vault at a very high frequency
	TaskTokenTTL string `mapstructure:"task_token_ttl"`

	// Addr is the address of the local Vault agent. This should be a complete
	// URL such as "http://vault.example.com"
	Addr string `mapstructure:"address"`

	// ConnectionRetryIntv is the interval to wait before re-attempting to
	// connect to Vault.
	ConnectionRetryIntv time.Duration

	// TLSCaFile is the path to a PEM-encoded CA cert file to use to verify the
	// Vault server SSL certificate.
	TLSCaFile string `mapstructure:"ca_file"`

	// TLSCaFile is the path to a directory of PEM-encoded CA cert files to
	// verify the Vault server SSL certificate.
	TLSCaPath string `mapstructure:"ca_path"`

	// TLSCertFile is the path to the certificate for Vault communication
	TLSCertFile string `mapstructure:"cert_file"`

	// TLSKeyFile is the path to the private key for Vault communication
	TLSKeyFile string `mapstructure:"key_file"`

	// TLSSkipVerify enables or disables SSL verification
	TLSSkipVerify *bool `mapstructure:"tls_skip_verify"`

	// TLSServerName, if set, is used to set the SNI host when connecting via TLS.
	TLSServerName string `mapstructure:"tls_server_name"`
}

// DefaultVaultConfig() returns the canonical defaults for the Nomad
// `vault` configuration.
func DefaultVaultConfig() *VaultConfig {
	return &VaultConfig{
		Addr:                "https://vault.service.consul:8200",
		ConnectionRetryIntv: DefaultVaultConnectRetryIntv,
		AllowUnauthenticated: func(b bool) *bool {
			return &b
		}(true),
	}
}

// IsEnabled returns whether the config enables Vault integration
func (a *VaultConfig) IsEnabled() bool {
	return a.Enabled != nil && *a.Enabled
}

// AllowsUnauthenticated returns whether the config allows unauthenticated
// access to Vault
func (a *VaultConfig) AllowsUnauthenticated() bool {
	return a.AllowUnauthenticated != nil && *a.AllowUnauthenticated
}

// Merge merges two Vault configurations together.
func (a *VaultConfig) Merge(b *VaultConfig) *VaultConfig {
	result := *a

	if b.Token != "" {
		result.Token = b.Token
	}
	if b.TaskTokenTTL != "" {
		result.TaskTokenTTL = b.TaskTokenTTL
	}
	if b.Addr != "" {
		result.Addr = b.Addr
	}
	if b.ConnectionRetryIntv.Nanoseconds() != 0 {
		result.ConnectionRetryIntv = b.ConnectionRetryIntv
	}
	if b.TLSCaFile != "" {
		result.TLSCaFile = b.TLSCaFile
	}
	if b.TLSCaPath != "" {
		result.TLSCaPath = b.TLSCaPath
	}
	if b.TLSCertFile != "" {
		result.TLSCertFile = b.TLSCertFile
	}
	if b.TLSKeyFile != "" {
		result.TLSKeyFile = b.TLSKeyFile
	}
	if b.TLSServerName != "" {
		result.TLSServerName = b.TLSServerName
	}
	if b.AllowUnauthenticated != nil {
		result.AllowUnauthenticated = b.AllowUnauthenticated
	}
	if b.TLSSkipVerify != nil {
		result.TLSSkipVerify = b.TLSSkipVerify
	}
	if b.Enabled != nil {
		result.Enabled = b.Enabled
	}

	return &result
}

// ApiConfig() returns a usable Vault config that can be passed directly to
// hashicorp/vault/api.
func (c *VaultConfig) ApiConfig() (*vault.Config, error) {
	conf := vault.DefaultConfig()
	tlsConf := &vault.TLSConfig{
		CACert:        c.TLSCaFile,
		CAPath:        c.TLSCaPath,
		ClientCert:    c.TLSCertFile,
		ClientKey:     c.TLSKeyFile,
		TLSServerName: c.TLSServerName,
	}
	if c.TLSSkipVerify != nil {
		tlsConf.Insecure = *c.TLSSkipVerify
	} else {
		tlsConf.Insecure = false
	}

	if err := conf.ConfigureTLS(tlsConf); err != nil {
		return nil, err
	}

	conf.Address = c.Addr
	return conf, nil
}

// Copy returns a copy of this Vault config.
func (c *VaultConfig) Copy() *VaultConfig {
	if c == nil {
		return nil
	}

	nc := new(VaultConfig)
	*nc = *c
	return nc
}
