package secrets

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-envparse"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

const (
	VAULT_KV    = "kv"
	VAULT_KV_V2 = "kv_v2"
)

type vaultProviderConfig struct {
	Engine string `mapstructure:"engine"`
}

func defaultVaultConfig() *vaultProviderConfig {
	return &vaultProviderConfig{
		Engine: VAULT_KV,
	}
}

type VaultProvider struct {
	secret   *structs.Secret
	tmplPath string
	conf     *vaultProviderConfig
}

// NewVaultProvider takes a task secret and decodes the config, overwriting the default config fields
// with any provided fields, returning an error if the secret or secret's config is invalid.
func NewVaultProvider(s *structs.Secret, path string) (*VaultProvider, error) {
	if s == nil {
		return nil, fmt.Errorf("empty secret for vault provider")
	}

	conf := defaultVaultConfig()
	if err := mapstructure.Decode(s.Config, conf); err != nil {
		return nil, err
	}

	return &VaultProvider{
		secret:   s,
		tmplPath: path,
		conf:     conf,
	}, nil
}

func (v *VaultProvider) BuildTemplate() *structs.Template {
	indexKey := ".Data"
	if v.conf.Engine == VAULT_KV_V2 {
		indexKey = ".Data.data"
	}

	data := fmt.Sprintf(`
		{{ with secret "%s" }}
		{{ range $k, $v := %s }}
		secret.%s.{{ $k }}={{ $v }}
		{{ end }}
		{{ end }}`,
		v.secret.Path, indexKey, v.secret.Name)

	return &structs.Template{
		EmbeddedTmpl: data,
		DestPath:     v.tmplPath,
		ChangeMode:   structs.TemplateChangeModeNoop,
		Once:         true,
	}
}

func (v *VaultProvider) Parse() (map[string]string, error) {
	// we checked escape before we rendered the file
	dest := filepath.Clean(v.tmplPath)
	f, err := os.Open(dest)
	if err != nil {
		return nil, fmt.Errorf("error opening env template: %v", err)
	}
	defer func() {
		f.Close()
		os.Remove(dest)
	}()

	return envparse.Parse(f)
}
