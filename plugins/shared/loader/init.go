package loader

import (
	"fmt"
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
	// Create a mapping of name to config
	configMap := configMap(config.Configs)

	// Initialize the internal plugins
	internal, err := l.initInternal(config.InternalPlugins, configMap)
	if err != nil {
		return fmt.Errorf("failed to fingerprint internal plugins: %v", err)
	}

	// Scan for eligibile binaries
	plugins, err := l.scan()
	if err != nil {
		return fmt.Errorf("failed to scan plugin directory %q: %v", l.pluginDir, err)
	}

	// Fingerprint the passed plugins
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
func (l *PluginLoader) initInternal(plugins map[PluginID]*InternalPluginConfig, configs map[string]*config.PluginConfig) (map[PluginID]*pluginInfo, error) {
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

		// Try to retrieve a user specified config
		if userConfig, ok := configs[k.Name]; ok && userConfig.Config != nil {
			info.config = userConfig.Config
		}

		// Fingerprint base info
		i, err := base.PluginInfo()
		if err != nil {
			multierror.Append(&mErr, fmt.Errorf("PluginInfo info failed for internal plugin %s: %v", k, err))
			continue
		}
		info.baseInfo = i

		// Parse and set the plugin version
		v, err := version.NewVersion(i.PluginVersion)
		if err != nil {
			multierror.Append(&mErr, fmt.Errorf("failed to parse version %q for internal plugin %s: %v", i.PluginVersion, k, err))
			continue
		}
		info.version = v

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
	if l.pluginDir == "" {
		return nil, nil
	}

	// Capture the list of binaries in the plugins folder
	f, err := os.Open(l.pluginDir)
	if err != nil {
		// There are no plugins to scan
		if os.IsNotExist(err) {
			l.logger.Debug("skipping external plugins since plugin_dir doesn't exist")
			return nil, nil
		}

		return nil, fmt.Errorf("failed to open plugin directory %q: %v", l.pluginDir, err)
	}
	files, err := f.Readdirnames(-1)
	f.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin directory %q: %v", l.pluginDir, err)
	}

	var plugins []os.FileInfo
	for _, f := range files {
		f = filepath.Join(l.pluginDir, f)
		s, err := os.Stat(f)
		if err != nil {
			return nil, fmt.Errorf("failed to stat file %q: %v", f, err)
		}
		if s.IsDir() {
			l.logger.Debug("skipping subdir in plugin folder", "subdir", f)
			continue
		}

		// Check if it is executable by anyone
		if s.Mode().Perm()&0111 == 0 {
			l.logger.Debug("skipping un-executable file in plugin folder", "file", f)
			continue
		}
		plugins = append(plugins, s)
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
			oldVersion := prev.version
			selectedVersion := info.version
			skip := false

			// Determine if we should keep the previous version or override
			if prev.version.GreaterThan(info.version) {
				oldVersion = info.version
				selectedVersion = prev.version
				skip = true
			}
			l.logger.Info("multiple versions of plugin detected",
				"plugin", info.baseInfo.Name, "older_version", oldVersion, "selected_version", selectedVersion)

			if skip {
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
	v, err := version.NewVersion(i.PluginVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse plugin %q (%v) version %q: %v",
			i.Name, info.exePath, i.PluginVersion, err)
	}
	info.version = v

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
				l.logger.Info("preferring internal version of plugin",
					"plugin", extPlugin.baseInfo.Name, "internal_version", internal.version,
					"external_version", extPlugin.version)
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

	// If there is no config, initialize it to an empty map so we can still
	// handle defaults
	if info.config == nil {
		info.config = map[string]interface{}{}
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
	instance, err := l.Dispense(id.Name, id.PluginType, nil, l.logger)
	if err != nil {
		return fmt.Errorf("failed to dispense plugin: %v", err)
	}
	defer instance.Kill()

	base, ok := instance.Plugin().(base.BasePlugin)
	if !ok {
		return fmt.Errorf("dispensed plugin %s doesn't meet base plugin interface", id)
	}

	if err := base.SetConfig(cdata, nil); err != nil {
		return fmt.Errorf("setting config on plugin failed: %v", err)
	}
	return nil
}
