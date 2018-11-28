//+build linux,lxc

package catalog

import (
	"github.com/hashicorp/nomad/drivers/lxc"
)

// This file is where all builtin plugins should be registered in the catalog.
// Plugins with build restrictions should be placed in the appropriate
// register_XXX.go file.
func init() {
	RegisterDeferredConfig(lxc.PluginID, lxc.PluginConfig, lxc.PluginLoader)
}
