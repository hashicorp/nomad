// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package secrets

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestVaultProvider_BuildTemplate(t *testing.T) {
	t.Run("kv template succeeds", func(t *testing.T) {
		testDir := t.TempDir()
		testSecret := &structs.Secret{
			Name:     "foo",
			Provider: "vault",
			Path:     "/test/path",
		}
		p, err := NewVaultProvider(testSecret, testDir, "test")
		must.NoError(t, err)

		tmpl := p.BuildTemplate()
		must.NotNil(t, tmpl)

		// expected template should have correct path, index, and name
		expectedTmpl := `
		{{ with secret "/test/path" }}
		{{ range $k, $v := .Data }}
		secret.foo.{{ $k }}={{ $v }}
		{{ end }}
		{{ end }}`
		// validate template string contains expected data
		must.Eq(t, tmpl.EmbeddedTmpl, expectedTmpl)
	})

	t.Run("kv_v2 template succeeds", func(t *testing.T) {
		testDir := t.TempDir()
		testSecret := &structs.Secret{
			Name:     "foo",
			Provider: "vault",
			Path:     "/test/path",
			Config: map[string]any{
				"engine": VAULT_KV_V2,
			},
		}
		p, err := NewVaultProvider(testSecret, testDir, "test")
		must.NoError(t, err)

		tmpl := p.BuildTemplate()
		must.NotNil(t, tmpl)

		// expected template should have correct path, index, and name
		expectedTmpl := `
		{{ with secret "/test/path" }}
		{{ range $k, $v := .Data.data }}
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
			Provider: "vault",
			Path:     "/test/path",
			Config: map[string]any{
				"engine": 123,
			},
		}
		_, err := NewVaultProvider(testSecret, testDir, "test")
		must.Error(t, err)
	})

	t.Run("path containing delimeter errors", func(t *testing.T) {
		testDir := t.TempDir()
		testSecret := &structs.Secret{
			Name:     "foo",
			Provider: "vault",
			Path:     "/test/path}}",
		}
		_, err := NewVaultProvider(testSecret, testDir, "test")
		must.Error(t, err)
	})
}
