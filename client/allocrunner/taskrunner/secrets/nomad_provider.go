// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package secrets

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-envparse"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

type nomadProviderConfig struct {
	Namespace string `mapstructure:"namespace"`
}

func defaultNomadConfig(namespace string) *nomadProviderConfig {
	return &nomadProviderConfig{
		Namespace: namespace,
	}
}

type NomadProvider struct {
	secret   *structs.Secret
	tmplPath string
	config   *nomadProviderConfig
}

// NewNomadProvider takes a task secret and decodes the config, overwriting the default config fields
// with any provided fields, returning an error if the secret or secret's config is invalid.
func NewNomadProvider(secret *structs.Secret, path string, namespace string) (*NomadProvider, error) {
	if secret == nil {
		return nil, fmt.Errorf("empty secret for nomad provider")
	}

	conf := defaultNomadConfig(namespace)
	if err := mapstructure.Decode(secret.Config, conf); err != nil {
		return nil, err
	}

	return &NomadProvider{
		config:   conf,
		secret:   secret,
		tmplPath: path,
	}, nil
}

func (n *NomadProvider) BuildTemplate() *structs.Template {
	data := fmt.Sprintf(`
		{{ with nomadVar "%s@%s" }}
		{{ range $k, $v := . }}
		secret.%s.{{ $k }}={{ $v }}
		{{ end }}
		{{ end }}`,
		n.secret.Path, n.config.Namespace, n.secret.Name)

	return &structs.Template{
		EmbeddedTmpl: data,
		DestPath:     n.tmplPath,
		ChangeMode:   structs.TemplateChangeModeNoop,
		Once:         true,
	}
}

func (n *NomadProvider) Parse() (map[string]string, error) {
	dest := filepath.Clean(n.tmplPath)
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
