package secrets

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/nomad/client/commonplugins"
)

type ExternalPluginProvider struct {
	// plugin is the commonplugin to be executed by this secret
	plugin commonplugins.SecretsPlugin

	// response is the plugin response saved after Fetch is called
	response *commonplugins.SecretResponse

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

func (p *ExternalPluginProvider) Fetch(ctx context.Context) error {
	resp, err := p.plugin.Fetch(ctx, p.path)
	if err != nil {
		return fmt.Errorf("failed to fetch secret from plugin %s: %w", p.name, err)
	}
	if resp.Error != nil {
		return fmt.Errorf("error returned from secret plugin %s: %s", p.name, *resp.Error)
	}

	p.response = resp
	return nil
}

func (p *ExternalPluginProvider) Parse() (map[string]string, error) {
	if p.response == nil {
		return nil, errors.New("no plugin response for provider to parse")
	}

	formatted := map[string]string{}
	for k, v := range p.response.Result {
		formatted[fmt.Sprintf("secret.%s.%s", p.name, k)] = v
	}

	return formatted, nil
}
