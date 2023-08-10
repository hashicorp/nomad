// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"

	"github.com/hashicorp/nomad/helper/pluginutils/catalog"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/helper/pluginutils/singleton"
)

// setupPlugins is used to setup the plugin loaders.
func (a *Agent) setupPlugins() error {
	// Get our internal plugins
	internal, err := a.internalPluginConfigs()
	if err != nil {
		return err
	}

	// Build the plugin loader
	config := &loader.PluginLoaderConfig{
		Logger:            a.logger,
		PluginDir:         a.config.PluginDir,
		Configs:           a.config.Plugins,
		InternalPlugins:   internal,
		SupportedVersions: loader.AgentSupportedApiVersions,
	}
	l, err := loader.NewPluginLoader(config)
	if err != nil {
		return fmt.Errorf("failed to create plugin loader: %v", err)
	}
	a.pluginLoader = l

	// Wrap the loader to get our singleton loader
	a.pluginSingletonLoader = singleton.NewSingletonLoader(a.logger, l)

	for k, plugins := range a.pluginLoader.Catalog() {
		for _, p := range plugins {
			a.logger.Info("detected plugin", "name", p.Name, "type", k, "plugin_version", p.PluginVersion)
		}
	}

	return nil
}

func (a *Agent) internalPluginConfigs() (map[loader.PluginID]*loader.InternalPluginConfig, error) {
	// Get the registered plugins
	catalog := catalog.Catalog()

	// Create our map of plugins
	internal := make(map[loader.PluginID]*loader.InternalPluginConfig, len(catalog))

	// Grab the client options map if we can
	var options map[string]string
	if a.config != nil && a.config.Client != nil {
		options = a.config.Client.Options
	}

	for id, reg := range catalog {
		if reg.Config == nil {
			a.logger.Error("skipping loading internal plugin because it is missing its configuration", "plugin", id)
			continue
		}

		pluginConfig := reg.Config.Config
		if reg.ConfigLoader != nil {
			pc, err := reg.ConfigLoader(options)
			if err != nil {
				return nil, fmt.Errorf("failed to retrieve config for internal plugin %v: %v", id, err)
			}

			pluginConfig = pc

			// TODO We should log the config to warn users about upgrade pathing
		}

		internal[id] = &loader.InternalPluginConfig{
			Factory: reg.Config.Factory,
			Config:  pluginConfig,
		}
	}

	return internal, nil
}
