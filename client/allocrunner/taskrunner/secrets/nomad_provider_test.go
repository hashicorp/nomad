// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package secrets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestNomadProvider_BuildTemplate(t *testing.T) {
	t.Run("variable template succeeds", func(t *testing.T) {
		testDir := t.TempDir()
		testSecret := &structs.Secret{
			Name:     "foo",
			Provider: "nomad",
			Path:     "/test/path",
			Config: map[string]any{
				"namespace": "dev",
			},
		}
		p, err := NewNomadProvider(testSecret, testDir, "test", "default")
		must.NoError(t, err)

		tmpl := p.BuildTemplate()
		must.NotNil(t, tmpl)

		// expected template should have correct path and name
		expectedTmpl := `
		{{ with nomadVar "/test/path@dev" }}
		{{ range $k, $v := . }}
		secret.foo.{{ $k }}={{ $v }}
		{{ end }}
		{{ end }}`
		// validate template string contains expected data
		must.Eq(t, tmpl.EmbeddedTmpl, expectedTmpl)
	})

	t.Run("invalid config options errors", func(t *testing.T) {
		testDir := t.TempDir()
		testSecret := &structs.Secret{
			Name:     "foo",
			Provider: "nomad",
			Path:     "/test/path",
			Config: map[string]any{
				"namespace": 123,
			},
		}
		_, err := NewNomadProvider(testSecret, testDir, "test", "default")
		must.Error(t, err)
	})

	t.Run("path with delimiter errors", func(t *testing.T) {
		testDir := t.TempDir()
		testSecret := &structs.Secret{
			Name:     "foo",
			Provider: "nomad",
			Path:     "/test/path}}",
			Config: map[string]any{
				"namespace": 123,
			},
		}
		_, err := NewNomadProvider(testSecret, testDir, "test", "default")
		must.Error(t, err)
	})

	t.Run("namespace with parenthesis errors", func(t *testing.T) {
		testDir := t.TempDir()
		testSecret := &structs.Secret{
			Name:     "foo",
			Provider: "nomad",
			Path:     "/test/path",
			Config: map[string]any{
				"namespace": "( some namespace )",
			},
		}
		_, err := NewNomadProvider(testSecret, testDir, "test", "default")
		must.Error(t, err)
	})
}

func TestNomadProvider_Parse(t *testing.T) {
	testDir := t.TempDir()
	tmplFile := "foo"
	tmplPath := filepath.Join(testDir, tmplFile)

	data := "foo=bar"
	err := os.WriteFile(tmplPath, []byte(data), 0777)
	must.NoError(t, err)

	p, err := NewNomadProvider(&structs.Secret{}, testDir, tmplFile, "default")
	must.NoError(t, err)

	vars, err := p.Parse()
	must.NoError(t, err)
	must.Eq(t, vars, map[string]string{"foo": "bar"})

	_, err = os.Stat(tmplPath)
	must.ErrorContains(t, err, "no such file")
}
