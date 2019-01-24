package catalog

import (
	"github.com/hashicorp/nomad/devices/gpu/nvidia"
	"github.com/hashicorp/nomad/drivers/rkt"
)

// This file is where all builtin plugins should be registered in the catalog.
// Plugins with build restrictions should be placed in the appropriate
// register_XXX.go file.
func init() {
	RegisterDeferredConfig(rkt.PluginID, rkt.PluginConfig, rkt.PluginLoader)
	Register(nvidia.PluginID, nvidia.PluginConfig)
}
