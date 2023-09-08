// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package catalog is used to register internal plugins such that they can be
// loaded.
package catalog

import (
	"sync"

	"github.com/hashicorp/nomad/helper/pluginutils/loader"
)

var (
	// catalog is the set of registered internal plugins
	catalog = map[loader.PluginID]*Registration{}
	mu      sync.Mutex
)

// Registration is the registration of an internal plugin
type Registration struct {
	Config       *loader.InternalPluginConfig
	ConfigLoader ConfigFromOptions
}

// ConfigFromOptions is used to retrieve a plugin config when passed a node's
// option map. This allows upgrade pathing from the old configuration format to
// the new config format.
type ConfigFromOptions func(options map[string]string) (config map[string]interface{}, err error)

// Register is used to register an internal plugin.
func Register(id loader.PluginID, config *loader.InternalPluginConfig) {
	mu.Lock()
	defer mu.Unlock()
	catalog[id] = &Registration{
		Config: config,
	}
}

// RegisterDeferredConfig is used to register an internal plugin that sets its
// config using the client's option map.
func RegisterDeferredConfig(id loader.PluginID, config *loader.InternalPluginConfig, configLoader ConfigFromOptions) {
	mu.Lock()
	defer mu.Unlock()
	catalog[id] = &Registration{
		Config:       config,
		ConfigLoader: configLoader,
	}
}

// Catalog returns the catalog of internal plugins
func Catalog() map[loader.PluginID]*Registration {
	mu.Lock()
	defer mu.Unlock()
	return catalog
}
