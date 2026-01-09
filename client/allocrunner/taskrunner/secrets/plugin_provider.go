// Copyright IBM Corp. 2015, 2025
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

	// name of the secret executing the plugin
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
		return nil, fmt.Errorf("failed executing plugin for secret %q: %w", p.name, err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("secret %q plugin response contained error: %q", p.name, *resp.Error)
	}

	formatted := make(map[string]string, len(resp.Result))
	for k, v := range resp.Result {
		formatted[fmt.Sprintf("secret.%s.%s", p.name, k)] = v
	}

	return formatted, nil
}
