package loader

import (
	"fmt"
	"os/exec"

	log "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/plugins"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

// PluginCatalog is used to retrieve plugins, either external or internal
type PluginCatalog interface {
	// Dispense returns the plugin given its name and type. This will also
	// configure the plugin
	Dispense(name, pluginType string, config *base.ClientAgentConfig, logger log.Logger) (PluginInstance, error)

	// Reattach is used to reattach to a previously launched external plugin.
	Reattach(name, pluginType string, config *plugin.ReattachConfig) (PluginInstance, error)

	// Catalog returns the catalog of all plugins keyed by plugin type
	Catalog() map[string][]*base.PluginInfoResponse
}

// PluginLoader is used to retrieve plugins either externally or from internal
// factories.
type PluginLoader struct {
	// logger is the plugin loaders logger
	logger log.Logger

	// pluginDir is the directory containing plugin binaries
	pluginDir string

	// plugins maps a plugin to information required to launch it
	plugins map[PluginID]*pluginInfo
}

// PluginID is a tuple identifying a plugin
type PluginID struct {
	// Name is the name of the plugin
	Name string

	// PluginType is the plugin's type
	PluginType string
}

// String returns a friendly representation of the plugin.
func (id PluginID) String() string {
	return fmt.Sprintf("%q (%v)", id.Name, id.PluginType)
}

func PluginInfoID(resp *base.PluginInfoResponse) PluginID {
	return PluginID{
		Name:       resp.Name,
		PluginType: resp.Type,
	}
}

// PluginLoaderConfig configures a plugin loader.
type PluginLoaderConfig struct {
	// Logger is the logger used by the plugin loader
	Logger log.Logger

	// PluginDir is the directory scanned for loading plugins
	PluginDir string

	// Configs is an optional set of configs for plugins
	Configs []*config.PluginConfig

	// InternalPlugins allows registering internal plugins.
	InternalPlugins map[PluginID]*InternalPluginConfig
}

// InternalPluginConfig is used to configure launching an internal plugin.
type InternalPluginConfig struct {
	Config  map[string]interface{}
	Factory plugins.PluginFactory
}

// pluginInfo captures the necessary information to launch and configure a
// plugin.
type pluginInfo struct {
	factory plugins.PluginFactory

	exePath string
	args    []string

	baseInfo *base.PluginInfoResponse
	version  *version.Version

	configSchema  *hclspec.Spec
	config        map[string]interface{}
	msgpackConfig []byte
}

// NewPluginLoader returns an instance of a plugin loader or an error if the
// plugins could not be loaded
func NewPluginLoader(config *PluginLoaderConfig) (*PluginLoader, error) {
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid plugin loader configuration passed: %v", err)
	}

	logger := config.Logger.Named("plugin_loader").With("plugin_dir", config.PluginDir)
	l := &PluginLoader{
		logger:    logger,
		pluginDir: config.PluginDir,
		plugins:   make(map[PluginID]*pluginInfo),
	}

	if err := l.init(config); err != nil {
		return nil, fmt.Errorf("failed to initialize plugin loader: %v", err)
	}

	return l, nil
}

// Dispense returns a plugin instance, loading it either internally or by
// launching an external plugin.
func (l *PluginLoader) Dispense(name, pluginType string, config *base.ClientAgentConfig, logger log.Logger) (PluginInstance, error) {
	id := PluginID{
		Name:       name,
		PluginType: pluginType,
	}
	pinfo, ok := l.plugins[id]
	if !ok {
		return nil, fmt.Errorf("unknown plugin with name %q and type %q", name, pluginType)
	}

	// If the plugin is internal, launch via the factory
	var instance PluginInstance
	if pinfo.factory != nil {
		instance = &internalPluginInstance{
			instance: pinfo.factory(logger),
		}
	} else {
		var err error
		instance, err = l.dispensePlugin(pinfo.baseInfo.Type, pinfo.exePath, pinfo.args, nil, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to launch plugin: %v", err)
		}
	}

	// Cast to the base type and set the config
	base, ok := instance.Plugin().(base.BasePlugin)
	if !ok {
		return nil, fmt.Errorf("plugin %s doesn't implement base plugin interface", id)
	}

	if len(pinfo.msgpackConfig) != 0 {
		if err := base.SetConfig(pinfo.msgpackConfig, config); err != nil {
			return nil, fmt.Errorf("setting config for plugin %s failed: %v", id, err)
		}
	}

	return instance, nil
}

// Reattach reattaches to a previously launched external plugin.
func (l *PluginLoader) Reattach(name, pluginType string, config *plugin.ReattachConfig) (PluginInstance, error) {
	return l.dispensePlugin(pluginType, "", nil, config, l.logger)
}

// dispensePlugin is used to launch or reattach to an external plugin.
func (l *PluginLoader) dispensePlugin(
	pluginType, cmd string, args []string, reattach *plugin.ReattachConfig,
	logger log.Logger) (PluginInstance, error) {

	var pluginCmd *exec.Cmd
	if cmd != "" && reattach != nil {
		return nil, fmt.Errorf("both launch command and reattach config specified")
	} else if cmd == "" && reattach == nil {
		return nil, fmt.Errorf("one of launch command or reattach config must be specified")
	} else if cmd != "" {
		pluginCmd = exec.Command(cmd, args...)
	}

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  base.Handshake,
		Plugins:          getPluginMap(pluginType),
		Cmd:              pluginCmd,
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Logger:           logger,
		Reattach:         reattach,
	})

	// Connect via RPC
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, err
	}

	// Request the plugin
	raw, err := rpcClient.Dispense(pluginType)
	if err != nil {
		client.Kill()
		return nil, err
	}

	instance := &externalPluginInstance{
		client:   client,
		instance: raw,
	}
	return instance, nil
}

// getPluginMap returns a plugin map based on the type of plugin being launched.
func getPluginMap(pluginType string) map[string]plugin.Plugin {
	pmap := map[string]plugin.Plugin{
		base.PluginTypeBase: &base.PluginBase{},
	}

	switch pluginType {
	case base.PluginTypeDevice:
		pmap[base.PluginTypeDevice] = &device.PluginDevice{}
	}

	return pmap
}

// Catalog returns the catalog of all plugins
func (l *PluginLoader) Catalog() map[string][]*base.PluginInfoResponse {
	c := make(map[string][]*base.PluginInfoResponse, 3)
	for id, info := range l.plugins {
		c[id.PluginType] = append(c[id.PluginType], info.baseInfo)
	}
	return c
}
