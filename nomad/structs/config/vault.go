// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"time"

	"github.com/hashicorp/nomad/helper/pointer"
	vault "github.com/hashicorp/vault/api"
)

const (
	// DefaultVaultConnectRetryIntv is the retry interval between trying to
	// connect to Vault
	DefaultVaultConnectRetryIntv = 30 * time.Second
)

// VaultConfig contains the configuration information necessary to
// communicate with Vault in order to:
//   - Renew Vault tokens/leases.
//   - Pass a token for the Nomad Server to derive sub-tokens.
//   - Create child tokens with policy subsets of the Server's token.
//   - Create Vault ACL tokens from workload identity JWTs.
type VaultConfig struct {
	// Servers and clients fields.

	// Name is used to identify the Vault cluster related to this
	// configuration.
	Name string `mapstructure:"name"`

	// Enabled enables or disables Vault support.
	Enabled *bool `mapstructure:"enabled"`

	// Role sets the role in which to create tokens from.
	//
	// When using workload identities this field defines the default role to
	// use when a job does not define a role in its `vault` block. If this
	// config value is also unset, the default auth method or cluster global
	// role is used.
	//
	// When not using workload identities, the Nomad servers will derive tokens
	// using this role. The Vault token provided to the Nomad server config
	// does not have to be created from this role but must have "update"
	// capability on "auth/token/create/<create_from_role>". If this value is
	// unset and the token is created from a role, the value is defaulted to
	// the role the token is from.
	//
	// This used to be a server-only field, but it's a client-only field when
	// workload identities are used, so it should be set in both places during
	// the transition period.
	Role string `mapstructure:"create_from_role"`

	// Clients-only fields.

	// Namespace sets the Vault namespace used for all calls against the
	// Vault API. If this is unset, then Nomad does not use Vault namespaces.
	Namespace string `mapstructure:"namespace"`

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

	// Servers-only fields.

	// UseIdentity defines if workload identities should be used to derive
	// Vault tokens.
	//
	// It is a transitional field used only during the adoption period of
	// workload identities and will be ignored and removed in future versions.
	UseIdentity *bool `mapstructure:"use_identity"`

	// DefaultIdentity is the default workload identity configuration used when
	// a job has a `vault` block but no `identity` named "vault_<name>", where
	// <name> matches this block `name` parameter.
	DefaultIdentity *WorkloadIdentityConfig `mapstructure:"default_identity"`

	// Deprecated fields.

	// Token is the Vault token given to Nomad such that it can
	// derive child tokens. Nomad will renew this token at half its lease
	// lifetime.
	//
	// Deprecated: Nomad 1.7.0 is able to derive Vault tokens from workload
	// identities. This field will be removed in a future release.
	Token string `mapstructure:"token"`

	// AllowUnauthenticated allows users to submit jobs requiring Vault tokens
	// without providing a Vault token proving they have access to these
	// policies.
	//
	// Deprecated: Nomad 1.7.0 no longer requires a Vault token for job
	// operations. This field will be removed in a future release.
	AllowUnauthenticated *bool `mapstructure:"allow_unauthenticated"`

	// TaskTokenTTL is the TTL of the tokens created by Nomad Servers and used
	// by the client.  There should be a minimum time value such that the client
	// does not have to renew with Vault at a very high frequency
	//
	// Deprecated: Nomad 1.7.0 derives tokens from workload identities that
	// receive their TTL configuration from the Vault role used. This field
	// will be removed in a future release.
	TaskTokenTTL string `mapstructure:"task_token_ttl"`
}

// DefaultVaultConfig returns the canonical defaults for the Nomad
// `vault` configuration.
func DefaultVaultConfig() *VaultConfig {
	return &VaultConfig{
		Name:                 "default",
		Addr:                 "https://vault.service.consul:8200",
		ConnectionRetryIntv:  DefaultVaultConnectRetryIntv,
		AllowUnauthenticated: pointer.Of(true),
		UseIdentity:          pointer.Of(false),
	}
}

// IsEnabled returns whether the config enables Vault integration
func (c *VaultConfig) IsEnabled() bool {
	return c.Enabled != nil && *c.Enabled
}

// AllowsUnauthenticated returns whether the config allows unauthenticated
// access to Vault
func (c *VaultConfig) AllowsUnauthenticated() bool {
	return c.AllowUnauthenticated != nil && *c.AllowUnauthenticated
}

// Merge merges two Vault configurations together.
func (c *VaultConfig) Merge(b *VaultConfig) *VaultConfig {
	result := *c

	if b.Name != "" {
		c.Name = b.Name
	}
	if b.Enabled != nil {
		result.Enabled = b.Enabled
	}
	if b.Role != "" {
		result.Role = b.Role
	}

	if b.Namespace != "" {
		result.Namespace = b.Namespace
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
	if b.TLSSkipVerify != nil {
		result.TLSSkipVerify = b.TLSSkipVerify
	}
	if b.TLSServerName != "" {
		result.TLSServerName = b.TLSServerName
	}

	result.UseIdentity = pointer.Merge(result.UseIdentity, b.UseIdentity)

	if result.DefaultIdentity == nil && b.DefaultIdentity != nil {
		sID := *b.DefaultIdentity
		result.DefaultIdentity = &sID
	} else if b.DefaultIdentity != nil {
		result.DefaultIdentity = result.DefaultIdentity.Merge(b.DefaultIdentity)
	}

	if b.Token != "" {
		result.Token = b.Token
	}
	if b.AllowUnauthenticated != nil {
		result.AllowUnauthenticated = b.AllowUnauthenticated
	}
	if b.TaskTokenTTL != "" {
		result.TaskTokenTTL = b.TaskTokenTTL
	}

	return &result
}

// ApiConfig returns a usable Vault config that can be passed directly to
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

// Equal compares two Vault configurations and returns a boolean indicating
// if they are equal.
func (c *VaultConfig) Equal(b *VaultConfig) bool {
	if c == nil && b != nil {
		return false
	}
	if c != nil && b == nil {
		return false
	}

	if c.Name != b.Name {
		return false
	}
	if !pointer.Eq(c.Enabled, b.Enabled) {
		return false
	}
	if c.Role != b.Role {
		return false
	}

	if c.Namespace != b.Namespace {
		return false
	}
	if c.Addr != b.Addr {
		return false
	}
	if c.ConnectionRetryIntv.Nanoseconds() != b.ConnectionRetryIntv.Nanoseconds() {
		return false
	}
	if c.TLSCaFile != b.TLSCaFile {
		return false
	}
	if c.TLSCaPath != b.TLSCaPath {
		return false
	}
	if c.TLSCertFile != b.TLSCertFile {
		return false
	}
	if c.TLSKeyFile != b.TLSKeyFile {
		return false
	}
	if !pointer.Eq(c.TLSSkipVerify, b.TLSSkipVerify) {
		return false
	}
	if c.TLSServerName != b.TLSServerName {
		return false
	}

	if !pointer.Eq(b.UseIdentity, c.UseIdentity) {
		return false
	}
	if !c.DefaultIdentity.Equal(b.DefaultIdentity) {
		return false
	}

	if c.Token != b.Token {
		return false
	}
	if !pointer.Eq(c.AllowUnauthenticated, b.AllowUnauthenticated) {
		return false
	}
	if c.TaskTokenTTL != b.TaskTokenTTL {
		return false
	}

	return true
}
