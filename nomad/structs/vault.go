// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"

	"github.com/hashicorp/go-secure-stdlib/strutil"
	vapi "github.com/hashicorp/vault/api"
	"github.com/mitchellh/mapstructure"
)

const (
	// VaultDefaultCluster is the name used for the Vault cluster that doesn't
	// have a name.
	VaultDefaultCluster = "default"

	// VaultNamespaceHeaderName is the header set to specify which namespace
	// the request is indented for. This is defined within Nomad, so we do not
	// need to import the entire Vault SDK package.
	VaultNamespaceHeaderName = "X-Vault-Namespace"
)

// VaultTokenData represents some of the fields returned in the Data map of the
// sercret returned by the Vault API when doing a token lookup request.
type VaultTokenData struct {
	CreationTTL   int      `mapstructure:"creation_ttl"`
	TTL           int      `mapstructure:"ttl"`
	Renewable     bool     `mapstructure:"renewable"`
	Policies      []string `mapstructure:"policies"`
	Role          string   `mapstructure:"role"`
	NamespacePath string   `mapstructure:"namespace_path"`

	// root caches if the token has the "root" policy to avoid travesring the
	// policies list every time.
	root *bool
}

// Root returns true if the token has the `root` policy.
func (d VaultTokenData) Root() bool {
	if d.root != nil {
		return *d.root
	}

	root := strutil.StrListContains(d.Policies, "root")
	d.root = &root

	return root
}

// VaultTokenRoleData represents some of the fields returned in the Data map of
// the sercret returned by the Vault API when reading a token role.
type VaultTokenRoleData struct {
	Name                 string `mapstructure:"name"`
	ExplicitMaxTtl       int    `mapstructure:"explicit_max_ttl"`
	TokenExplicitMaxTtl  int    `mapstructure:"token_explicit_max_ttl"`
	Orphan               bool
	Period               int
	TokenPeriod          int `mapstructure:"token_period"`
	Renewable            bool
	DisallowedPolicies   []string `mapstructure:"disallowed_policies"`
	AllowedEntityAliases []string `mapstructure:"allowed_entity_aliases"`
	AllowedPolicies      []string `mapstructure:"allowed_policies"`
}

// DecodeVaultSecretData decodes a Vault sercret Data map into a struct.
func DecodeVaultSecretData(s *vapi.Secret, out interface{}) error {
	if s == nil {
		return fmt.Errorf("cannot decode nil Vault secret")
	}

	if err := mapstructure.WeakDecode(s.Data, &out); err != nil {
		return err
	}

	return nil
}

func ValidateVaultClusterName(cluster string) error {
	if !validConsulVaultClusterName.MatchString(cluster) {
		return fmt.Errorf("invalid name %q, must match regex %s", cluster, validConsulVaultClusterName)
	}

	return nil
}

// GetVaultClusterName gets the Vault cluster for this task. Only a single
// default cluster is supported in Nomad CE, but this function can be safely
// used for ENT as well because the appropriate Cluster value will be set at the
// time of job submission.
func (t *Task) GetVaultClusterName() string {
	if t.Vault != nil && t.Vault.Cluster != "" {
		return t.Vault.Cluster
	}
	return VaultDefaultCluster
}
