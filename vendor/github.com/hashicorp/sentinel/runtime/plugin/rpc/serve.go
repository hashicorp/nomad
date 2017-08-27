package rpc

import (
	goplugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/sentinel/runtime/plugin"
)

// The constants below are the names of the plugins that can be dispensed
// from the plugin server.
const (
	ImportPluginName = "import"
)

// Handshake is the HandshakeConfig used to configure clients and servers.
var Handshake = goplugin.HandshakeConfig{
	// The ProtocolVersion is the version that must match between core
	// and plugins. This should be bumped whenever a change happens in
	// one or the other that makes it so that they can't safely communicate.
	// This could be adding a new interface value, it could be how
	// helper/schema computes diffs, etc.
	ProtocolVersion: 1,

	// The magic cookie values should NEVER be changed.
	MagicCookieKey:   "SENTINEL_PLUGIN_MAGIC_COOKIE",
	MagicCookieValue: "2b7847b7b705781d7cf21a05e9c1bb37cbf078aea103bc3edcc6aca52ab65453",
}

// PluginMap should be used by clients for the map of plugins.
var PluginMap = map[string]goplugin.Plugin{
	ImportPluginName: &ImportPlugin{},
}

type ImportFunc func() plugin.Import

// ServeOpts are the configurations to serve a plugin.
type ServeOpts struct {
	ImportFunc ImportFunc
}

// Serve serves a plugin. This function never returns and should be the final
// function called in the main function of the plugin.
func Serve(opts *ServeOpts) {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins:         pluginMap(opts),
		GRPCServer:      goplugin.DefaultGRPCServer,
	})
}

// pluginMap returns the map[string]goplugin.Plugin to use for configuring a plugin
// server or client.
func pluginMap(opts *ServeOpts) map[string]goplugin.Plugin {
	return map[string]goplugin.Plugin{
		ImportPluginName: &ImportPlugin{F: opts.ImportFunc},
	}
}
