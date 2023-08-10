// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package loader

import (
	"strings"

	"github.com/hashicorp/nomad/nomad/structs/config"
)

// configMap returns a mapping of plugin binary name to config.
func configMap(configs []*config.PluginConfig) map[string]*config.PluginConfig {
	pluginMapping := make(map[string]*config.PluginConfig, len(configs))
	for _, c := range configs {
		pluginMapping[c.Name] = c
	}
	return pluginMapping
}

// cleanPluginExecutable strips the executable name of common suffixes
func cleanPluginExecutable(name string) string {
	switch {
	case strings.HasSuffix(name, ".exe"):
		return strings.TrimSuffix(name, ".exe")
	default:
		return name
	}
}
