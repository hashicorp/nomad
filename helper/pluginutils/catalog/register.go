package catalog

import (
	"github.com/hashicorp/nomad/drivers/docker"
	"github.com/hashicorp/nomad/drivers/exec"
	"github.com/hashicorp/nomad/drivers/java"
	"github.com/hashicorp/nomad/drivers/qemu"
	"github.com/hashicorp/nomad/drivers/rawexec"
	"github.com/hashicorp/nomad/plugins/logging/logmon"
	"github.com/hashicorp/nomad/plugins/logging/rotatelogs"
)

// This file is where all builtin plugins should be registered in the catalog.
// Plugins with build restrictions should be placed in the appropriate
// register_XXX.go file.
func init() {
	RegisterDeferredConfig(rawexec.PluginID, rawexec.PluginConfig, rawexec.PluginLoader)
	Register(exec.PluginID, exec.PluginConfig)
	Register(qemu.PluginID, qemu.PluginConfig)
	Register(java.PluginID, java.PluginConfig)
	RegisterDeferredConfig(docker.PluginID, docker.PluginConfig, docker.PluginLoader)
	Register(logmon.PluginID, logmon.PluginConfig)
	Register(rotatelogs.PluginID, rotatelogs.PluginConfig)
}
