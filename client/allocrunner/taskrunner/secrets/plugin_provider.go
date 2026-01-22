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

	// pluginName refers to the provider parameter of the secret block
	// and is here mainly for debugging purposes
	pluginName string

	// secretName is the secret block name executing the plugin
	secretName string

	// path is the secret location used in Fetch
	path string
}

type Response struct {
	Result map[string]string `json:"result"`
	Error  *string           `json:"error,omitempty"`
}

func NewExternalPluginProvider(plugin commonplugins.SecretsPlugin, pluginName, secretName, path string) *ExternalPluginProvider {
	return &ExternalPluginProvider{
		plugin:     plugin,
		pluginName: pluginName,
		secretName: secretName,
		path:       path,
	}
}

func (p *ExternalPluginProvider) Fetch(ctx context.Context) (map[string]string, error) {
	resp, err := p.plugin.Fetch(ctx, p.path)
	if err != nil {
		return nil, fmt.Errorf("failed executing plugin %q for secret %q: %w", p.pluginName, p.secretName, err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("provider %q for secret %q response contained error: %q", p.pluginName, p.secretName, *resp.Error)
	}

	formatted := make(map[string]string, len(resp.Result))
	for k, v := range resp.Result {
		formatted[fmt.Sprintf("secret.%s.%s", p.secretName, k)] = v
	}

	return formatted, nil
}
