// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package secrets

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

const (
	SecretProviderVault = "vault"

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
	secret    *structs.Secret
	secretDir string
	tmplFile  string
	conf      *vaultProviderConfig
}

// NewVaultProvider takes a task secret and decodes the config, overwriting the default config fields
// with any provided fields, returning an error if the secret or secret's config is invalid.
func NewVaultProvider(secret *structs.Secret, secretDir string, tmplFile string) (*VaultProvider, error) {
	conf := defaultVaultConfig()
	if err := mapstructure.Decode(secret.Config, conf); err != nil {
		return nil, err
	}

	if strings.ContainsAny(secret.Path, "(){}") {
		return nil, errors.New("secret path cannot contain template delimiters or parenthesis")
	}

	return &VaultProvider{
		secret:    secret,
		secretDir: secretDir,
		tmplFile:  tmplFile,
		conf:      conf,
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
		DestPath:     filepath.Clean(filepath.Join(v.secretDir, v.tmplFile)),
		ChangeMode:   structs.TemplateChangeModeNoop,
		Once:         true,
	}
}
