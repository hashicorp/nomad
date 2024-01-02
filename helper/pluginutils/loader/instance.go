// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package loader

import plugin "github.com/hashicorp/go-plugin"

// PluginInstance wraps an instance of a plugin. If the plugin is external, it
// provides methods to retrieve the ReattachConfig and to kill the plugin.
type PluginInstance interface {
	// Internal returns if the plugin is internal
	Internal() bool

	// Kill kills the plugin if it is external. It is safe to call on internal
	// plugins.
	Kill()

	// ReattachConfig returns the ReattachConfig and whether the plugin is internal
	// or not. If the second return value is false, no ReattachConfig is
	// possible to return.
	ReattachConfig() (config *plugin.ReattachConfig, canReattach bool)

	// Plugin returns the wrapped plugin instance.
	Plugin() interface{}

	// Exited returns whether the plugin has exited
	Exited() bool

	// ApiVersion returns the API version to be used with the plugin
	ApiVersion() string
}

// internalPluginInstance wraps an internal plugin
type internalPluginInstance struct {
	instance   interface{}
	apiVersion string
	killFn     func()
}

func (p *internalPluginInstance) Internal() bool { return true }
func (p *internalPluginInstance) Kill()          { p.killFn() }

func (p *internalPluginInstance) ReattachConfig() (*plugin.ReattachConfig, bool) { return nil, false }
func (p *internalPluginInstance) Plugin() interface{}                            { return p.instance }
func (p *internalPluginInstance) Exited() bool                                   { return false }
func (p *internalPluginInstance) ApiVersion() string                             { return p.apiVersion }

// externalPluginInstance wraps an external plugin
type externalPluginInstance struct {
	client     *plugin.Client
	instance   interface{}
	apiVersion string
}

func (p *externalPluginInstance) Internal() bool      { return false }
func (p *externalPluginInstance) Plugin() interface{} { return p.instance }
func (p *externalPluginInstance) Exited() bool        { return p.client.Exited() }
func (p *externalPluginInstance) ApiVersion() string  { return p.apiVersion }

func (p *externalPluginInstance) ReattachConfig() (*plugin.ReattachConfig, bool) {
	return p.client.ReattachConfig(), true
}

func (p *externalPluginInstance) Kill() {
	p.client.Kill()
}
