// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build cgo

package catalog

import (
	"github.com/hashicorp/nomad/v2/drivers/docker"
	"github.com/hashicorp/nomad/v2/drivers/exec"
	"github.com/hashicorp/nomad/v2/drivers/java"
	"github.com/hashicorp/nomad/v2/drivers/qemu"
	"github.com/hashicorp/nomad/v2/drivers/rawexec"
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
}
