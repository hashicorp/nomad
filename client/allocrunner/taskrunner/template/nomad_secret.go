// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package template

import (
	"context"
	"fmt"
	"text/template"

	"github.com/hashicorp/nomad/client/commonplugins"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/nomad/structs"
)

// NomadSecretItems is a map type returned by the secret template function
// that can be iterated over in templates. It wraps a map[string]string.
type NomadSecretItems map[string]string

// nomadSecretConfig contains the configuration needed to create
// secret plugin template functions.
type nomadSecretConfig struct {
	// CommonPluginDir is the directory containing common plugins
	CommonPluginDir string

	// Namespace is the Nomad namespace for the task
	Namespace string

	// JobID is the job ID for the task
	JobID string

	// Secrets is the list of secrets configured for the task
	Secrets []*structs.Secret
}

// nomadSecretFuncs returns template functions for accessing secret plugins.
// The returned FuncMap can be merged into consul-template's ExtFuncMap.
func nomadSecretFuncs(cfg *nomadSecretConfig) template.FuncMap {
	return template.FuncMap{
		"nomadSecret": nomadSecretFunc(cfg),
	}
}

// nomadSecretFunc returns a template function that fetches secrets from a
// pre-configured secret block by name. The returned map can be iterated
// over in templates using range.
//
// Usage in templates:
//
//	{{ range $k, $v := nomadSecret "app_secrets" }}
//	{{ $k }}={{ $v }}
//	{{ end }}
//
// Or with the `with` clause:
//
//	{{ with nomadSecret "db_creds" }}
//	DB_USER={{ index . "username" }}
//	DB_PASS={{ index . "password" }}
//	{{ end }}
func nomadSecretFunc(cfg *nomadSecretConfig) func(secretName string) (NomadSecretItems, error) {
	// Build a lookup map of secrets by name for efficient access
	secretsByName := make(map[string]*structs.Secret, len(cfg.Secrets))
	for _, s := range cfg.Secrets {
		if s != nil {
			secretsByName[s.Name] = s
		}
	}

	return func(secretName string) (NomadSecretItems, error) {
		if secretName == "" {
			return nil, fmt.Errorf("secret name is required")
		}

		// Look up the secret configuration by name
		secret, ok := secretsByName[secretName]
		if !ok {
			return nil, fmt.Errorf("secret %q not found in task configuration", secretName)
		}

		// Build environment variables for the plugin
		env := make(map[string]string)
		// Copy any env vars configured on the secret block
		for k, v := range secret.Env {
			env[k] = v
		}
		// Add/override with Nomad-specific env vars
		env[taskenv.Namespace] = cfg.Namespace
		env[taskenv.JobID] = cfg.JobID

		// Create the secrets plugin
		plugin, err := commonplugins.NewExternalSecretsPlugin(cfg.CommonPluginDir, secret.Provider, env)
		if err != nil {
			return nil, fmt.Errorf("failed to create secrets plugin %q for secret %q: %w", secret.Provider, secretName, err)
		}

		// Fetch the secrets using the configured path
		ctx := context.Background()
		resp, err := plugin.Fetch(ctx, secret.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch secret %q from plugin %q at path %q: %w", secretName, secret.Provider, secret.Path, err)
		}

		if resp.Error != nil {
			return nil, fmt.Errorf("secret plugin %q returned error for secret %q: %s", secret.Provider, secretName, *resp.Error)
		}

		return NomadSecretItems(resp.Result), nil
	}
}
