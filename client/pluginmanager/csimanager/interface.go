package csimanager

import "github.com/hashicorp/nomad/client/pluginmanager"

type CSIManager interface {
	// PluginManager returns a PluginManager for use by the node fingerprinter.
	PluginManager() pluginmanager.PluginManager

	// Shutdown shuts down the CSIManager and unmounts any locally attached volumes.
	Shutdown()
}
