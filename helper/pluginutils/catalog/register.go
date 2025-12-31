// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !cgo

package catalog

import (
	"github.com/hashicorp/nomad/drivers/docker"
	"github.com/hashicorp/nomad/drivers/java"
	"github.com/hashicorp/nomad/drivers/qemu"
	"github.com/hashicorp/nomad/drivers/rawexec"
)

// This file is where all builtin plugins should be registered in the catalog.
// Plugins with build restrictions should be placed in the appropriate
// register_XXX.go file.
func init() {
	RegisterDeferredConfig(rawexec.PluginID, rawexec.PluginConfig, rawexec.PluginLoader)
	Register(qemu.PluginID, qemu.PluginConfig)
	Register(java.PluginID, java.PluginConfig)
	RegisterDeferredConfig(docker.PluginID, docker.PluginConfig, docker.PluginLoader)
}
