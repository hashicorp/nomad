package catalog

import (
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/plugins/shared/loader"
	"github.com/mitchellh/go-testing-interface"
)

// TestPluginLoader returns a plugin loader populated only with internal plugins
func TestPluginLoader(t testing.T) loader.PluginCatalog {
	return TestPluginLoaderWithOptions(t, "", nil, nil)
}

// TestPluginLoaderWithOptions allows configuring the plugin loader fully.
func TestPluginLoaderWithOptions(t testing.T,
	pluginDir string,
	options map[string]string,
	configs []*config.PluginConfig) loader.PluginCatalog {

	// Get a logger
	logger := testlog.HCLogger(t)

	// Get the registered plugins
	catalog := Catalog()

	// Create our map of plugins
	internal := make(map[loader.PluginID]*loader.InternalPluginConfig, len(catalog))

	for id, reg := range catalog {
		if reg.Config == nil {
			logger.Warn("skipping loading internal plugin because it is missing its configuration", "plugin", id)
			continue
		}

		pluginConfig := reg.Config.Config
		if reg.ConfigLoader != nil {
			pc, err := reg.ConfigLoader(options)
			if err != nil {
				t.Fatalf("failed to retrieve config for internal plugin %v: %v", id, err)
			}

			pluginConfig = pc
		}

		internal[id] = &loader.InternalPluginConfig{
			Factory: reg.Config.Factory,
			Config:  pluginConfig,
		}
	}

	// Build the plugin loader
	config := &loader.PluginLoaderConfig{
		Logger:          logger,
		PluginDir:       "",
		Configs:         configs,
		InternalPlugins: internal,
	}
	l, err := loader.NewPluginLoader(config)
	if err != nil {
		t.Fatalf("failed to create plugin loader: %v", err)
	}

	return l
}
