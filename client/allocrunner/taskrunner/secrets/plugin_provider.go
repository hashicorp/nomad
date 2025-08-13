// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package secrets

import (
	"context"
	"fmt"

	"github.com/hashicorp/nomad/client/commonplugins"
)

type ExternalPluginProvider struct {
	// plugin is the commonplugin to be executed by this secret
	plugin commonplugins.SecretsPlugin

	// name of the plugin and also the executable
	name string

	// path is the secret location used in Fetch
	path string
}

type Response struct {
	Result map[string]string `json:"result"`
	Error  *string           `json:"error,omitempty"`
}

func NewExternalPluginProvider(plugin commonplugins.SecretsPlugin, name string, path string) *ExternalPluginProvider {
	return &ExternalPluginProvider{
		plugin: plugin,
		name:   name,
		path:   path,
	}
}

func (p *ExternalPluginProvider) Fetch(ctx context.Context) (map[string]string, error) {
	resp, err := p.plugin.Fetch(ctx, p.path)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch secret from plugin %s: %w", p.name, err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("error returned from secret plugin %s: %s", p.name, *resp.Error)
	}

	formatted := make(map[string]string, len(resp.Result))
	for k, v := range resp.Result {
		formatted[fmt.Sprintf("secret.%s.%s", p.name, k)] = v
	}

	return formatted, nil
}
