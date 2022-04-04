package structs

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-secure-stdlib/strutil"
	vapi "github.com/hashicorp/vault/api"
	"github.com/mitchellh/mapstructure"
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

// AllowsEntityAlias returns true if the token role allows the given entity
// alias to be used when creating a token.
// It applies the same checks as in:
// https://github.com/hashicorp/vault/blob/v1.10.0/vault/token_store.go#L2569-L2578
func (d VaultTokenRoleData) AllowsEntityAlias(alias string) bool {
	lowcaseAlias := strings.ToLower(alias)
	return strutil.StrListContains(d.AllowedEntityAliases, lowcaseAlias) ||
		strutil.StrListContainsGlob(d.AllowedEntityAliases, lowcaseAlias)
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
