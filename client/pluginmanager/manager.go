package pluginmanager

// PluginManager orchestrates the lifecycle of a set of plugins
type PluginManager interface {
	// Run starts a plugin manager and must block until shutdown
	Run()

	// Ready returns a channel that blocks until the plugin mananger has
	// initialized all plugins
	Ready() <-chan struct{}

	// Shutdown should gracefully shutdown all plugins managed by the manager.
	// It must block until shutdown is complete
	Shutdown()

	// PluginType is the type of plugin which the manager manages
	PluginType() string
}
