package loader

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	multierror "github.com/hashicorp/go-multierror"
	plugin "github.com/hashicorp/go-plugin"
	version "github.com/hashicorp/go-version"
	hcl2 "github.com/hashicorp/hcl2/hcl"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/shared"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/zclconf/go-cty/cty/msgpack"
)

var (
	// configParseCtx is the context used to parse a plugin's configuration
	// stanza
	configParseCtx = &hcl2.EvalContext{
		Functions: shared.GetStdlibFuncs(),
	}
)

// validateConfig returns whether or not the configuration is valid
func validateConfig(config *PluginLoaderConfig) error {
	var mErr multierror.Error
	if config == nil {
		return fmt.Errorf("nil config passed")
	} else if config.Logger == nil {
		multierror.Append(&mErr, fmt.Errorf("nil logger passed"))
	} else if config.PluginDir == "" {
		multierror.Append(&mErr, fmt.Errorf("invalid plugin dir %q passed", config.PluginDir))
	}

	// Validate that all plugins have a binary name
	for _, c := range config.Configs {
		if c.Name == "" {
			multierror.Append(&mErr, fmt.Errorf("plugin config passed without binary name"))
		}
	}

	// Validate internal plugins
	for k, config := range config.InternalPlugins {
		// Validate config
		if config == nil {
			multierror.Append(&mErr, fmt.Errorf("nil config passed for internal plugin %s", k))
			continue
		} else if config.Factory == nil {
			multierror.Append(&mErr, fmt.Errorf("nil factory passed for internal plugin %s", k))
			continue
		}
	}

	return mErr.ErrorOrNil()
}

// init initializes the plugin loader by compiling both internal and external
// plugins and selecting the highest versioned version of any given plugin.
func (l *PluginLoader) init(config *PluginLoaderConfig) error {
	// Initialize the internal plugins
	internal, err := l.initInternal(config.InternalPlugins)
	if err != nil {
		return fmt.Errorf("failed to fingerprint internal plugins: %v", err)
	}

	// Scan for eligibile binaries
	plugins, err := l.scan()
	if err != nil {
		return fmt.Errorf("failed to scan plugin directory %q: %v", l.pluginDir, err)
	}

	// Fingerprint the passed plugins
	configMap := configMap(config.Configs)
	external, err := l.fingerprintPlugins(plugins, configMap)
	if err != nil {
		return fmt.Errorf("failed to fingerprint plugins: %v", err)
	}

	// Merge external and internal plugins
	l.plugins = l.mergePlugins(internal, external)

	// Validate that the configs are valid for the plugins
	if err := l.validatePluginConfigs(); err != nil {
		return fmt.Errorf("parsing plugin configurations failed: %v", err)
	}

	return nil
}

// initInternal initializes internal plugins.
func (l *PluginLoader) initInternal(plugins map[PluginID]*InternalPluginConfig) (map[PluginID]*pluginInfo, error) {
	var mErr multierror.Error
	fingerprinted := make(map[PluginID]*pluginInfo, len(plugins))
	for k, config := range plugins {
		// Create an instance
		raw := config.Factory(l.logger)
		base, ok := raw.(base.BasePlugin)
		if !ok {
			multierror.Append(&mErr, fmt.Errorf("internal plugin %s doesn't meet base plugin interface", k))
			continue
		}

		info := &pluginInfo{
			factory: config.Factory,
			config:  config.Config,
		}

		// Fingerprint base info
		i, err := base.PluginInfo()
		if err != nil {
			multierror.Append(&mErr, fmt.Errorf("PluginInfo info failed for internal plugin %s: %v", k, err))
			continue
		}
		info.baseInfo = i

		// Parse and set the plugin version
		if v, err := version.NewVersion(i.PluginVersion); err != nil {
			multierror.Append(&mErr, fmt.Errorf("failed to parse version %q for internal plugin %s: %v", i.PluginVersion, k, err))
			continue
		} else {
			info.version = v
		}

		// Get the config schema
		schema, err := base.ConfigSchema()
		if err != nil {
			multierror.Append(&mErr, fmt.Errorf("failed to retrieve config schema for internal plugin %s: %v", k, err))
			continue
		}
		info.configSchema = schema

		// Store the fingerprinted config
		fingerprinted[k] = info
	}

	return fingerprinted, nil
}

// scan scans the plugin directory and retrieves potentially eligible binaries
func (l *PluginLoader) scan() ([]os.FileInfo, error) {
	// Capture the list of binaries in the plugins folder
	files, err := ioutil.ReadDir(l.pluginDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin directory %q: %v", l.pluginDir, err)
	}

	var plugins []os.FileInfo
	for _, f := range files {
		if f.IsDir() {
			l.logger.Debug("skipping sub-dir in plugin folder", "sub-dir", f.Name())
			continue
		}
		plugins = append(plugins, f)
	}

	return plugins, nil
}

// fingerprintPlugins fingerprints all external plugin binaries
func (l *PluginLoader) fingerprintPlugins(plugins []os.FileInfo, configs map[string]*config.PluginConfig) (map[PluginID]*pluginInfo, error) {
	var mErr multierror.Error
	fingerprinted := make(map[PluginID]*pluginInfo, len(plugins))
	for _, p := range plugins {
		name := cleanPluginExecutable(p.Name())
		c := configs[name]
		info, err := l.fingerprintPlugin(p, c)
		if err != nil {
			l.logger.Error("failed to fingerprint plugin", "plugin", name)
			multierror.Append(&mErr, err)
			continue
		}

		id := PluginID{
			Name:       info.baseInfo.Name,
			PluginType: info.baseInfo.Type,
		}

		// Detect if we already have seen a version of this plugin
		if prev, ok := fingerprinted[id]; ok {

			// Determine if we should keep the previous version or override
			if prev.version.GreaterThan(info.version) {
				l.logger.Info("multiple versions of plugin detected", "plugin", info.baseInfo.Name)
				continue
			}
		}

		// Add the plugin
		fingerprinted[id] = info
	}

	if err := mErr.ErrorOrNil(); err != nil {
		return nil, err
	}

	return fingerprinted, nil
}

