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

func defaultNomadConfig() *nomadProviderConfig {
	return &nomadProviderConfig{
		Namespace: "default",
	}
}

type NomadProvider struct {
	secret   *structs.Secret
	tmplPath string
}

func NewNomadProvider(s *structs.Secret, p string) *NomadProvider {
	return &NomadProvider{
		secret:   s,
		tmplPath: p,
	}
}

func (n *NomadProvider) BuildTemplate() (*structs.Template, error) {
	if n.secret == nil {
		return nil, fmt.Errorf("empty secret for nomad provider")
	}

	conf := defaultNomadConfig()
	if err := mapstructure.Decode(n.secret.Config, conf); err != nil {
		return nil, err
	}

	data := fmt.Sprintf(`
		{{ with nomadVar "%s@%s" }}
		{{ range $k, $v := . }}
		secret.%s.{{ $k }}={{ $v }}
		{{ end }}
		{{ end }}`,
		n.secret.Path, conf.Namespace, n.secret.Name)

	return &structs.Template{
		EmbeddedTmpl: data,
		DestPath:     n.tmplPath,
		ChangeMode:   structs.TemplateChangeModeNoop,
		Once:         true,
	}, nil
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
