package config

import (
	"fmt"

	vault "github.com/hashicorp/vault/api"
)

const (
	// VaultTokenCreateTTL is the duration the wrapped token for the client is
	// valid for. The units are in seconds.
	VaultTokenCreateTTL = "60s"
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
	Enabled bool `mapstructure:"enabled"`

	// Token is the Vault token given to Nomad such that it can
	// derive child tokens. Nomad will renew this token at half its lease
	// lifetime.
	Token string `mapstructure:"token"`

	// AllowUnauthenticated allows users to submit jobs requiring Vault tokens
	// without providing a Vault token proving they have access to these
	// policies.
	AllowUnauthenticated bool `mapstructure:"allow_unauthenticated"`

	// TaskTokenTTL is the TTL of the tokens created by Nomad Servers and used
	// by the client.  There should be a minimum time value such that the client
	// does not have to renew with Vault at a very high frequency
	TaskTokenTTL string `mapstructure:"task_token_ttl"`

	// Addr is the address of the local Vault agent
	Addr string `mapstructure:"address"`

	// TLSCaFile is the path to a PEM-encoded CA cert file to use to verify the
	// Vault server SSL certificate.
	TLSCaFile string `mapstructure:"tls_ca_file"`

	// TLSCaFile is the path to a directory of PEM-encoded CA cert files to
	// verify the Vault server SSL certificate.
	TLSCaPath string `mapstructure:"tls_ca_path"`

	// TLSCertFile is the path to the certificate for Vault communication
	TLSCertFile string `mapstructure:"tls_cert_file"`

	// TLSKeyFile is the path to the private key for Vault communication
	TLSKeyFile string `mapstructure:"tls_key_file"`

	// TLSSkipVerify enables or disables SSL verification
	TLSSkipVerify bool `mapstructure:"tls_skip_verify"`

	// TLSServerName, if set, is used to set the SNI host when connecting via TLS.
	TLSServerName string `mapstructure:"tls_server_name"`
}

// DefaultVaultConfig() returns the canonical defaults for the Nomad
// `vault` configuration.
func DefaultVaultConfig() *VaultConfig {
	return &VaultConfig{
		Enabled:              true,
		AllowUnauthenticated: false,
		Addr:                 "vault.service.consul:8200",
	}
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

	result.AllowUnauthenticated = b.AllowUnauthenticated
	result.TLSSkipVerify = b.TLSSkipVerify
	result.Enabled = b.Enabled
	return &result
}

// ApiConfig() returns a usable Vault config that can be passed directly to
// hashicorp/vault/api. If readEnv is true, the environment is read for
// appropriate Vault variables.
func (c *VaultConfig) ApiConfig(readEnv bool) (*vault.Config, error) {
	conf := vault.DefaultConfig()
	if readEnv {
		if err := conf.ReadEnvironment(); err != nil {
			return nil, err
		}
	}

	tlsConf := &vault.TLSConfig{
		CACert:        c.TLSCaFile,
		CAPath:        c.TLSCaPath,
		ClientCert:    c.TLSCertFile,
		ClientKey:     c.TLSKeyFile,
		TLSServerName: c.TLSServerName,
		Insecure:      c.TLSSkipVerify,
	}
	if err := conf.ConfigureTLS(tlsConf); err != nil {
		return nil, err
	}

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

// GetWrappingFn returns an appropriate wrapping function for Nomad
func (c *VaultConfig) GetWrappingFn() func(operation, path string) string {
	createPath := fmt.Sprintf("auth/token/create/%s", c.TokenRoleName)
	return func(operation, path string) string {
		// Only wrap the token create operation
		if operation != "POST" || path != createPath {
			return ""
		}

		return VaultTokenCreateTTL
	}
}
