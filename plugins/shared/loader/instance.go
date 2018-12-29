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
}

// internalPluginInstance wraps an internal plugin
type internalPluginInstance struct {
	instance interface{}
}

func (p *internalPluginInstance) Internal() bool                                 { return true }
func (p *internalPluginInstance) Kill()                                          {}
func (p *internalPluginInstance) ReattachConfig() (*plugin.ReattachConfig, bool) { return nil, false }
func (p *internalPluginInstance) Plugin() interface{}                            { return p.instance }
func (p *internalPluginInstance) Exited() bool                                   { return false }

// externalPluginInstance wraps an external plugin
type externalPluginInstance struct {
	client   *plugin.Client
	instance interface{}
}

func (p *externalPluginInstance) Internal() bool      { return false }
func (p *externalPluginInstance) Plugin() interface{} { return p.instance }
func (p *externalPluginInstance) Exited() bool        { return p.client.Exited() }

func (p *externalPluginInstance) ReattachConfig() (*plugin.ReattachConfig, bool) {
	return p.client.ReattachConfig(), true
}

func (p *externalPluginInstance) Kill() {
	p.client.Kill()
}
