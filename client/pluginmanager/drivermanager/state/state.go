package state

import "github.com/hashicorp/nomad/plugins/shared"

// PluginState is used to store the driver managers state across restarts of the
// agent
type PluginState struct {
	// ReattachConfigs are the set of reattach configs for plugin's launched by
	// the driver manager
	ReattachConfigs map[string]*shared.ReattachConfig
}
