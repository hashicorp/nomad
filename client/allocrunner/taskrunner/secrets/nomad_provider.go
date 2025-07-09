// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package secrets

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

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
	secret    *structs.Secret
	secretDir string
	tmplFile  string
	config    *nomadProviderConfig
}

// NewNomadProvider takes a task secret and decodes the config, overwriting the default config fields
// with any provided fields, returning an error if the secret or secret's config is invalid.
func NewNomadProvider(secret *structs.Secret, secretDir string, tmplFile string, namespace string) (*NomadProvider, error) {
	conf := defaultNomadConfig(namespace)
	if err := mapstructure.Decode(secret.Config, conf); err != nil {
		return nil, err
	}

	if err := validateNomadInputs(conf, secret.Path); err != nil {
		return nil, err
	}

	return &NomadProvider{
		config:    conf,
		secret:    secret,
		secretDir: secretDir,
		tmplFile:  tmplFile,
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
		DestPath:     filepath.Join(n.secretDir, n.tmplFile),
		ChangeMode:   structs.TemplateChangeModeNoop,
		Once:         true,
	}
}

func (n *NomadProvider) Parse() (map[string]string, error) {
	f, err := os.OpenInRoot(n.secretDir, n.tmplFile)
	if err != nil {
		return nil, fmt.Errorf("error opening env template: %v", err)
	}
	defer func() {
		f.Close()
		os.Remove(filepath.Join(n.secretDir, n.tmplFile))
	}()

	return envparse.Parse(f)
}

// validateNomadInputs ensures none of the user provided inputs contain delimiters
// that could be used to inject other CT functions.
func validateNomadInputs(conf *nomadProviderConfig, path string) error {
	// match if a string contains (...), {{, or }}
	var templateValidate = regexp.MustCompile(`\(.*\)|\{\{|\}\}`)

	if templateValidate.MatchString(conf.Namespace) {
		return fmt.Errorf("namespace cannot contain template delimiters or parenthesis")
	}

	if templateValidate.MatchString(path) {
		return fmt.Errorf("path cannot contain template delimiters or parenthesis")
	}

	return nil
}
