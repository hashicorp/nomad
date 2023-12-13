// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package singleton

import (
	"fmt"
	"sync"

	log "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/plugins/base"
)

var (
	// SingletonPluginExited is returned when the dispense is called and the
	// existing plugin has exited. The caller should retry, and this will issue
	// a new plugin instance.
	SingletonPluginExited = fmt.Errorf("singleton plugin exited")
)

// SingletonLoader is used to only load a single external plugin at a time.
type SingletonLoader struct {
	// Loader is the underlying plugin loader that we wrap to give a singleton
	// behavior.
	loader loader.PluginCatalog

	// instances is a mapping of the plugin to a future which holds a plugin
	// instance
	instances    map[loader.PluginID]*future
	instanceLock sync.Mutex

	// logger is the logger used by the singleton
	logger log.Logger
}

// NewSingletonLoader wraps a plugin catalog and provides singleton behavior on
// top by caching running instances.
func NewSingletonLoader(logger log.Logger, catalog loader.PluginCatalog) *SingletonLoader {
	return &SingletonLoader{
		loader:    catalog,
		logger:    logger.Named("singleton_plugin_loader"),
		instances: make(map[loader.PluginID]*future, 4),
	}
}

// Catalog returns the catalog of all plugins keyed by plugin type
func (s *SingletonLoader) Catalog() map[string][]*base.PluginInfoResponse {
	return s.loader.Catalog()
}

// Dispense returns the plugin given its name and type. This will also
// configure the plugin. If there is an instance of an already running plugin,
// this is used.
func (s *SingletonLoader) Dispense(name, pluginType string, config *base.AgentConfig, logger log.Logger) (loader.PluginInstance, error) {
	return s.getPlugin(false, name, pluginType, logger, config, nil)
}

// Reattach is used to reattach to a previously launched external plugin.
func (s *SingletonLoader) Reattach(name, pluginType string, config *plugin.ReattachConfig) (loader.PluginInstance, error) {
	return s.getPlugin(true, name, pluginType, nil, nil, config)
}

// getPlugin is a helper that either dispenses or reattaches to a plugin using
// futures to ensure only a single instance is retrieved
func (s *SingletonLoader) getPlugin(reattach bool, name, pluginType string, logger log.Logger,
	nomadConfig *base.AgentConfig, config *plugin.ReattachConfig) (loader.PluginInstance, error) {

	// Lock the instance map to prevent races
	s.instanceLock.Lock()

	// Check if there is a future already
	id := loader.PluginID{Name: name, PluginType: pluginType}
	f, ok := s.instances[id]

	// Create the future and go get a plugin
	if !ok {
		f = newFuture()
		s.instances[id] = f

		if reattach {
			go s.reattach(f, name, pluginType, config)
		} else {
			go s.dispense(f, name, pluginType, nomadConfig, logger)
		}
	}

	// Unlock so that the created future can be shared
	s.instanceLock.Unlock()

	i, err := f.wait().result()
	if err != nil {
		s.clearFuture(id, f)
		return nil, err
	}

	if i.Exited() {
		s.clearFuture(id, f)
		return nil, SingletonPluginExited
	}

	return i, nil
}

// dispense should be called in a go routine to not block and creates the
// desired plugin, setting the results in the future.
func (s *SingletonLoader) dispense(f *future, name, pluginType string, config *base.AgentConfig, logger log.Logger) {
	i, err := s.loader.Dispense(name, pluginType, config, logger)
	f.set(i, err)
}

// reattach should be called in a go routine to not block and reattaches to the
// desired plugin, setting the results in the future.
func (s *SingletonLoader) reattach(f *future, name, pluginType string, config *plugin.ReattachConfig) {
	i, err := s.loader.Reattach(name, pluginType, config)
	f.set(i, err)
}

// clearFuture clears the future from the instances map only if the futures
// match. This prevents clearing the unintented instance.
func (s *SingletonLoader) clearFuture(id loader.PluginID, f *future) {
	s.instanceLock.Lock()
	defer s.instanceLock.Unlock()
	if f.equal(s.instances[id]) {
		delete(s.instances, id)
	}
}