// fingerprintPlugin fingerprints the passed external plugin
func (l *PluginLoader) fingerprintPlugin(pluginExe os.FileInfo, config *config.PluginConfig) (*pluginInfo, error) {
	info := &pluginInfo{
		exePath: filepath.Join(l.pluginDir, pluginExe.Name()),
	}

	// Build the command
	cmd := exec.Command(info.exePath)
	if config != nil {
		cmd.Args = append(cmd.Args, config.Args...)
		info.args = config.Args
		info.config = config.Config
	}

	// Launch the plugin
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: base.Handshake,
		Plugins: map[string]plugin.Plugin{
			base.PluginTypeBase: &base.PluginBase{},
		},
		Cmd:              cmd,
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Logger:           l.logger,
	})
	defer client.Kill()

	// Connect via RPC
	rpcClient, err := client.Client()
	if err != nil {
		return nil, err
	}

	// Request the plugin
	raw, err := rpcClient.Dispense(base.PluginTypeBase)
	if err != nil {
		return nil, err
	}

	// Cast the plugin to the base type
	bplugin := raw.(base.BasePlugin)

	// Retrieve base plugin information
	i, err := bplugin.PluginInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get plugin info for plugin %q: %v", info.exePath, err)
	}
	info.baseInfo = i

	// Parse and set the plugin version
	if v, err := version.NewVersion(i.PluginVersion); err != nil {
		return nil, fmt.Errorf("failed to parse plugin %q (%v) version %q: %v",
			i.Name, info.exePath, i.PluginVersion, err)
	} else {
		info.version = v
	}

	// Retrieve the schema
	schema, err := bplugin.ConfigSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to get plugin config schema for plugin %q: %v", info.exePath, err)
	}
	info.configSchema = schema

	return info, nil
}

// mergePlugins merges internal and external plugins, preferring the highest
// version.
func (l *PluginLoader) mergePlugins(internal, external map[PluginID]*pluginInfo) map[PluginID]*pluginInfo {
	finalized := make(map[PluginID]*pluginInfo, len(internal))

	// Load the internal plugins
	for k, v := range internal {
		finalized[k] = v
	}

	for k, extPlugin := range external {
		internal, ok := finalized[k]
		if ok {
			// We have overlapping plugins, determine if we should keep the
			// internal version or override
			if extPlugin.version.LessThan(internal.version) {
				l.logger.Info("preferring external version of plugin",
					"plugin", extPlugin.baseInfo.Name, "internal_version", internal.version.String(),
					"external_version", extPlugin.version.String())
				continue
			}
		}

		// Add external plugin
		finalized[k] = extPlugin
	}

	return finalized
}

// validatePluginConfigs is used to validate each plugins' configuration. If the
// plugin has a config, it is parsed with the plugins config schema and
// SetConfig is called to ensure the config is valid.
func (l *PluginLoader) validatePluginConfigs() error {
	var mErr multierror.Error
	for id, info := range l.plugins {
		if err := l.validePluginConfig(id, info); err != nil {
			wrapped := multierror.Prefix(err, fmt.Sprintf("plugin %s:", id))
			multierror.Append(&mErr, wrapped)
		}
	}

	return mErr.ErrorOrNil()
}

// validatePluginConfig is used to validate the plugin's configuration. If the
// plugin has a config, it is parsed with the plugins config schema and
// SetConfig is called to ensure the config is valid.
func (l *PluginLoader) validePluginConfig(id PluginID, info *pluginInfo) error {
	var mErr multierror.Error

	// Check if a config is allowed
	if info.configSchema == nil {
		if info.config != nil {
			return fmt.Errorf("configuration not allowed but config passed")
		}

		// Nothing to do
		return nil
	}

	// Convert the schema to hcl
	spec, diag := hclspec.Convert(info.configSchema)
	if diag.HasErrors() {
		multierror.Append(&mErr, diag.Errs()...)
		return multierror.Prefix(&mErr, "failed converting config schema:")
	}

	// If there is no config there is nothing to do
	if info.config == nil {
		return nil
	}

	// Parse the config using the spec
	val, diag := shared.ParseHclInterface(info.config, spec, configParseCtx)
	if diag.HasErrors() {
		multierror.Append(&mErr, diag.Errs()...)
		return multierror.Prefix(&mErr, "failed parsing config:")
	}

	// Marshal the value
	cdata, err := msgpack.Marshal(val, val.Type())
	if err != nil {
		return fmt.Errorf("failed to msgpack encode config: %v", err)
	}

	// Store the marshalled config
	info.msgpackConfig = cdata

	// Dispense the plugin and set its config and ensure it is error free
	instance, err := l.Dispense(id.Name, id.PluginType, l.logger)
	if err != nil {
		return fmt.Errorf("failed to dispense plugin: %v", err)
	}
	defer instance.Kill()

	base, ok := instance.Plugin().(base.BasePlugin)
	if !ok {
		return fmt.Errorf("dispensed plugin %s doesn't meet base plugin interface", id)
	}

	if err := base.SetConfig(cdata); err != nil {
		return fmt.Errorf("setting config on plugin failed: %v", err)
	}
	return nil
}
