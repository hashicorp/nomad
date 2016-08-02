package config

import vault "github.com/hashicorp/nomad/api"

// VaultConfig contains the configuration information necessary to
// communicate with Vault in order to:
//
// - Renew Vault tokens/leases.
//
// - Pass a token for the Nomad Server to derive sub-tokens.
//
// - Create child tokens with policy subsets of the Server's token.
type VaultConfig struct {

	// RoleName is the Vault role in which Nomad will derive child tokens using
	// /auth/token/create/[role_name]
	RoleName string `mapstructure:"role_name"`

	// RoleToken is the periodic Vault token given to Nomad such that it can
	// derive child tokens. The RoleToken should be created from the passed
	// RoleName. Nomad will renew this token at half its lease lifetime.
	RoleToken string `mapstructure:"role_token"`

	// AllowUnauthenticated allows users to submit jobs requiring Vault tokens
	// without providing a Vault token proving they have access to these
	// policies.
	AllowUnauthenticated bool `mapstructure:"allow_unauthenticated"`

	// ChildTokenTTL is the TTL of the tokens created by Nomad Servers and used
	// by the client.  There should be a minimum time value such that the client
	// does not have to renew with Vault at a very high frequency
	ChildTokenTTL string `mapstructure:"child_token_ttl"`

	// Addr is the address of the local Vault agent
	Addr string `mapstructure:"address"`

	// CACert is the path to a PEM-encoded CA cert file to use to verify the
	// Vault server SSL certificate.
	CACert string `mapstructure:"ca_cert"`

	// CAPath is the path to a directory of PEM-encoded CA cert files to verify
	// the Vault server SSL certificate.
	CAPath string `mapstructure:"ca_path"`

	// CertFile is the path to the certificate for Vault communication
	CertFile string `mapstructure:"cert_file"`

	// KeyFile is the path to the private key for Vault communication
	KeyFile string `mapstructure:"key_file"`

	// VerifySSL enables or disables SSL verification
	VerifySSL bool `mapstructure:"verify_ssl"`

	// TLSServerName, if set, is used to set the SNI host when connecting via TLS.
	TLSServerName string `mapstructure:"tls_server_name"`
}

// DefaultVaultConfig() returns the canonical defaults for the Nomad
// `vault` configuration.
func DefaultVaultConfig() *VaultConfig {
	return &VaultConfig{
		AllowUnauthenticated: false,
		Addr:                 "vault.service.consul:8200",
	}
}

// Merge merges two Vault configurations together.
func (a *VaultConfig) Merge(b *VaultConfig) *VaultConfig {
	result := *a

	if b.RoleName != "" {
		result.RoleName = b.RoleName
	}
	if b.RoleToken != "" {
		result.RoleToken = b.RoleToken
	}
	if b.AllowUnauthenticated {
		result.AllowUnauthenticated = true
	}
	if b.ChildTokenTTL != "" {
		result.ChildTokenTTL = b.ChildTokenTTL
	}
	if b.Addr != "" {
		result.Addr = b.Addr
	}
	if b.CACert != "" {
		result.CACert = b.CACert
	}
	if b.CAPath != "" {
		result.CAPath = b.CAPath
	}
	if b.CertFile != "" {
		result.CertFile = b.CertFile
	}
	if b.KeyFile != "" {
		result.KeyFile = b.KeyFile
	}
	if b.VerifySSL {
		result.VerifySSL = true
	}
	if b.TLSServerName != "" {
		result.TLSServerName = b.TLSServerName
	}
	return &result
}

// ApiConfig() returns a usable Vault config that can be passed directly to
// hashicorp/vault/api.
func (c *VaultConfig) ApiConfig() (*vault.Config, error) {
	return nil, nil
}
