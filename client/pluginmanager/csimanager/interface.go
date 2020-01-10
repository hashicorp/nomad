package csimanager

import "github.com/hashicorp/nomad/client/pluginmanager"

type Manager interface {
	// PluginManager returns a PluginManager for use by the node fingerprinter.
	PluginManager() pluginmanager.PluginManager

	// Shutdown shuts down the Manager and unmounts any locally attached volumes.
	Shutdown()
}
